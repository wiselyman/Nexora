//go:build windows

package startup

import (
	"encoding/base64"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestPowerShellEncodingRoundTripsUnicode(t *testing.T) {
	input := "$path='C:\\Users\\用户\\Apps\\📦\\Nexora.exe';Write-Output 'safe'"
	encoded := encodePowerShell(input)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(data)%2 != 0 {
		t.Fatalf("UTF-16LE byte length is odd: %d", len(data))
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	if got := string(utf16.Decode(units)); got != input {
		t.Fatalf("decoded PowerShell input = %q, want %q", got, input)
	}
}

func TestShortcutScriptQuotesUserPathsAsPowerShellLiterals(t *testing.T) {
	path := `C:\Users\O'Neil\AppData\Startup\Nexora.lnk`
	exe := `C:\Apps\Nexora $Tools\Nexora.exe`
	script := shortcutScript(path, exe)
	for _, want := range []string{
		"CreateShortcut(" + psQuote(path) + ")",
		"$s.TargetPath=" + psQuote(exe),
		"$s.WorkingDirectory=" + psQuote(`C:\Apps\Nexora $Tools`),
		"$s.IconLocation=" + psQuote(exe+",0"),
	} {
		if !strings.Contains(script, want) {
			t.Errorf("shortcut script missing safely quoted value %q: %s", want, script)
		}
	}
	if strings.Contains(script, "O'Neil") {
		t.Fatal("raw apostrophe appeared inside a PowerShell single-quoted literal")
	}
}

func TestEncodePowerShellMatchesUTF16CodeUnits(t *testing.T) {
	input := "A😀Z"
	got, err := base64.StdEncoding.DecodeString(encodePowerShell(input))
	if err != nil {
		t.Fatal(err)
	}
	wantUnits := utf16.Encode([]rune(input))
	want := make([]byte, len(wantUnits)*2)
	for i, unit := range wantUnits {
		binary.LittleEndian.PutUint16(want[i*2:], unit)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("encoded bytes = %v, want %v", got, want)
	}
}
