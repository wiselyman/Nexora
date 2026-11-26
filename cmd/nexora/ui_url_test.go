package main

import "testing"

func TestConfigurationURLSeedsTheUserFacingLocalhostOrigin(t *testing.T) {
	got := configurationURL()
	want := "http://localhost:18080"
	if got != want {
		t.Fatalf("configurationURL() = %q, want %q", got, want)
	}
}
