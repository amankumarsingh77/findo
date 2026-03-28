package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"universal-search/internal/embedder"
	"universal-search/internal/store"
	"universal-search/internal/vectorstore"
)

// IndexStatus tracks progress of an indexing operation.
type IndexStatus struct {
	TotalFiles    int
	IndexedFiles  int
	FailedFiles   int
	CurrentFile   string
	IsRunning     bool
	QuotaPaused   bool
	QuotaResumeAt string // ISO 8601 timestamp or empty
}

// Pipeline orchestrates file indexing: walking folders, hashing for change
// detection, generating thumbnails, extracting/embedding content, and storing
// vectors and metadata.
type Pipeline struct {
	store    *store.Store
	index    *vectorstore.Index
	embedder *embedder.Embedder
	thumbDir string
	logger   *slog.Logger

	mu      sync.RWMutex
	status  IndexStatus
	paused  bool
	pauseCh chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewPipeline creates a new indexing pipeline.
func NewPipeline(s *store.Store, idx *vectorstore.Index, emb *embedder.Embedder, thumbDir string, logger *slog.Logger) *Pipeline {
	ctx, cancel := context.WithCancel(context.Background())
	log := logger.WithGroup("indexer")
	log.Info("pipeline created", "thumbDir", thumbDir)
	return &Pipeline{
		store:    s,
		index:    idx,
		embedder: emb,
		thumbDir: thumbDir,
		logger:   log,
		pauseCh:  make(chan struct{}, 1),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Status returns the current indexing progress.
func (p *Pipeline) Status() IndexStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// Pause pauses the indexing pipeline.
func (p *Pipeline) Pause() {
	p.logger.Info("pipeline paused")
	p.mu.Lock()
	p.paused = true
	p.mu.Unlock()
}

// Resume resumes the indexing pipeline after a pause.
func (p *Pipeline) Resume() {
	p.logger.Info("pipeline resumed")
	p.mu.Lock()
	p.paused = false
	p.mu.Unlock()
	select {
	case p.pauseCh <- struct{}{}:
	default:
	}
}

// Stop cancels the indexing pipeline.
func (p *Pipeline) Stop() {
	p.logger.Info("pipeline stopping")
	p.cancel()
}

func (p *Pipeline) waitIfPaused() {
	for {
		p.mu.RLock()
		paused := p.paused
		p.mu.RUnlock()
		if !paused {
			return
		}
		select {
		case <-p.pauseCh:
		case <-p.ctx.Done():
			return
		}
	}
}

// IndexFolder walks a folder, classifies files, and indexes each one.
// Files matching excludePatterns (glob) are skipped.
func (p *Pipeline) IndexFolder(folderPath string, excludePatterns []string) error {
	p.logger.Info("indexing folder", "path", folderPath, "excludePatterns", len(excludePatterns))
	start := time.Now()

	var files []string
	err := filepath.WalkDir(folderPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors, continue walking
		}
		if d.IsDir() {
			for _, pat := range excludePatterns {
				if matched, _ := filepath.Match(pat, d.Name()); matched {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ft := ClassifyFile(path)
		if ft != "" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	p.logger.Info("discovered files", "count", len(files), "folder", folderPath)

	p.mu.Lock()
	p.status = IndexStatus{TotalFiles: len(files), IsRunning: true}
	p.mu.Unlock()

	for _, filePath := range files {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		default:
		}

		p.waitIfPaused()

		p.mu.Lock()
		p.status.CurrentFile = filePath
		p.mu.Unlock()

		if err := p.indexFile(filePath); err != nil {
			p.logger.Warn("file indexing failed", "path", filePath, "error", err)
			p.mu.Lock()
			p.status.FailedFiles++
			if isQuotaExhaustedError(err) {
				p.status.QuotaPaused = true
				p.status.QuotaResumeAt = time.Now().Add(30 * time.Minute).Format(time.RFC3339)
				p.logger.Error("all API keys exhausted, pausing indexing", "resumeAt", p.status.QuotaResumeAt)
			}
			p.mu.Unlock()

			if isQuotaExhaustedError(err) {
				p.waitForQuotaRecovery()
			}
		} else {
			p.mu.Lock()
			p.status.IndexedFiles++
			p.mu.Unlock()
		}
	}

	p.mu.Lock()
	p.status.IsRunning = false
	p.mu.Unlock()

	p.logger.Info("folder indexing complete",
		"folder", folderPath,
		"indexed", p.status.IndexedFiles,
		"failed", p.status.FailedFiles,
		"total", len(files),
		"duration", time.Since(start),
	)

	return nil
}

// IndexSingleFile indexes a single file.
func (p *Pipeline) IndexSingleFile(filePath string) error {
	return p.indexFile(filePath)
}

func (p *Pipeline) indexFile(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	hash, err := hashFile(filePath)
	if err != nil {
		return err
	}

	// Check if already indexed with same hash
	existing, err := p.store.GetFileByPath(filePath)
	if err == nil && existing.ContentHash == hash {
		p.logger.Debug("skipping unchanged file", "path", filePath)
		return nil // unchanged
	}

	fileType := ClassifyFile(filePath)
	ext := filepath.Ext(filePath)
	p.logger.Debug("indexing file", "path", filePath, "type", fileType, "size", info.Size())

	// Generate thumbnail
	thumbPath, _ := GenerateThumbnail(filePath, p.thumbDir, fileType)

	// Upsert file record
	fileID, err := p.store.UpsertFile(store.FileRecord{
		Path:          filePath,
		FileType:      fileType,
		Extension:     ext,
		SizeBytes:     info.Size(),
		ModifiedAt:    info.ModTime(),
		IndexedAt:     time.Now(),
		ContentHash:   hash,
		ThumbnailPath: thumbPath,
	})
	if err != nil {
		return err
	}

	// Delete old chunks/vectors for this file
	oldVecIDs, _ := p.store.GetVectorIDsByFileID(fileID)
	for _, vid := range oldVecIDs {
		p.index.Delete(vid)
	}
	p.store.DeleteChunksByFileID(fileID)

	// Embed based on file type
	switch fileType {
	case "text":
		return p.indexTextFile(filePath, fileID)
	case "image":
		return p.indexBinaryFile(filePath, fileID, MimeType(filePath))
	case "video":
		return p.indexVideoFile(filePath, fileID)
	case "audio":
		return p.indexBinaryFile(filePath, fileID, MimeType(filePath))
	}

	return nil
}

func (p *Pipeline) indexTextFile(filePath string, fileID int64) error {
	ext := filepath.Ext(filePath)
	if ext == ".pdf" {
		// PDFs: embed raw bytes via multimodal
		return p.indexBinaryFile(filePath, fileID, "application/pdf")
	}

	content, err := ExtractText(filePath)
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}

	vec, err := p.embedder.EmbedDocument(p.ctx, content)
	if err != nil {
		return err
	}

	vecID := fmt.Sprintf("f%d-c0", fileID)
	if err := p.index.Add(vecID, vec); err != nil {
		return err
	}

	_, err = p.store.InsertChunk(store.ChunkRecord{
		FileID: fileID, VectorID: vecID, ChunkIndex: 0,
	})
	return err
}

func (p *Pipeline) indexBinaryFile(filePath string, fileID int64, mimeType string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	vec, err := p.embedder.EmbedBytes(p.ctx, data, mimeType)
	if err != nil {
		return err
	}

	vecID := fmt.Sprintf("f%d-c0", fileID)
	if err := p.index.Add(vecID, vec); err != nil {
		return err
	}

	_, err = p.store.InsertChunk(store.ChunkRecord{
		FileID: fileID, VectorID: vecID, ChunkIndex: 0,
	})
	return err
}

func (p *Pipeline) indexVideoFile(filePath string, fileID int64) error {
	tmpDir, err := os.MkdirTemp("", "vidchunk-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	chunks, err := ChunkVideo(filePath, tmpDir, 30, 5)
	if err != nil {
		return err
	}

	for _, chunk := range chunks {
		// Check for still frames
		still, err := IsStillFrame(chunk.Path)
		if err == nil && still {
			p.logger.Debug("skipping still-frame chunk", "path", filePath, "chunk", chunk.Index)
			continue
		}

		// Preprocess
		preprocessed := filepath.Join(tmpDir, fmt.Sprintf("pre_%03d.mp4", chunk.Index))
		if err := PreprocessChunk(chunk.Path, preprocessed); err != nil {
			p.logger.Warn("video preprocess failed", "path", filePath, "chunk", chunk.Index, "error", err)
			continue
		}

		data, err := os.ReadFile(preprocessed)
		if err != nil {
			p.logger.Warn("reading preprocessed chunk failed", "path", filePath, "chunk", chunk.Index, "error", err)
			continue
		}

		vec, err := p.embedder.EmbedBytes(p.ctx, data, "video/mp4")
		if err != nil {
			p.logger.Warn("video embedding failed", "path", filePath, "chunk", chunk.Index, "error", err)
			continue
		}

		vecID := fmt.Sprintf("f%d-c%d", fileID, chunk.Index)
		if err := p.index.Add(vecID, vec); err != nil {
			p.logger.Warn("adding video vector failed", "path", filePath, "chunk", chunk.Index, "error", err)
			continue
		}

		p.store.InsertChunk(store.ChunkRecord{
			FileID:     fileID,
			VectorID:   vecID,
			StartTime:  chunk.StartTime,
			EndTime:    chunk.EndTime,
			ChunkIndex: chunk.Index,
		})
	}

	return nil
}

func isQuotaExhaustedError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "keys exhausted") ||
		strings.Contains(s, "keys are cooling or exhausted")
}

func (p *Pipeline) waitForQuotaRecovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			p.status.QuotaPaused = false
			p.status.QuotaResumeAt = ""
			p.mu.Unlock()
			p.logger.Info("quota recovery check, resuming indexing")
			return
		}
	}
}

// hashFile computes the SHA-256 hash of a file's contents.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
