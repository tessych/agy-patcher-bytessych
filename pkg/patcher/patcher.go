package patcher

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	bin "github.com/tessych/agy-patcher-bytessych/pkg/binary"
)

const BackupSuffix = ".agybak"

// AppTarget определяет тип патчимого приложения
type AppTarget string

const (
	TargetCLI       AppTarget = "cli"
	TargetManager   AppTarget = "manager"
	TargetIDE       AppTarget = "ide"
	TargetExtension AppTarget = "extension"
)

// Patcher управляет статусом, бэкапами и транзакционным патчингом
type Patcher struct {
	Target AppTarget
	Path   string
	Gate   *MultiGate
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	si, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, si.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func FilesEqual(aPath, bPath string) (bool, error) {
	a, err := os.Open(aPath)
	if err != nil {
		return false, err
	}
	defer a.Close()

	b, err := os.Open(bPath)
	if err != nil {
		return false, err
	}
	defer b.Close()

	aBuf := make([]byte, 64*1024)
	bBuf := make([]byte, 64*1024)

	for {
		na, ea := io.ReadFull(a, aBuf)
		nb, eb := io.ReadFull(b, bBuf)

		if !bytes.Equal(aBuf[:na], bBuf[:nb]) {
			return false, nil
		}
		if ea == io.EOF || ea == io.ErrUnexpectedEOF {
			return eb == io.EOF || eb == io.ErrUnexpectedEOF, nil
		}
		if ea != nil || eb != nil {
			return false, fmt.Errorf("read error: a=%v, b=%v", ea, eb)
		}
	}
}

// GetStatus returns target file status
func (p *Patcher) GetStatus() (GateStatus, *MatchInfo, error) {
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return StatusUnknown, nil, err
	}

	if p.Target == TargetIDE {
		info, err := ResolveIDE(data)
		if err != nil {
			return StatusUnknown, nil, err
		}
		return info.State, info, nil
	}

	info, err := bin.InspectFile(p.Path)
	if err != nil {
		return StatusUnknown, nil, err
	}

	match, err := p.Gate.Resolve(data, info.Ranges, info.Arch)
	if err != nil {
		return StatusUnknown, nil, err
	}
	return match.State, match, nil
}

// MakeBackup creates a *.agybak copy
func (p *Patcher) MakeBackup() (string, error) {
	bak := p.Path + BackupSuffix
	if _, err := os.Stat(bak); err == nil {
		equal, err := FilesEqual(p.Path, bak)
		if err == nil && equal {
			return bak, nil
		}
	}

	if err := CopyFile(p.Path, bak); err != nil {
		return "", fmt.Errorf("backup failed: %w", err)
	}

	equal, err := FilesEqual(p.Path, bak)
	if err != nil || !equal {
		_ = os.Remove(bak)
		return "", errors.New("backup verification failed (mismatch)")
	}

	return bak, nil
}

// ApplyPatch безопасно применяет патч с созданием бэкапа и автоматическим откатом
func (p *Patcher) ApplyPatch() error {
	status, match, err := p.GetStatus()
	if err != nil {
		return fmt.Errorf("cannot inspect %s: %w", p.Path, err)
	}

	if status == StatusPatched {
		return nil
	}
	if status != StatusUnpatched {
		return fmt.Errorf("file %s is in unrecognized state %s", p.Path, status)
	}

	bak, err := p.MakeBackup()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(p.Path)
	if err != nil {
		return err
	}

	var modified []byte
	if p.Target == TargetIDE {
		loc := IDERegex.FindIndex(data)
		if loc == nil {
			return errors.New("IDE gate vanished before write")
		}
		modified = append(data[:loc[0]], append(IDEFixBytes, data[loc[1]:]...)...)
	} else {
		modified = make([]byte, len(data))
		copy(modified, data)
		copy(modified[match.Offset:], match.Gate.Fix)
	}

	dir := filepath.Dir(p.Path)
	tmpFile, err := os.CreateTemp(dir, "patch_tmp_*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(modified); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write patch: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	fi, err := os.Stat(p.Path)
	if err == nil {
		_ = os.Chmod(tmpName, fi.Mode())
	}

	if err := os.Rename(tmpName, p.Path); err != nil {
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	newStatus, _, err := p.GetStatus()
	if err != nil || newStatus != StatusPatched {
		_ = CopyFile(bak, p.Path)
		return fmt.Errorf("post-patch verification failed, rolled back to original: %v", err)
	}

	if p.Target == TargetIDE {
		ClearIDECache()
	}

	return nil
}

// ClearIDECache cleans V8 bytecode caches for IDE
func ClearIDECache() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	bases := []string{
		filepath.Join(cfg, "Antigravity IDE"),
		filepath.Join(cfg, "antigravity-ide"),
		filepath.Join(cfg, "Antigravity"),
		filepath.Join(cfg, "antigravity"),
		filepath.Join(home, "Library", "Application Support", "Antigravity IDE"),
		filepath.Join(os.Getenv("APPDATA"), "Antigravity IDE"),
	}

	for _, b := range bases {
		_ = os.RemoveAll(filepath.Join(b, "CachedData"))
		_ = os.RemoveAll(filepath.Join(b, "Code Cache", "js"))
	}
}

// Restore восстанавливает файл из бэкапа *.agybak
func (p *Patcher) Restore() error {
	bak := p.Path + BackupSuffix
	if _, err := os.Stat(bak); os.IsNotExist(err) {
		return fmt.Errorf("no backup found at %s", bak)
	}

	if err := CopyFile(bak, p.Path); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	status, _, err := p.GetStatus()
	if err != nil || status != StatusUnpatched {
		return fmt.Errorf("restore verification failed (state=%s): %v", status, err)
	}

	return nil
}
