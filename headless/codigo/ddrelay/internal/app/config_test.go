package app

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestControlModesUploadSourceOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script.lua")
	if err := os.WriteFile(path, []byte(`return fight.state()`), 0600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name              string
		args              []string
		operation, source string
		valid             bool
	}{
		{"evaluate", []string{"-session", "2", "-eval", "return 1"}, "eval", "return 1", true},
		{"file", []string{"-session", "2", "-eval-file", path}, "eval", `return fight.state()`, true},
		{"load", []string{"-session", "2", "-load-script", path}, "load", `return fight.state()`, true},
		{"stop", []string{"-session", "2", "-stop-script"}, "stop", "", true},
		{"sessions", []string{"-sessions"}, "sessions", "", true},
		{"mixed", []string{"-session", "2", "-eval", "return 1", "-stop-script"}, "", "", false},
		{"missing session", []string{"-eval-file", path}, "", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, err := parseControlConfig(test.args, io.Discard)
			if (err == nil) != test.valid {
				t.Fatalf("config %v", err)
			}

			if test.valid && (c.Operation != test.operation || c.Code != test.source) {
				t.Fatalf("operation=%s source=%s", c.Operation, c.Code)
			}
		})
	}
}
