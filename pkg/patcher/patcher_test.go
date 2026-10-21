package patcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIDEPatchAndRestore(t *testing.T) {
	tmpDir := t.TempDir()
	mainJS := filepath.Join(tmpDir, "main.js")

	originalContent := []byte("function init(){resetIsTierGCPTos(),this._authService.isGoogleInternal?enable():disable();}")
	if err := os.WriteFile(mainJS, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	p := &Patcher{
		Target: TargetIDE,
		Path:   mainJS,
	}

	// 1. Initial status check
	status, _, err := p.GetStatus()
	if err != nil {
		t.Fatalf("unexpected error checking status: %v", err)
	}
	if status != StatusUnpatched {
		t.Fatalf("expected unpatched, got %s", status)
	}

	// 2. Apply patch
	if err := p.ApplyPatch(); err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}

	// 3. Status check after patch
	status, _, err = p.GetStatus()
	if err != nil {
		t.Fatalf("status error after patch: %v", err)
	}
	if status != StatusPatched {
		t.Fatalf("expected patched, got %s", status)
	}

	// 4. Verify content
	patchedData, _ := os.ReadFile(mainJS)
	expected := "function init(){resetIsTierGCPTos(),true?enable():disable();}"
	if string(patchedData) != expected {
		t.Fatalf("content mismatch: got %q, expected %q", string(patchedData), expected)
	}

	// 5. Restore
	if err := p.Restore(); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 6. Verify restored
	status, _, err = p.GetStatus()
	if err != nil {
		t.Fatalf("status error after restore: %v", err)
	}
	if status != StatusUnpatched {
		t.Fatalf("expected unpatched after restore, got %s", status)
	}
}
