package fileaccess

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormalizeRoots accepts only existing directories and stores their resolved,
// absolute paths so symlink aliases do not expand the user's grant later.
func NormalizeRoots(roots []string) ([]string, error) {
	clean := make([]string, 0, len(roots))
	seen := make(map[string]struct{}, len(roots))
	for _, raw := range roots {
		root := strings.TrimSpace(raw)
		if root == "" {
			continue
		}
		if strings.ContainsAny(root, "\x00\r\n") {
			return nil, errors.New("folder path contains an invalid character")
		}
		if root == "~" || strings.HasPrefix(root, "~/") || strings.HasPrefix(root, "~\\") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("cannot find the current user's home folder: %w", err)
			}
			if root == "~" {
				root = home
			} else {
				root = filepath.Join(home, root[2:])
			}
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("invalid folder path: %w", err)
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("cannot open selected folder %q: %w", root, err)
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("selected path is not an accessible folder: %s", root)
		}
		resolved = filepath.Clean(resolved)
		if _, exists := seen[resolved]; exists {
			continue
		}
		seen[resolved] = struct{}{}
		clean = append(clean, resolved)
	}
	return clean, nil
}

// Authorize resolves a requested path through existing symlinks and verifies
// that the result is inside one of the configured roots. For writes, the final
// file may not exist yet; its nearest existing ancestor is resolved instead.
func Authorize(roots []string, requested string, forWrite bool) (string, error) {
	if len(roots) == 0 {
		return "", errors.New("add at least one allowed folder in Nexora settings")
	}
	if strings.TrimSpace(requested) == "" || strings.ContainsRune(requested, '\x00') {
		return "", errors.New("a valid file path is required")
	}
	abs, err := filepath.Abs(requested)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %w", err)
	}
	resolved, err := resolvePath(abs, forWrite)
	if err != nil {
		return "", err
	}
	for _, configuredRoot := range roots {
		root, err := filepath.EvalSymlinks(configuredRoot)
		if err != nil {
			continue
		}
		root = filepath.Clean(root)
		if isWithin(root, resolved) {
			return resolved, nil
		}
	}
	return "", errors.New("requested path is outside the allowed folders")
}

// OpenAuthorized opens the configured root containing requested and returns a
// path relative to that root. All subsequent filesystem operations should use
// the returned Root so symlink changes cannot redirect access outside it.
func OpenAuthorized(roots []string, requested string) (*os.Root, string, error) {
	if len(roots) == 0 {
		return nil, "", errors.New("add at least one allowed folder in Nexora settings")
	}
	if strings.TrimSpace(requested) == "" || strings.ContainsAny(requested, "\x00\r\n") {
		return nil, "", errors.New("a valid file path is required")
	}
	abs, err := filepath.Abs(requested)
	if err != nil {
		return nil, "", fmt.Errorf("invalid file path: %w", err)
	}
	abs = filepath.Clean(abs)
	for _, configuredRoot := range roots {
		rootPath, err := filepath.EvalSymlinks(configuredRoot)
		if err != nil {
			continue
		}
		rootPath = filepath.Clean(rootPath)
		rel, err := filepath.Rel(rootPath, abs)
		if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		root, err := os.OpenRoot(rootPath)
		if err != nil {
			return nil, "", fmt.Errorf("cannot open allowed folder: %w", err)
		}
		return root, rel, nil
	}
	return nil, "", errors.New("requested path is outside the allowed folders")
}

func resolvePath(path string, allowMissing bool) (string, error) {
	if !allowMissing {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("cannot resolve file path: %w", err)
		}
		return filepath.Clean(resolved), nil
	}
	candidate := filepath.Clean(path)
	suffix := make([]string, 0, 4)
	for {
		_, err := os.Lstat(candidate)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("cannot inspect file path: %w", err)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", errors.New("cannot resolve file path")
		}
		suffix = append(suffix, filepath.Base(candidate))
		candidate = parent
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("cannot resolve file path: %w", err)
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, suffix[i])
	}
	return filepath.Clean(resolved), nil
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
