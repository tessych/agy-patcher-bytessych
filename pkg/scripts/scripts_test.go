package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildBatchContent(t *testing.T) {
	// Standard invocation
	content := BuildBatchContent("agy-patcher.exe", "patch", "Run patch")

	if !strings.HasPrefix(content, "@echo off\r\n") {
		t.Errorf("expected @echo off at start, got %q", content)
	}

	if !strings.Contains(content, `cd /d "%~dp0"`) {
		t.Errorf("expected directory change in batch file, got %q", content)
	}

	if !strings.Contains(content, `"%~dp0agy-patcher.exe" patch %*`) {
		t.Errorf("expected command execution with arg pass-through in batch file, got %q", content)
	}

	if !strings.Contains(content, "\r\npause\r\n") {
		t.Errorf("expected pause with CRLF, got %q", content)
	}

	// Full path should be sanitized to base executable name
	contentWithPath := BuildBatchContent(`C:\Program Files\Agy\agy-patcher.exe`, "status", "Check status")
	if !strings.Contains(contentWithPath, `"%~dp0agy-patcher.exe" status %*`) {
		t.Errorf("expected sanitized base exe name in batch file, got %q", contentWithPath)
	}

	// Empty exeName defaults to agy-patcher.exe
	contentDefault := BuildBatchContent("", "", "Default run")
	if !strings.Contains(contentDefault, `"%~dp0agy-patcher.exe" %*`) {
		t.Errorf("expected default exe name without command, got %q", contentDefault)
	}
}

func TestGenerateBatchScripts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scripts_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	created, err := GenerateBatchScripts(tmpDir, "custom-patcher.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFiles := []string{"patch.bat", "doctor.bat", "status.bat", "restore.bat"}
	if len(created) != len(expectedFiles) {
		t.Fatalf("expected %d scripts, got %d", len(expectedFiles), len(created))
	}

	for _, name := range expectedFiles {
		p := filepath.Join(tmpDir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("file %s not found: %v", name, err)
			continue
		}
		s := string(data)
		if !strings.Contains(s, "@echo off") || !strings.Contains(s, "pause") {
			t.Errorf("file %s missing echo off or pause: %s", name, s)
		}
		if !strings.Contains(s, "custom-patcher.exe") {
			t.Errorf("file %s missing custom-patcher.exe: %s", name, s)
		}
	}
}

func TestEnsureBatchScripts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ensure_scripts_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Pre-create patch.bat with custom text
	existingPatch := filepath.Join(tmpDir, "patch.bat")
	if err := os.WriteFile(existingPatch, []byte("REM already exists"), 0644); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureBatchScripts(tmpDir, "agy-patcher.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not have overwritten patch.bat
	data, _ := os.ReadFile(existingPatch)
	if string(data) != "REM already exists" {
		t.Errorf("existing patch.bat was overwritten!")
	}

	// Should have created doctor.bat, status.bat, restore.bat
	if len(created) != 3 {
		t.Errorf("expected 3 newly created files, got %d (%v)", len(created), created)
	}
}
