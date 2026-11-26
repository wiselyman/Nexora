//go:build darwin || linux

package startup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCreatesAndRemovesOwnedStartupEntry(t *testing.T) {
	root := isolateStartupHome(t)
	entry, config, err := entryPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(true); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Fatal("startup entry is empty")
	}
	marker, err := os.ReadFile(filepath.Join(config, "startup-managed"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(marker), entry+"\n") {
		t.Fatalf("ownership marker does not identify the created entry: %q", marker)
	}

	if err := Apply(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(entry); !os.IsNotExist(err) {
		t.Fatalf("owned entry remains after disable: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(config, "startup-managed")); !os.IsNotExist(err) {
		t.Fatalf("ownership marker remains after disable: %v", err)
	}
	if !strings.HasPrefix(entry, root+string(filepath.Separator)) {
		t.Fatalf("test wrote startup entry outside its temporary home: %s", entry)
	}
}

func TestApplyPreservesUnownedStartupEntry(t *testing.T) {
	isolateStartupHome(t)
	entry, config, err := entryPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(entry), 0700); err != nil {
		t.Fatal(err)
	}
	want := []byte("user-owned startup configuration\n")
	if err := os.WriteFile(entry, want, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Apply(true); err == nil || !strings.Contains(err.Error(), "not managed by Nexora") {
		t.Fatalf("Apply(true) error = %v, want unowned-entry error", err)
	}
	got, err := os.ReadFile(entry)
	if err != nil || string(got) != string(want) {
		t.Fatalf("unowned startup entry changed: %q, %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(config, "startup-managed")); !os.IsNotExist(err) {
		t.Fatalf("created ownership marker for unowned entry: %v", err)
	}
	if err := Apply(false); err != nil {
		t.Fatalf("disabling startup changed an unowned entry: %v", err)
	}
}

func TestApplyRefusesToOverwriteOrRemoveChangedOwnedEntry(t *testing.T) {
	isolateStartupHome(t)
	entry, config, err := entryPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(true); err != nil {
		t.Fatal(err)
	}
	modified := []byte("user changed this startup entry\n")
	if err := os.WriteFile(entry, modified, 0600); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		if err := Apply(enabled); err == nil || !strings.Contains(err.Error(), "changed since Nexora created it") {
			t.Fatalf("Apply(%v) error = %v, want changed-entry error", enabled, err)
		}
		got, err := os.ReadFile(entry)
		if err != nil || string(got) != string(modified) {
			t.Fatalf("modified startup entry changed: %q, %v", got, err)
		}
		if _, err := os.Stat(filepath.Join(config, "startup-managed")); err != nil {
			t.Fatalf("ownership marker was removed after refusing modified entry: %v", err)
		}
	}
}

func isolateStartupHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("APPDATA", filepath.Join(root, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	entry, _, err := entryPaths()
	if err != nil {
		t.Fatalf("resolve isolated startup entry: %v", err)
	}
	rel, err := filepath.Rel(root, entry)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("startup test path is outside its temporary home: %q (root %q)", entry, root)
	}
	return root
}
