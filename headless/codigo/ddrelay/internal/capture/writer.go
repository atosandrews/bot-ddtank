package capture

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	directoryMode = 0o700
	fileMode      = 0o600
	fileTimeStamp = "20060102-150405.000000000"
)

type CaptureExistsError struct {
	Path string
}

func (e *CaptureExistsError) Error() string {
	return fmt.Sprintf("capture %s already exists; every session needs a new file", e.Path)
}

type Writer struct {
	file    *os.File
	encoder *json.Encoder
}

func OpenAppend(path string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), directoryMode); err != nil {
		return nil, fmt.Errorf("creating capture directory for %s: %w", path, err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, fileMode)
	if err != nil {
		return nil, fmt.Errorf("opening capture %s: %w", path, err)
	}

	return newWriter(file), nil
}

func CreateNew(directory string, now time.Time) (*Writer, error) {
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return nil, fmt.Errorf("creating capture directory %s: %w", directory, err)
	}

	path := filepath.Join(directory, fmt.Sprintf("session-%s.jsonl", now.Format(fileTimeStamp)))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_APPEND, fileMode)
	if errors.Is(err, os.ErrExist) {
		return nil, &CaptureExistsError{Path: path}
	}
	if err != nil {
		return nil, fmt.Errorf("creating capture %s: %w", path, err)
	}

	return newWriter(file), nil
}

func newWriter(file *os.File) *Writer {
	return &Writer{file: file, encoder: json.NewEncoder(file)}
}

func (w *Writer) Destination() string {
	return w.file.Name()
}

func (w *Writer) Write(record Record) error {
	if err := w.encoder.Encode(record); err != nil {
		return fmt.Errorf("writing capture record to %s: %w", w.file.Name(), err)
	}

	return nil
}

func (w *Writer) Close() error {
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("syncing capture %s: %w", w.file.Name(), err)
	}

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("closing capture %s: %w", w.file.Name(), err)
	}

	return nil
}
