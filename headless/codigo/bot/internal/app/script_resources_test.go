package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ddbot/bot/internal/script"
)

func TestLibraryDirectoryLoadsNestedModulesWithoutFollowingLinks(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "data"), 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "data", "value.lua"), []byte(`return {n=7}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}

	options, err := (ScriptResources{LibraryPath: root}).load()
	if err != nil {
		t.Fatal(err)
	}

	want := []script.Module{{Name: "data.value", Source: []byte(`return {n=7}`)}}
	if !reflect.DeepEqual(options.Library, want) {
		t.Fatalf("library = %#v", options.Library)
	}

	if err := os.Symlink(filepath.Join(root, "data", "value.lua"), filepath.Join(root, "escape.lua")); err != nil {
		t.Fatal(err)
	}

	if _, err := (ScriptResources{LibraryPath: root}).load(); err == nil {
		t.Fatal("accepted symlink")
	}
}
