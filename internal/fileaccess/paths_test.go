package fileaccess

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeRootsResolvesAliasesAndDeduplicates(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	roots, err := NormalizeRoots([]string{root, alias, "  ", filepath.Join(root, ".")})
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0] != canonicalRoot {
		t.Fatalf("normalized roots = %#v, want one canonical root %q", roots, canonicalRoot)
	}
}

func TestAuthorizeBlocksTraversalAndSymlinkEscapesForReadsAndWrites(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	insideFile := filepath.Join(root, "inside.txt")
	if err := os.WriteFile(insideFile, []byte("inside"), 0600); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "outside-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	for _, tc := range []struct {
		name     string
		path     string
		forWrite bool
		wantOK   bool
	}{
		{name: "inside read", path: insideFile, wantOK: true},
		{name: "inside new write", path: filepath.Join(root, "new.txt"), forWrite: true, wantOK: true},
		{name: "parent traversal read", path: filepath.Join(root, "..", "outside", "secret.txt")},
		{name: "parent traversal write", path: filepath.Join(root, "..", "outside", "new.txt"), forWrite: true},
		{name: "symlink read", path: filepath.Join(link, "secret.txt")},
		{name: "symlink write", path: filepath.Join(link, "new.txt"), forWrite: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Authorize([]string{root}, tc.path, tc.forWrite)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("Authorize() error = %v", err)
				}
				if got == "" {
					t.Fatal("Authorize() returned an empty path")
				}
			} else if err == nil {
				t.Fatalf("Authorize(%q) unexpectedly succeeded as %q", tc.path, got)
			}
		})
	}
}

func TestOpenAuthorizedAnchorsAccessAtAllowedRoot(t *testing.T) {
	base := t.TempDir()
	rootPath := filepath.Join(base, "root")
	outsidePath := filepath.Join(base, "outside")
	if err := os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outsidePath, 0700); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	root, rel, err := OpenAuthorized([]string{canonicalRoot}, filepath.Join(canonicalRoot, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if rel != "file.txt" {
		t.Fatalf("relative path = %q, want file.txt", rel)
	}
	if _, err := root.Open("../outside/secret.txt"); err == nil {
		t.Fatal("rooted filesystem handle permitted parent traversal")
	}

	if runtime.GOOS != "windows" {
		link := filepath.Join(rootPath, "outside-link")
		if err := os.Symlink(outsidePath, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := root.Open("outside-link/secret.txt"); err == nil {
			t.Fatal("rooted filesystem handle followed an escaping symlink")
		}
	}
}
