package protect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendEnvToShellFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "protect_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, ".bashrc")

	// 1. First append to non-existent file
	changed, err := AppendEnvToShellFile(testFile, "TEST_VAR", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("expected changed=true on new file")
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "export TEST_VAR=1") {
		t.Errorf("expected export TEST_VAR=1 in file, got:\n%s", string(data))
	}

	// 2. Second append should detect existing configuration and be idempotent
	changed, err = AppendEnvToShellFile(testFile, "TEST_VAR", "1")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if changed {
		t.Errorf("expected changed=false on already configured file")
	}

	// Ensure no duplicate lines
	count := strings.Count(string(data), "TEST_VAR=1")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence, found %d", count)
	}

	// 3. Commented out variable should NOT be treated as active and should trigger append
	commentedFile := filepath.Join(tmpDir, ".zshrc")
	_ = os.WriteFile(commentedFile, []byte("# export TEST_VAR=1\n"), 0644)
	changed, err = AppendEnvToShellFile(commentedFile, "TEST_VAR", "1")
	if err != nil || !changed {
		t.Errorf("expected changed=true when variable is commented out, got %v, err=%v", changed, err)
	}

	// 4. Disabled variable (value 0) should be replaced with value 1
	disabledFile := filepath.Join(tmpDir, ".profile")
	_ = os.WriteFile(disabledFile, []byte("export TEST_VAR=0\n"), 0644)
	changed, err = AppendEnvToShellFile(disabledFile, "TEST_VAR", "1")
	if err != nil || !changed {
		t.Errorf("expected changed=true when variable is disabled, got %v, err=%v", changed, err)
	}
	disData, _ := os.ReadFile(disabledFile)
	if strings.Contains(string(disData), "TEST_VAR=0") || !strings.Contains(string(disData), "export TEST_VAR=1") {
		t.Errorf("expected disabled var to be replaced with 1, got:\n%s", string(disData))
	}

	// 5. Fish shell support
	fishFile := filepath.Join(tmpDir, "config.fish")
	changed, err = AppendEnvToShellFile(fishFile, "TEST_VAR", "1")
	if err != nil || !changed {
		t.Errorf("expected changed=true on fish config, err=%v", err)
	}
	fishData, _ := os.ReadFile(fishFile)
	if !strings.Contains(string(fishData), "set -gx TEST_VAR 1") {
		t.Errorf("expected set -gx TEST_VAR 1 in fish config, got:\n%s", string(fishData))
	}
}

func TestIsShellLineActive(t *testing.T) {
	tests := []struct {
		line     string
		key      string
		expected bool
	}{
		{"export AGY=1", "AGY", true},
		{"export AGY=\"1\"", "AGY", true},
		{"export AGY='1'", "AGY", true},
		{"export AGY=true", "AGY", true},
		{"AGY=1", "AGY", true},
		{"# export AGY=1", "AGY", false},
		{"#AGY=1", "AGY", false},
		{"export AGY=0", "AGY", false},
		{"export AGY=false", "AGY", false},
		{"export OTHER_VAR=1", "AGY", false},
		{"set -gx AGY 1", "AGY", true},
		{"set -x AGY true", "AGY", true},
		{"# set -gx AGY 1", "AGY", false},
		{"set -gx AGY 0", "AGY", false},
	}

	for _, tt := range tests {
		got := isShellLineActive(tt.line, tt.key)
		if got != tt.expected {
			t.Errorf("isShellLineActive(%q, %q) = %v; want %v", tt.line, tt.key, got, tt.expected)
		}
	}
}

func TestParseRegistryValue(t *testing.T) {
	out := `
HKEY_CURRENT_USER\Environment
    AGY_CLI_DISABLE_AUTO_UPDATE    REG_SZ    1
    PATH    REG_EXPAND_SZ    C:\bin
`
	val, ok := parseRegistryValue(out, EnvDisableAutoUpdate)
	if !ok || val != "1" {
		t.Errorf("expected 1, got val=%q, ok=%v", val, ok)
	}

	valPath, ok := parseRegistryValue(out, "PATH")
	if !ok || valPath != `C:\bin` {
		t.Errorf("expected C:\\bin, got val=%q, ok=%v", valPath, ok)
	}

	_, ok = parseRegistryValue(out, "NON_EXISTENT")
	if ok {
		t.Errorf("expected false for NON_EXISTENT")
	}
}

func TestIsAutoUpdateDisabledEnv(t *testing.T) {
	orig := os.Getenv(EnvDisableAutoUpdate)
	defer func() {
		_ = os.Setenv(EnvDisableAutoUpdate, orig)
	}()

	_ = os.Setenv(EnvDisableAutoUpdate, "0")
	if IsAutoUpdateDisabled() {
		t.Errorf("expected IsAutoUpdateDisabled=false when env is explicitly 0")
	}

	_ = os.Setenv(EnvDisableAutoUpdate, "false")
	if IsAutoUpdateDisabled() {
		t.Errorf("expected IsAutoUpdateDisabled=false when env is explicitly false")
	}

	_ = os.Setenv(EnvDisableAutoUpdate, "1")
	if !IsAutoUpdateDisabled() {
		t.Errorf("expected IsAutoUpdateDisabled=true when env is 1")
	}

	_ = os.Setenv(EnvDisableAutoUpdate, "true")
	if !IsAutoUpdateDisabled() {
		t.Errorf("expected IsAutoUpdateDisabled=true when env is true")
	}
}
