package binary

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRanges(t *testing.T) {
	ranges := []Range{
		{Start: 10, End: 20},
		{Start: 15, End: 25},
		{Start: 30, End: 40},
		{Start: 5, End: 12},
	}
	norm := NormalizeRanges(ranges, 50)
	if len(norm) != 2 {
		t.Fatalf("expected 2 merged ranges, got %d", len(norm))
	}
	if norm[0].Start != 5 || norm[0].End != 25 {
		t.Errorf("expected [5, 25), got [%d, %d)", norm[0].Start, norm[0].End)
	}
	if norm[1].Start != 30 || norm[1].End != 40 {
		t.Errorf("expected [30, 40), got [%d, %d)", norm[1].Start, norm[1].End)
	}
}

func TestInspectScript(t *testing.T) {
	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "test.js")
	content := []byte("console.log('hello world');")
	if err := os.WriteFile(jsFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	info, err := InspectFile(jsFile)
	if err != nil {
		t.Fatalf("InspectFile failed: %v", err)
	}
	if info.Arch != ArchAll || info.Format != "raw" {
		t.Errorf("expected raw/all, got %s/%s", info.Format, info.Arch)
	}
	if len(info.Ranges) != 1 || info.Ranges[0].Start != 0 || info.Ranges[0].End != int64(len(content)) {
		t.Errorf("unexpected ranges: %+v", info.Ranges)
	}
}
