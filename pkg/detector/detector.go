package detector

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type TargetPaths struct {
	CLI       string
	CLIs      []string
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

func expandWinEnv(s string) string {
	for {
		start := strings.Index(s, "%")
		if start == -1 {
			break
		}
		end := strings.Index(s[start+1:], "%")
		if end == -1 {
			break
		}
		end = start + 1 + end
		varName := s[start+1 : end]
		val := os.Getenv(varName)
		s = s[:start] + val + s[end+1:]
	}
	return s
}

func isScoopShim(p string) bool {
	dir := filepath.Base(filepath.Dir(p))
	if strings.EqualFold(dir, "shims") {
		return true
	}
	shimCandidates := []string{
		strings.TrimSuffix(p, filepath.Ext(p)) + ".shim",
		p + ".shim",
	}
	for _, sc := range shimCandidates {
		if fileExists(sc) {
			return true
		}
	}
	return false
}

// ResolveScoopShim проверяет, является ли путь Scoop-шимом, и возвращает реальный целевой бинарник
func ResolveScoopShim(p string) string {
	if p == "" {
		return ""
	}

	// 1. Проверяем наличие сопутствующего .shim файла (Scoop shim metadata)
	shimCandidates := []string{
		strings.TrimSuffix(p, filepath.Ext(p)) + ".shim",
		p + ".shim",
	}

	for _, shimFile := range shimCandidates {
		if f, err := os.Open(shimFile); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(strings.ToLower(line), "path") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						target := strings.TrimSpace(parts[1])
						target = strings.Trim(target, "\"'")
						target = expandWinEnv(target)
						if !filepath.IsAbs(target) {
							target = filepath.Join(filepath.Dir(p), target)
						}
						target = filepath.Clean(target)
						if fileExists(target) {
							_ = f.Close()
							return target
						}
					}
				}
			}
			_ = f.Close()
		}
	}

	// 2. Если файл находится в директории shims, ищем в apps/<app>/current/bin
	dir := filepath.Dir(p)
	if strings.EqualFold(filepath.Base(dir), "shims") {
		parent := filepath.Dir(dir)
		baseName := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		candidates := []string{
			filepath.Join(parent, "apps", baseName, "current", "bin", filepath.Base(p)),
			filepath.Join(parent, "apps", baseName, "current", filepath.Base(p)),
			filepath.Join(parent, "apps", baseName, "current", "bin", baseName+".exe"),
			filepath.Join(parent, "apps", baseName, "current", baseName+".exe"),
		}
		for _, c := range candidates {
			if fileExists(c) {
				return c
			}
		}
	}

	return p
}

func addCandidate(list *[]string, seen map[string]bool, rawPath string) {
	if rawPath == "" {
		return
	}

	// Разрешаем Scoop shim
	target := ResolveScoopShim(rawPath)

	// На Windows отфильтровываем скриптовые обертки (.cmd, .bat, .ps1) и неразрешенные Scoop шимы
	if runtime.GOOS == "windows" {
		lower := strings.ToLower(target)
		if strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat") || strings.HasSuffix(lower, ".ps1") {
			return
		}
		if !strings.HasSuffix(lower, ".exe") {
			return
		}
		if isScoopShim(target) {
			return
		}
	}

	// Разрешаем симлинки, если применимо
	if resolved, err := filepath.EvalSymlinks(target); err == nil && fileExists(resolved) {
		target = resolved
	}

	target = filepath.Clean(target)
	if !fileExists(target) {
		return
	}

	key := target
	if runtime.GOOS == "windows" {
		key = strings.ToLower(target)
	}

	if !seen[key] {
		seen[key] = true
		*list = append(*list, target)
	}
}

// FindCLITargets находит ВСЕ установленные экземпляры Agy CLI на машине
func FindCLITargets() []string {
	var candidates []string
	seen := make(map[string]bool)
	home, _ := os.UserHomeDir()

	// 1. Поиск в PATH
	if p, err := exec.LookPath("agy"); err == nil {
		addCandidate(&candidates, seen, p)
	}

	pathEnv := os.Getenv("PATH")
	for _, rawDir := range filepath.SplitList(pathEnv) {
		dir := strings.Trim(strings.TrimSpace(rawDir), `"'`)
		if dir == "" {
			continue
		}
		if runtime.GOOS == "windows" {
			addCandidate(&candidates, seen, filepath.Join(dir, "agy.exe"))
		} else {
			addCandidate(&candidates, seen, filepath.Join(dir, "agy"))
		}
	}

	// 2. Стандартные системные пути
	if runtime.GOOS == "windows" {
		localApp := os.Getenv("LOCALAPPDATA")
		progFiles := os.Getenv("ProgramFiles")
		progFilesX86 := os.Getenv("ProgramFiles(x86)")
		scoopEnv := os.Getenv("SCOOP")
		scoopGlobal := os.Getenv("SCOOP_GLOBAL")

		var winCandidates []string
		if localApp != "" {
			winCandidates = append(winCandidates,
				filepath.Join(localApp, "Programs", "agy", "bin", "agy.exe"),
				filepath.Join(localApp, "Programs", "agy", "agy.exe"),
			)
		}
		if progFiles != "" {
			winCandidates = append(winCandidates,
				filepath.Join(progFiles, "agy", "bin", "agy.exe"),
				filepath.Join(progFiles, "agy", "agy.exe"),
			)
		}
		if progFilesX86 != "" {
			winCandidates = append(winCandidates,
				filepath.Join(progFilesX86, "agy", "bin", "agy.exe"),
				filepath.Join(progFilesX86, "agy", "agy.exe"),
			)
		}
		if home != "" {
			winCandidates = append(winCandidates,
				filepath.Join(home, "scoop", "apps", "agy", "current", "bin", "agy.exe"),
				filepath.Join(home, "scoop", "apps", "agy", "current", "agy.exe"),
				filepath.Join(home, "scoop", "shims", "agy.exe"),
			)
		}
		if scoopEnv != "" {
			winCandidates = append(winCandidates,
				filepath.Join(scoopEnv, "apps", "agy", "current", "bin", "agy.exe"),
				filepath.Join(scoopEnv, "apps", "agy", "current", "agy.exe"),
				filepath.Join(scoopEnv, "shims", "agy.exe"),
			)
		}
		if scoopGlobal != "" {
			winCandidates = append(winCandidates,
				filepath.Join(scoopGlobal, "apps", "agy", "current", "bin", "agy.exe"),
				filepath.Join(scoopGlobal, "apps", "agy", "current", "agy.exe"),
				filepath.Join(scoopGlobal, "shims", "agy.exe"),
			)
		}
		winCandidates = append(winCandidates,
			`C:\ProgramData\scoop\apps\agy\current\bin\agy.exe`,
			`C:\ProgramData\scoop\apps\agy\current\agy.exe`,
		)

		for _, c := range winCandidates {
			addCandidate(&candidates, seen, c)
		}
	} else {
		var unixCandidates []string
		if home != "" {
			unixCandidates = append(unixCandidates,
				filepath.Join(home, ".local", "bin", "agy"),
				filepath.Join(home, "bin", "agy"),
			)
		}
		unixCandidates = append(unixCandidates,
			"/usr/local/bin/agy",
			"/usr/bin/agy",
			"/opt/homebrew/bin/agy",
			"/home/linuxbrew/.linuxbrew/bin/agy",
		)

		for _, c := range unixCandidates {
			addCandidate(&candidates, seen, c)
		}
	}

	return candidates
}

func FindTargets() TargetPaths {
	var res TargetPaths

	home, _ := os.UserHomeDir()

	// 1. CLI (agy) - поиск ВСЕХ установок
	res.CLIs = FindCLITargets()
	if len(res.CLIs) > 0 {
		res.CLI = res.CLIs[0]
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
			"/Applications/Antigravity.app/Contents/Resources/app/out/main.js",
			filepath.Join(home, "Applications", "Antigravity.app", "Contents/Resources/app/out/main.js"),
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
