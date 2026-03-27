package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"universal-search/internal/embedder"
	"universal-search/internal/store"
	"universal-search/internal/vectorstore"
)

// IndexStatus tracks progress of an indexing operation.
type IndexStatus struct {
	TotalFiles   int
	IndexedFiles int
	CurrentFile  string
	IsRunning    bool
}

// Pipeline orchestrates file indexing: walking folders, hashing for change
// detection, generating thumbnails, extracting/embedding content, and storing
// vectors and metadata.
type Pipeline struct {
	store    *store.Store
	index    *vectorstore.Index
	embedder *embedder.Embedder
	thumbDir string

	mu      sync.RWMutex
	status  IndexStatus
	paused  bool
	pauseCh chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewPipeline creates a new indexing pipeline.
func NewPipeline(s *store.Store, idx *vectorstore.Index, emb *embedder.Embedder, thumbDir string) *Pipeline {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pipeline{
		store:    s,
		index:    idx,
		embedder: emb,
		thumbDir: thumbDir,
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
	p.mu.Lock()
	p.paused = true
	p.mu.Unlock()
}

// Resume resumes the indexing pipeline after a pause.
func (p *Pipeline) Resume() {
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

	p.mu.Lock()
	p.status = IndexStatus{TotalFiles: len(files), IsRunning: true}
	p.mu.Unlock()

	for i, filePath := range files {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		default:
		}

		p.waitIfPaused()

		p.mu.Lock()
		p.status.CurrentFile = filePath
		p.status.IndexedFiles = i
		p.mu.Unlock()

		if err := p.indexFile(filePath); err != nil {
			// Log error but continue with other files
			fmt.Fprintf(os.Stderr, "index %s: %v\n", filePath, err)
		}
	}

	p.mu.Lock()
	p.status.IndexedFiles = len(files)
	p.status.IsRunning = false
	p.mu.Unlock()

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
		return nil // unchanged
	}

	fileType := ClassifyFile(filePath)
	ext := filepath.Ext(filePath)

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
			continue
		}

		// Preprocess
		preprocessed := filepath.Join(tmpDir, fmt.Sprintf("pre_%03d.mp4", chunk.Index))
		if err := PreprocessChunk(chunk.Path, preprocessed); err != nil {
			continue
		}

		data, err := os.ReadFile(preprocessed)
		if err != nil {
			continue
		}

		vec, err := p.embedder.EmbedBytes(p.ctx, data, "video/mp4")
		if err != nil {
			continue
		}

		vecID := fmt.Sprintf("f%d-c%d", fileID, chunk.Index)
		if err := p.index.Add(vecID, vec); err != nil {
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
