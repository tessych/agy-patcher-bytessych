package detector

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type TargetPaths struct {
	CLI       string
	Manager   string
	IDE       string
	Extension string
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func selectNewest(candidates []string) string {
	var newest string
	var newestTime time.Time

	for _, c := range candidates {
		fi, err := os.Stat(c)
		if err == nil && !fi.IsDir() {
			if newest == "" || fi.ModTime().After(newestTime) {
				newest = c
				newestTime = fi.ModTime()
			}
		}
	}
	return newest
}

func FindTargets() TargetPaths {
	var res TargetPaths

	home, _ := os.UserHomeDir()

	// 1. CLI (agy)
	if p, err := exec.LookPath("agy"); err == nil {
		res.CLI = p
	} else if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "agy", "bin", "agy.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "agy", "bin", "agy.exe"),
			filepath.Join(home, "scoop", "apps", "agy", "current", "bin", "agy.exe"),
		}
		res.CLI = selectNewest(candidates)
	} else {
		candidates := []string{
			filepath.Join(home, ".local", "bin", "agy"),
			filepath.Join(home, "bin", "agy"),
			"/usr/local/bin/agy",
			"/usr/bin/agy",
			"/opt/homebrew/bin/agy",
		}
		res.CLI = selectNewest(candidates)
	}

	// 2. Manager (language_server)
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Antigravity", "resources", "bin", "language_server.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Antigravity", "resources", "bin", "language_server.exe"),
		}
		res.Manager = selectNewest(candidates)
	} else if runtime.GOOS == "darwin" {
		candidates := []string{
			"/Applications/Antigravity.app/Contents/Resources/bin/language_server",
			filepath.Join(home, "Applications", "Antigravity.app", "Contents/Resources/bin/language_server"),
		}
		res.Manager = selectNewest(candidates)
	} else {
		candidates := []string{
			"/opt/Antigravity/resources/bin/language_server",
			filepath.Join(home, ".local", "share", "Antigravity", "resources", "bin", "language_server"),
			"/usr/lib/antigravity/resources/bin/language_server",
			"/usr/share/antigravity/resources/bin/language_server",
		}
		res.Manager = selectNewest(candidates)
	}

	// 3. IDE (main.js)
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "antigravity-ide", "resources", "app", "out", "main.js"),
			filepath.Join(os.Getenv("ProgramFiles"), "antigravity-ide", "resources", "app", "out", "main.js"),
		}
		res.IDE = selectNewest(candidates)
	} else if runtime.GOOS == "darwin" {
		candidates := []string{
			"/Applications/Antigravity IDE.app/Contents/Resources/app/out/main.js",
			filepath.Join(home, "Applications", "Antigravity IDE.app", "Contents/Resources/app/out/main.js"),
		}
		res.IDE = selectNewest(candidates)
	} else {
		candidates := []string{
			"/opt/antigravity-ide/resources/app/out/main.js",
			filepath.Join(home, ".local", "share", "antigravity-ide", "resources", "app", "out", "main.js"),
			"/usr/share/antigravity-ide/resources/app/out/main.js",
		}
		res.IDE = selectNewest(candidates)
	}

	// 4. Extension (agy --hub)
	if home != "" {
		binName := "agy"
		if runtime.GOOS == "windows" {
			binName = "agy.exe"
		}
		extPath := filepath.Join(home, ".gemini", "bin", binName)
		if fileExists(extPath) {
			res.Extension = extPath
		}
	}

	return res
}
