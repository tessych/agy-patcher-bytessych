package protect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	EnvDisableAutoUpdate = "AGY_CLI_DISABLE_AUTO_UPDATE"
)

// ProtectionResult содержит детали выполненной настройки
type ProtectionResult struct {
	AlreadyActive bool
	Targets       []string
	Details       string
}

// IsAutoUpdateDisabled проверяет, отключено ли автообновление в текущей среде или постоянных конфигурациях
func IsAutoUpdateDisabled() bool {
	val := os.Getenv(EnvDisableAutoUpdate)
	if val == "1" || strings.ToLower(val) == "true" {
		return true
	}
	if val == "0" || strings.ToLower(val) == "false" {
		return false
	}

	if runtime.GOOS == "windows" {
		return isWindowsRegistryDisabled()
	}

	return isUnixShellConfigured()
}

func parseRegistryValue(out, key string) (string, bool) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], key) {
			return strings.Join(fields[2:], " "), true
		}
	}
	return "", false
}

func isWindowsRegistryDisabled() bool {
	cmd := exec.Command("reg", "query", `HKCU\Environment`, "/v", EnvDisableAutoUpdate)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	val, ok := parseRegistryValue(string(out), EnvDisableAutoUpdate)
	if !ok {
		return false
	}
	val = strings.TrimSpace(val)
	return val == "1" || strings.EqualFold(val, "true")
}

// isShellLineActive проверяет, активирует ли конкретная строка файла переменную окружения со значением 1/true
func isShellLineActive(line, key string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return false
	}

	// POSIX shells (export KEY=1 или KEY=1)
	candidate := trimmed
	if strings.HasPrefix(candidate, "export ") {
		candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "export "))
	}
	if strings.HasPrefix(candidate, key+"=") || strings.HasPrefix(candidate, key+" =") {
		parts := strings.SplitN(candidate, "=", 2)
		if len(parts) == 2 {
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			return val == "1" || strings.EqualFold(val, "true")
		}
	}

	// Fish shell syntax: set -gx KEY 1 или set -x KEY 1
	if strings.HasPrefix(trimmed, "set ") {
		fields := strings.Fields(trimmed)
		for i, f := range fields {
			if strings.EqualFold(f, key) && i+1 < len(fields) {
				val := strings.Trim(fields[i+1], `"'`)
				return val == "1" || strings.EqualFold(val, "true")
			}
		}
	}

	return false
}

func isUnixShellConfigured() bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}

	candidates := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}

	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if isShellLineActive(line, EnvDisableAutoUpdate) {
					return true
				}
			}
		}
	}

	return false
}

// AppendEnvToShellFile добавляет или обновляет export ПЕРЕМЕННАЯ=ЗНАЧЕНИЕ в указанном конфигурационном файле шелла
func AppendEnvToShellFile(filePath, key, val string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	isFish := strings.HasSuffix(filePath, "config.fish")
	content := string(data)
	lines := strings.Split(content, "\n")

	alreadyActive := false
	replaced := false
	var newLines []string

	for _, line := range lines {
		if isShellLineActive(line, key) {
			alreadyActive = true
			newLines = append(newLines, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		// Если переменная присутствует в незакомментированном виде (но выключена или устарела), обновляем её
		if !strings.HasPrefix(trimmed, "#") {
			if !isFish && (strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, "export "+key+"=")) {
				newLines = append(newLines, fmt.Sprintf("export %s=%s", key, val))
				replaced = true
				continue
			}
			if isFish && strings.HasPrefix(trimmed, "set ") && strings.Contains(trimmed, key) {
				fields := strings.Fields(trimmed)
				for _, f := range fields {
					if strings.EqualFold(f, key) {
						newLines = append(newLines, fmt.Sprintf("set -gx %s %s", key, val))
						replaced = true
						break
					}
				}
				if replaced {
					continue
				}
			}
		}

		newLines = append(newLines, line)
	}

	if alreadyActive {
		return false, nil
	}

	if replaced {
		newContent := strings.Join(newLines, "\n")
		if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
			return false, err
		}
		return true, nil
	}

	// Добавляем новый блок в конец файла
	var block string
	if isFish {
		block = fmt.Sprintf("\n# Added by agy-patcher: disable CLI auto-updater to protect patched binaries\nset -gx %s %s\n", key, val)
	} else {
		block = fmt.Sprintf("\n# Added by agy-patcher: disable CLI auto-updater to protect patched binaries\nexport %s=%s\n", key, val)
	}

	newContent := content
	if len(newContent) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += block

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return false, err
	}

	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// EnableAutoUpdateProtection включает постоянную защиту от автообновления CLI на Windows/Linux/macOS
func EnableAutoUpdateProtection() (*ProtectionResult, error) {
	_ = os.Setenv(EnvDisableAutoUpdate, "1")

	if runtime.GOOS == "windows" {
		return enableWindowsProtection()
	}

	return enableUnixProtection()
}

func enableWindowsProtection() (*ProtectionResult, error) {
	if isWindowsRegistryDisabled() {
		return &ProtectionResult{
			AlreadyActive: true,
			Targets:       []string{`Registry (HKCU\Environment)`},
			Details:       "Already configured in HKCU\\Environment (" + EnvDisableAutoUpdate + "=1)",
		}, nil
	}

	var targets []string
	var errs []string

	// 1. setx для широковещательного уведомления пользовательского окружения
	if err := exec.Command("setx", EnvDisableAutoUpdate, "1").Run(); err != nil {
		errs = append(errs, fmt.Sprintf("setx failed: %v", err))
	} else {
		targets = append(targets, "User Environment (setx)")
	}

	// 2. HKCU\Environment реестр для надежности
	if err := exec.Command("reg", "add", `HKCU\Environment`, "/v", EnvDisableAutoUpdate, "/t", "REG_SZ", "/d", "1", "/f").Run(); err != nil {
		errs = append(errs, fmt.Sprintf("reg add failed: %v", err))
	} else {
		targets = append(targets, `Registry (HKCU\Environment)`)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("failed to configure Windows environment: %s", strings.Join(errs, "; "))
	}

	return &ProtectionResult{
		AlreadyActive: false,
		Targets:       targets,
		Details:       "Configured HKCU\\Environment & User Environment to " + EnvDisableAutoUpdate + "=1",
	}, nil
}

func enableUnixProtection() (*ProtectionResult, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, fmt.Errorf("could not determine user home directory: %v", err)
	}

	candidates := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}

	var existingFiles []string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			existingFiles = append(existingFiles, c)
		}
	}

	// Если файлов нет, выбираем целевой файл по текущему шеллу
	if len(existingFiles) == 0 {
		shell := os.Getenv("SHELL")
		if strings.Contains(shell, "fish") {
			existingFiles = append(existingFiles, filepath.Join(home, ".config", "fish", "config.fish"))
		} else if strings.Contains(shell, "zsh") {
			existingFiles = append(existingFiles, filepath.Join(home, ".zshrc"))
		} else {
			existingFiles = append(existingFiles, filepath.Join(home, ".bashrc"))
		}
	}

	var modified []string
	var alreadyConfigured []string

	for _, file := range existingFiles {
		changed, err := AppendEnvToShellFile(file, EnvDisableAutoUpdate, "1")
		if err != nil {
			return nil, fmt.Errorf("failed to update %s: %w", file, err)
		}
		if changed {
			modified = append(modified, file)
		} else {
			alreadyConfigured = append(alreadyConfigured, file)
		}
	}

	if len(modified) == 0 && len(alreadyConfigured) > 0 {
		return &ProtectionResult{
			AlreadyActive: true,
			Targets:       alreadyConfigured,
			Details:       "Already configured in: " + strings.Join(alreadyConfigured, ", "),
		}, nil
	}

	return &ProtectionResult{
		AlreadyActive: false,
		Targets:       modified,
		Details:       "Appended export " + EnvDisableAutoUpdate + "=1 to: " + strings.Join(modified, ", "),
	}, nil
}
