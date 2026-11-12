package detector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveScoopShimWithShimFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scoop_shim_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	shimsDir := filepath.Join(tmpDir, "shims")
	appsDir := filepath.Join(tmpDir, "apps", "agy", "current", "bin")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create actual binary
	realBin := filepath.Join(appsDir, "agy.exe")
	if err := os.WriteFile(realBin, []byte("fake real agy binary"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create shim executable
	shimExe := filepath.Join(shimsDir, "agy.exe")
	if err := os.WriteFile(shimExe, []byte("fake shim exe"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create agy.shim with relative path
	shimMeta := filepath.Join(shimsDir, "agy.shim")
	shimContent := "path = ..\\apps\\agy\\current\\bin\\agy.exe\r\nargs = \r\n"
	if err := os.WriteFile(shimMeta, []byte(shimContent), 0644); err != nil {
		t.Fatal(err)
	}

	resolved := ResolveScoopShim(shimExe)
	if filepath.Clean(resolved) != filepath.Clean(realBin) {
		t.Errorf("expected resolved %s, got %s", realBin, resolved)
	}
}

func TestResolveScoopShimWithEnvVar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scoop_shim_env_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	_ = os.Setenv("TEST_SCOOP_ROOT", tmpDir)
	defer os.Unsetenv("TEST_SCOOP_ROOT")

	shimsDir := filepath.Join(tmpDir, "shims")
	appsDir := filepath.Join(tmpDir, "apps", "agy", "current", "bin")
	_ = os.MkdirAll(shimsDir, 0755)
	_ = os.MkdirAll(appsDir, 0755)

	realBin := filepath.Join(appsDir, "agy.exe")
	_ = os.WriteFile(realBin, []byte("fake real agy binary"), 0755)

	shimExe := filepath.Join(shimsDir, "agy.exe")
	_ = os.WriteFile(shimExe, []byte("fake shim exe"), 0755)

	// Shim with %TEST_SCOOP_ROOT%
	shimMeta := filepath.Join(shimsDir, "agy.shim")
	shimContent := "path = %TEST_SCOOP_ROOT%\\apps\\agy\\current\\bin\\agy.exe\r\n"
	_ = os.WriteFile(shimMeta, []byte(shimContent), 0644)

	resolved := ResolveScoopShim(shimExe)
	if filepath.Clean(resolved) != filepath.Clean(realBin) {
		t.Errorf("expected resolved %s with env expansion, got %s", realBin, resolved)
	}
}

func TestResolveScoopShimWithoutShimFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scoop_shim_nometadata_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	shimsDir := filepath.Join(tmpDir, "shims")
	appsDir := filepath.Join(tmpDir, "apps", "agy", "current", "bin")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create actual binary
	realBin := filepath.Join(appsDir, "agy.exe")
	if err := os.WriteFile(realBin, []byte("fake real agy binary"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create shim executable inside shims/ without .shim metadata file
	shimExe := filepath.Join(shimsDir, "agy.exe")
	if err := os.WriteFile(shimExe, []byte("fake shim exe"), 0755); err != nil {
		t.Fatal(err)
	}

	resolved := ResolveScoopShim(shimExe)
	if filepath.Clean(resolved) != filepath.Clean(realBin) {
		t.Errorf("expected fallback resolved %s, got %s", realBin, resolved)
	}
}

func TestResolveScoopShimNonShim(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "regular_bin_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	regularBin := filepath.Join(tmpDir, "agy")
	if err := os.WriteFile(regularBin, []byte("regular binary"), 0755); err != nil {
		t.Fatal(err)
	}

	resolved := ResolveScoopShim(regularBin)
	if resolved != regularBin {
		t.Errorf("expected %s, got %s", regularBin, resolved)
	}
}

func TestAddCandidateDeduplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "candidate_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bin1 := filepath.Join(tmpDir, "agy1")
	if err := os.WriteFile(bin1, []byte("bin1"), 0755); err != nil {
		t.Fatal(err)
	}

	var candidates []string
	seen := make(map[string]bool)

	addCandidate(&candidates, seen, bin1)
	addCandidate(&candidates, seen, bin1) // duplicate
	addCandidate(&candidates, seen, filepath.Join(tmpDir, "non_existent"))

	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate after dedup, got %d", len(candidates))
	}
	if candidates[0] != bin1 {
		t.Errorf("expected candidate %s, got %s", bin1, candidates[0])
	}
}

func TestExpandWinEnv(t *testing.T) {
	_ = os.Setenv("TEST_FOO", "hello")
	_ = os.Setenv("TEST_BAR", "world")
	defer os.Unsetenv("TEST_FOO")
	defer os.Unsetenv("TEST_BAR")

	res := expandWinEnv("%TEST_FOO%\\%TEST_BAR%\\bin")
	expected := "hello\\world\\bin"
	if res != expected {
		t.Errorf("expected %q, got %q", expected, res)
	}

	unchanged := expandWinEnv("no_env_here")
	if unchanged != "no_env_here" {
		t.Errorf("expected no_env_here, got %q", unchanged)
	}
}
