package headless

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAcceptsOptionalTransportAndRecoveryPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "headless.yaml")
	data := "session_file: session.json\nauth:\n  transport: launcher_chromium87\n  status_file: run/auth.json\n  coordination_dir: run/auth-locks\ncharacters:\n  - id: 123\n    website_id: 456\n    name: Fixture\n    upstream: 127.0.0.1:1401\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("valid transport config was rejected: %v", err)
	}
	if config.Auth.Transport != "launcher_chromium87" || config.Auth.StatusFile != filepath.Join(filepath.Dir(path), "run/auth.json") || config.Auth.CoordinationDir != filepath.Join(filepath.Dir(path), "run/auth-locks") {
		t.Fatalf("auth config paths = %#v", config.Auth)
	}
}

func TestLoadConfigDefaultsAndRejectsUnknownTransport(t *testing.T) {
	for _, tc := range []struct {
		name, auth string
		valid      bool
	}{
		{"legacy", "", true},
		{"status only", "auth:\n  status_file: auth.json\n", true},
		{"coordination only", "auth:\n  coordination_dir: coordination\n", true},
		{"unknown transport", "auth:\n  transport: random_profiles\n", false},
		{"unknown option", "auth:\n  disable_certificates: true\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "headless.yaml")
			data := "session_file: session.json\n" + tc.auth + "characters:\n  - id: 123\n    website_id: 456\n    name: Fixture\n    upstream: 127.0.0.1:1401\n"
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			c, err := LoadConfig(path)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if tc.valid && c.Auth.Transport != "standard" {
				t.Fatalf("legacy default = %q", c.Auth.Transport)
			}
		})
	}
}
