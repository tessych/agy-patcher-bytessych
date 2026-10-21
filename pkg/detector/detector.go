package detector

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// TargetPaths хранит найденные пути к установленным компонентам
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

// FindTargets ищет установленные компоненты Antigravity в системе
func FindTargets() TargetPaths {
	var res TargetPaths

	home, _ := os.UserHomeDir()

	// 1. Поиск CLI (agy)
	if p, err := exec.LookPath("agy"); err == nil {
		res.CLI = p
	} else if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "agy", "bin", "agy.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "agy", "bin", "agy.exe"),
			filepath.Join(home, "scoop", "apps", "agy", "current", "bin", "agy.exe"),
		}
		for _, c := range candidates {
			if fileExists(c) {
				res.CLI = c
				break
			}
		}
	} else {
		candidates := []string{
			filepath.Join(home, ".local", "bin", "agy"),
			filepath.Join(home, "bin", "agy"),
			"/usr/local/bin/agy",
			"/usr/bin/agy",
			"/opt/homebrew/bin/agy",
		}
		for _, c := range candidates {
			if fileExists(c) {
				res.CLI = c
				break
			}
		}
	}

	// 2. Поиск Manager (language_server)
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Antigravity", "resources", "bin", "language_server.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Antigravity", "resources", "bin", "language_server.exe"),
		}
		for _, c := range candidates {
			if fileExists(c) {
				res.Manager = c
				break
			}
		}
	} else if runtime.GOOS == "darwin" {
		candidates := []string{
			"/Applications/Antigravity.app/Contents/Resources/bin/language_server",
			filepath.Join(home, "Applications", "Antigravity.app", "Contents/Resources/bin/language_server"),
		}
		for _, c := range candidates {
			if fileExists(c) {
				res.Manager = c
				break
			}
		}
	} else {
		// Linux
		candidates := []string{
			"/opt/Antigravity/resources/bin/language_server",
			filepath.Join(home, ".local", "share", "Antigravity", "resources", "bin", "language_server"),
			"/usr/lib/antigravity/resources/bin/language_server",
			"/usr/share/antigravity/resources/bin/language_server",
		}
		for _, c := range candidates {
			if fileExists(c) {
				res.Manager = c
				break
			}
		}
	}

	// 3. Поиск IDE (main.js)
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "antigravity-ide", "resources", "app", "out", "main.js"),
			filepath.Join(os.Getenv("ProgramFiles"), "antigravity-ide", "resources", "app", "out", "main.js"),
		}
		for _, c := range candidates {
			if fileExists(c) {
				res.IDE = c
				break
			}
		}
	} else if runtime.GOOS == "darwin" {
		candidates := []string{
			"/Applications/Antigravity IDE.app/Contents/Resources/app/out/main.js",
			filepath.Join(home, "Applications", "Antigravity IDE.app", "Contents/Resources/app/out/main.js"),
		}
		for _, c := range candidates {
			if fileExists(c) {
				res.IDE = c
				break
			}
		}
	} else {
		// Linux
		candidates := []string{
			filepath.Join(home, ".local", "share", "antigravity-ide", "resources", "app", "out", "main.js"),
			"/opt/antigravity-ide/resources/app/out/main.js",
			"/usr/share/antigravity-ide/resources/app/out/main.js",
		}
		for _, c := range candidates {
			if fileExists(c) {
				res.IDE = c
				break
			}
		}
	}

	// 4. Поиск Extension (agy --hub)
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
