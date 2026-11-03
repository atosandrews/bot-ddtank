package capture

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

const (
	initialLineSize = 64 << 10
	maxLineSize     = 16 << 20
)

type MalformedRecordError struct {
	Path string
	Line int
	Err  error
}

func (e *MalformedRecordError) Error() string {
	return fmt.Sprintf("capture %s line %d is not a valid record: %v", e.Path, e.Line, e.Err)
}

func (e *MalformedRecordError) Unwrap() error {
	return e.Err
}

func Read(path string, handle func(Record) error) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening capture %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, initialLineSize), maxLineSize)

	line := 0
	for scanner.Scan() {
		line++

		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return &MalformedRecordError{Path: path, Line: line, Err: err}
		}

		if err := handle(record); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading capture %s: %w", path, err)
	}

	return nil
}
