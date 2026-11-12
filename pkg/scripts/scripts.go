package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScriptDef описывает генерируемый командный файл
type ScriptDef struct {
	FileName string
	Command  string
	Comment  string
}

// DefaultScripts возвращает стандартный набор батников для Windows
func DefaultScripts() []ScriptDef {
	return []ScriptDef{
		{
			FileName: "patch.bat",
			Command:  "patch",
			Comment:  "Apply Antigravity patches to all detected components",
		},
		{
			FileName: "doctor.bat",
			Command:  "doctor",
			Comment:  "Run system and Google API connectivity diagnostics",
		},
		{
			FileName: "status.bat",
			Command:  "status",
			Comment:  "Check patch status of all components",
		},
		{
			FileName: "restore.bat",
			Command:  "restore",
			Comment:  "Restore original files from *.agybak backups",
		},
	}
}

func baseName(p string) string {
	p = filepath.Base(p)
	if idx := strings.LastIndex(p, `\`); idx != -1 {
		p = p[idx+1:]
	}
	return p
}

// BuildBatchContent формирует содержимое .bat файла с Windows CRLF, переходом в директорию скрипта, передачей аргументов и паузой
func BuildBatchContent(exeName, command, comment string) string {
	if exeName == "" {
		exeName = "agy-patcher.exe"
	}
	exeName = baseName(exeName)

	var execLine string
	if command != "" {
		execLine = fmt.Sprintf(`"%%~dp0%s" %s %%*`, exeName, command)
	} else {
		execLine = fmt.Sprintf(`"%%~dp0%s" %%*`, exeName)
	}

	lines := []string{
		"@echo off",
		"rem " + comment,
		`cd /d "%~dp0"`,
		execLine,
		"echo.",
		"pause",
		"",
	}

	return strings.Join(lines, "\r\n")
}

// GenerateBatchScripts создает или перезаписывает Windows .bat лаунчеры в указанной директории
func GenerateBatchScripts(dir, exeName string) ([]string, error) {
	if dir == "" {
		dir = "."
	}
	if exeName == "" {
		exeName = "agy-patcher.exe"
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create script directory %s: %w", dir, err)
	}

	var created []string
	for _, sc := range DefaultScripts() {
		targetPath := filepath.Join(dir, sc.FileName)
		content := BuildBatchContent(exeName, sc.Command, sc.Comment)
		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return created, fmt.Errorf("failed to write %s: %w", targetPath, err)
		}
		created = append(created, targetPath)
	}

	return created, nil
}

// EnsureBatchScripts проверяет наличие .bat файлов в директории и создает отсутствующие
func EnsureBatchScripts(dir, exeName string) ([]string, error) {
	if dir == "" {
		dir = "."
	}
	if exeName == "" {
		exeName = "agy-patcher.exe"
	}

	var created []string
	for _, sc := range DefaultScripts() {
		targetPath := filepath.Join(dir, sc.FileName)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			content := BuildBatchContent(exeName, sc.Command, sc.Comment)
			if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
				return created, fmt.Errorf("failed to write %s: %w", targetPath, err)
			}
			created = append(created, targetPath)
		}
	}

	return created, nil
}
