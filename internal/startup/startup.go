package startup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Apply creates or removes the current user's login startup entry. Removal is
// limited to an entry previously marked as managed by Nexora.
func Apply(enabled bool) error {
	entryPath, configDir, err := entryPaths()
	if err != nil {
		return err
	}
	return applyEntry(enabled, entryPath, configDir, writeEntry)
}

func applyEntry(enabled bool, entryPath, configDir string, write func(string) error) error {
	markerPath := filepath.Join(configDir, "startup-managed")
	marker, markerErr := os.ReadFile(markerPath)
	markerLines := strings.Split(strings.TrimSpace(string(marker)), "\n")
	owned := markerErr == nil && len(markerLines) > 0 && markerLines[0] == entryPath
	if markerErr != nil && !errors.Is(markerErr, os.ErrNotExist) {
		return fmt.Errorf("cannot read startup ownership marker: %w", markerErr)
	}
	if owned && len(markerLines) < 2 {
		return fmt.Errorf("startup ownership marker has no integrity digest; entry was left unchanged")
	}
	if enabled {
		var previous []byte
		var previousMode os.FileMode = 0600
		previousExists := false
		if !owned {
			if _, err := os.Lstat(entryPath); err == nil {
				return fmt.Errorf("startup entry already exists and is not managed by Nexora: %s", entryPath)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("cannot inspect startup entry: %w", err)
			}
		} else {
			if err := verifyEntry(entryPath, markerLines[1]); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("startup entry changed since Nexora created it; it was left unchanged: %w", err)
			}
			if info, err := os.Lstat(entryPath); err == nil {
				if !info.Mode().IsRegular() {
					return fmt.Errorf("startup entry is not a regular file; it was left unchanged")
				}
				previous, err = os.ReadFile(entryPath)
				if err != nil {
					return fmt.Errorf("cannot preserve existing startup entry: %w", err)
				}
				previousMode = info.Mode().Perm()
				previousExists = true
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("cannot inspect startup entry: %w", err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(entryPath), 0700); err != nil {
			return fmt.Errorf("cannot create startup folder: %w", err)
		}
		if err := write(entryPath); err != nil {
			return withRestoreError(err, restoreEntry(entryPath, previous, previousMode, previousExists))
		}
		digest, err := entryDigest(entryPath)
		if err != nil {
			cause := fmt.Errorf("cannot verify created startup entry: %w", err)
			return withRestoreError(cause, restoreEntry(entryPath, previous, previousMode, previousExists))
		}
		if err := os.MkdirAll(configDir, 0700); err != nil {
			cause := fmt.Errorf("cannot create Nexora settings folder: %w", err)
			return withRestoreError(cause, restoreEntry(entryPath, previous, previousMode, previousExists))
		}
		if err := atomicWrite(markerPath, []byte(entryPath+"\n"+digest+"\n"), 0600); err != nil {
			cause := fmt.Errorf("cannot record startup ownership: %w", err)
			return withRestoreError(cause, restoreEntry(entryPath, previous, previousMode, previousExists))
		}
		return nil
	}
	if !owned {
		return nil
	}
	if err := verifyEntry(entryPath, markerLines[1]); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if removeErr := os.Remove(markerPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return fmt.Errorf("cannot remove startup ownership marker: %w", removeErr)
			}
			return nil
		}
		return fmt.Errorf("startup entry changed since Nexora created it; it was left unchanged: %w", err)
	}
	if err := os.Remove(entryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot remove Nexora startup entry: %w", err)
	}
	if err := os.Remove(markerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot remove startup ownership marker: %w", err)
	}
	return nil
}

func verifyEntry(path, expectedDigest string) error {
	digest, err := entryDigest(path)
	if err != nil {
		return err
	}
	if digest != expectedDigest {
		return errors.New("integrity digest does not match")
	}
	return nil
}

func entryDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("startup entry is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func withRestoreError(cause, restoreErr error) error {
	if restoreErr != nil {
		return fmt.Errorf("%w; failed to restore previous startup entry: %v", cause, restoreErr)
	}
	return cause
}

func restoreEntry(path string, previous []byte, mode os.FileMode, existed bool) error {
	if existed {
		return atomicWrite(path, previous, mode)
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".Nexora-startup-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
