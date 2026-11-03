package app

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"ddbot/bot/internal/script"
)

type ScriptResources struct {
	LibraryPath string
}

func (r ScriptResources) load() (script.Options, error) {
	var options script.Options
	if r.LibraryPath != "" {
		total := 0
		err := filepath.WalkDir(r.LibraryPath, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return &script.Error{Operation: "read library", Cause: walkErr}
			}

			if entry.Type()&os.ModeSymlink != 0 {
				return &script.Error{Operation: "read library", Reason: "symlinks are not supported: " + path}
			}

			if entry.IsDir() || filepath.Ext(path) != ".lua" {
				return nil
			}

			if !entry.Type().IsRegular() {
				return &script.Error{Operation: "read library", Reason: "regular Lua file required: " + path}
			}

			relative, err := filepath.Rel(r.LibraryPath, path)
			if err != nil {
				return &script.Error{Operation: "read library", Cause: err}
			}

			source, err := readModule(path)
			if err != nil {
				return err
			}

			total += len(source)
			if total > script.MaxLibraryBytes || len(options.Library) >= script.MaxLibraryModules {
				return &script.Error{Operation: "read library", Reason: "library source limit exceeded"}
			}

			name := strings.ReplaceAll(strings.TrimSuffix(filepath.ToSlash(relative), ".lua"), "/", ".")
			options.Library = append(options.Library, script.Module{Name: name, Source: source})

			return nil
		})
		if err != nil {
			return options, err
		}
	}

	return options, nil
}

func readModule(path string) (data []byte, result error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &script.Error{Operation: "read module", Cause: err}
	}

	defer func() {
		if err := file.Close(); err != nil {
			result = errors.Join(result, &script.Error{Operation: "close module", Cause: err})
		}
	}()
	source, err := io.ReadAll(io.LimitReader(file, script.MaxModuleBytes+1))
	if err != nil {
		return nil, &script.Error{Operation: "read module", Cause: err}
	}

	if len(source) > script.MaxModuleBytes {
		return nil, &script.Error{Operation: "read module", Reason: "module exceeds 8 MiB"}
	}

	return source, nil
}
