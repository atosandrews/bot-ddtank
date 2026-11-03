package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationRejectsUnknownAndUnsafeValues(t *testing.T) {
	t.Setenv("BOT_TEST_KEY", "synthetic-key-for-config")
	base := "listen: 127.0.0.1:7443\nlua_dir: ''\ntenants:\n  - id: one\n    api_key_env: BOT_TEST_KEY\n"

	for _, test := range []struct {
		name, text string
		valid      bool
	}{{"valid", base, true}, {"unknown", base + "surprise: true\n", false}, {"second document", base + "---\nlisten: 127.0.0.1:7444\n", false}, {"remote plaintext", strings.Replace(base, "127.0.0.1", "0.0.0.0", 1), false}, {"zero sessions", base + "max_sessions: 0\n", false}, {"negative grace", base + "reconnect_grace: -1s\n", false}, {"oversize", base + "#" + strings.Repeat("x", 1<<20), false}} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(test.text), 0600); err != nil {
				t.Fatal(err)
			}

			_, keys, err := LoadConfig(path)
			if (err == nil) != test.valid {
				t.Fatalf("validation %v", err)
			}

			if test.valid && keys["synthetic-key-for-config"] != "one" {
				t.Fatalf("keys %v", keys)
			}
		})
	}
}
