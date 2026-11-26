//go:build linux

package startup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopQuoteRoundTripsReservedCharacters(t *testing.T) {
	input := `/home/普通用户/Apps/a b/$cash/100%/x"quoted"/back\slash/` + "`tick"
	encoded := desktopQuote(input)
	if !strings.Contains(encoded, "%%") {
		t.Fatalf("literal percent was not escaped from field-code expansion: %q", encoded)
	}
	got, err := decodeDesktopExecArgument(encoded)
	if err != nil {
		t.Fatalf("cannot decode generated Exec argument: %v", err)
	}
	if got != input {
		t.Fatalf("decoded Exec argument = %q, want %q (encoded %q)", got, input, encoded)
	}
}

func TestLinuxStartupEntryContainsResolvableExecArgument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Nexora.desktop")
	if err := writeEntry(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var execLine string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Exec=") {
			execLine = strings.TrimPrefix(line, "Exec=")
			break
		}
	}
	if execLine == "" {
		t.Fatal("generated desktop entry has no Exec line")
	}
	got, err := decodeDesktopExecArgument(execLine)
	if err != nil {
		t.Fatalf("cannot decode generated Exec line: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if got != exe {
		t.Fatalf("desktop Exec path = %q, want %q", got, exe)
	}
}

func decodeDesktopExecArgument(encoded string) (string, error) {
	if len(encoded) < 2 || encoded[0] != '"' || encoded[len(encoded)-1] != '"' {
		return "", os.ErrInvalid
	}
	value := encoded[1 : len(encoded)-1]
	var desktop strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			desktop.WriteByte(value[i])
			continue
		}
		if i+1 >= len(value) || value[i+1] != '\\' {
			return "", os.ErrInvalid
		}
		desktop.WriteByte('\\')
		i++
	}
	value = desktop.String()
	// Desktop Entry reserves percent-prefixed field codes in Exec values;
	// %% represents a literal percent before command argument parsing.
	value = strings.ReplaceAll(value, "%%", "%")
	var argument strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			argument.WriteByte(value[i])
			continue
		}
		if i+1 >= len(value) {
			return "", os.ErrInvalid
		}
		i++
		switch value[i] {
		case '\\', '"', '`', '$':
			argument.WriteByte(value[i])
		default:
			return "", os.ErrInvalid
		}
	}
	return argument.String(), nil
}
