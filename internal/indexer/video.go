package indexer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Chunk represents a time range within a video.
type Chunk struct {
	Start float64
	End   float64
	Index int
}

// ChunkFile represents a chunk that has been written to disk.
type ChunkFile struct {
	Path      string
	StartTime float64
	EndTime   float64
	Index     int
}

// calculateChunks divides a video duration into overlapping chunks.
func calculateChunks(duration float64, chunkLen float64, overlap float64) []Chunk {
	var chunks []Chunk
	step := chunkLen - overlap
	i := 0
	for start := 0.0; start < duration; start += step {
		end := start + chunkLen
		if end > duration {
			end = duration
		}
		chunks = append(chunks, Chunk{Start: start, End: end, Index: i})
		i++
		if end >= duration {
			break
		}
	}
	return chunks
}

// buildChunkArgs returns the ffmpeg arguments for extracting a chunk.
func buildChunkArgs(input, output string, start, duration float64) []string {
	return []string{
		"-ss", strconv.FormatFloat(start, 'f', -1, 64),
		"-i", input,
		"-t", strconv.FormatFloat(duration, 'f', -1, 64),
		"-c", "copy",
		"-y", output,
	}
}

// GetVideoDuration returns the duration of a video file in seconds using ffprobe.
func GetVideoDuration(path string) (float64, error) {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

// ChunkVideo splits a video into overlapping chunks using ffmpeg stream copy.
func ChunkVideo(inputPath, outputDir string, chunkLen, overlap float64) ([]ChunkFile, error) {
	duration, err := GetVideoDuration(inputPath)
	if err != nil {
		return nil, err
	}

	chunks := calculateChunks(duration, chunkLen, overlap)
	var results []ChunkFile

	for _, c := range chunks {
		outName := fmt.Sprintf("chunk_%03d.mp4", c.Index)
		outPath := filepath.Join(outputDir, outName)
		args := buildChunkArgs(inputPath, outPath, c.Start, c.End-c.Start)

		if err := exec.Command("ffmpeg", args...).Run(); err != nil {
			return nil, fmt.Errorf("ffmpeg chunk %d: %w", c.Index, err)
		}
		results = append(results, ChunkFile{
			Path:      outPath,
			StartTime: c.Start,
			EndTime:   c.End,
			Index:     c.Index,
		})
	}
	return results, nil
}

// PreprocessChunk re-encodes a video chunk for analysis (480p, 5fps, no audio).
func PreprocessChunk(inputPath, outputPath string) error {
	args := []string{
		"-i", inputPath,
		"-vf", "scale=-2:480",
		"-r", "5",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-an",
		"-y", outputPath,
	}
	return exec.Command("ffmpeg", args...).Run()
}

// IsStillFrame detects whether a video chunk is essentially a still image
// by comparing JPEG-encoded frames at different positions.
func IsStillFrame(chunkPath string) (bool, error) {
	duration, err := GetVideoDuration(chunkPath)
	if err != nil {
		return false, err
	}

	tmpDir, err := os.MkdirTemp("", "stillcheck-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmpDir)

	// Extract 3 frames at 25%, 50%, 75% of duration
	positions := []float64{duration * 0.25, duration * 0.5, duration * 0.75}
	var sizes []int64

	for i, pos := range positions {
		outPath := filepath.Join(tmpDir, fmt.Sprintf("frame_%d.jpg", i))
		err := exec.Command("ffmpeg",
			"-ss", strconv.FormatFloat(pos, 'f', 2, 64),
			"-i", chunkPath,
			"-frames:v", "1",
			"-q:v", "5",
			"-y", outPath,
		).Run()
		if err != nil {
			return false, fmt.Errorf("extract frame %d: %w", i, err)
		}
		info, err := os.Stat(outPath)
		if err != nil {
			return false, err
		}
		sizes = append(sizes, info.Size())
	}

	// Compare: if all frame sizes are within 2% of each other, it's a still frame
	for i := 1; i < len(sizes); i++ {
		ratio := float64(min(sizes[0], sizes[i])) / float64(max(sizes[0], sizes[i]))
		if ratio < 0.98 {
			return false, nil
		}
	}
	return true, nil
}

// ExtractPreviewClip extracts a short preview clip centered on a timestamp.
func ExtractPreviewClip(videoPath, outputPath string, timestamp, duration float64) error {
	startTime := timestamp - duration/2
	if startTime < 0 {
		startTime = 0
	}
	// Try stream copy first (fastest)
	err := exec.Command("ffmpeg",
		"-ss", strconv.FormatFloat(startTime, 'f', 2, 64),
		"-i", videoPath,
		"-t", strconv.FormatFloat(duration, 'f', 2, 64),
		"-c", "copy",
		"-an",
		"-y", outputPath,
	).Run()
	if err == nil {
		return nil
	}
	// Fallback: re-encode
	return exec.Command("ffmpeg",
		"-ss", strconv.FormatFloat(startTime, 'f', 2, 64),
		"-i", videoPath,
		"-t", strconv.FormatFloat(duration, 'f', 2, 64),
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-an",
		"-y", outputPath,
	).Run()
}
