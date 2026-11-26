//go:build darwin || linux

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectTunnelClientVersionAndFailureCases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		output  string
		want    string
		wantErr string
	}{
		{name: "minimum supported", output: "tunnel-client v0.0.17\n", want: "0.0.17"},
		{name: "newer supported", output: "OpenAI tunnel-client 1.2.3+build.4\n", want: "1.2.3"},
		{name: "too old", output: "tunnel-client v0.0.16\n", want: "0.0.16", wantErr: "版本过旧"},
		{name: "unrecognized", output: "not the official client\n", wantErr: "无法识别"},
		{name: "too much output", output: strings.Repeat("x", 5000), wantErr: "输出过长"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tunnel-client")
			body := "#!/bin/sh\nprintf '%s' '" + strings.ReplaceAll(tc.output, "'", "'\\''") + "'\n"
			if err := os.WriteFile(path, []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			got, err := inspectTunnelClient(path)
			if tc.wantErr == "" {
				if err != nil || got != tc.want {
					t.Fatalf("inspectTunnelClient() = %q, %v; want %q, nil", got, err, tc.want)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("inspectTunnelClient() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
