package chunker

import (
	"testing"
)

func TestCalculateTimeRanges(t *testing.T) {
	ranges := calculateTimeRanges(65.0, 30, 5)
	if len(ranges) != 3 {
		t.Fatalf("expected 3 ranges, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 30 {
		t.Errorf("range 0: got %v", ranges[0])
	}
	if ranges[1].Start != 25 || ranges[1].End != 55 {
		t.Errorf("range 1: got %v", ranges[1])
	}
	if ranges[2].Start != 50 || ranges[2].End != 65 {
		t.Errorf("range 2: got %v", ranges[2])
	}
}

func TestCalculateTimeRanges_ShortVideo(t *testing.T) {
	ranges := calculateTimeRanges(10.0, 30, 5)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 10 {
		t.Errorf("range 0: got %v", ranges[0])
	}
}

func TestCalculateTimeRanges_ExactFit(t *testing.T) {
	ranges := calculateTimeRanges(30.0, 30, 5)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 30 {
		t.Errorf("range 0: got %v", ranges[0])
	}
}
