package headless

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Status is a private local heartbeat consumed by the process supervisor.
// Reasons contain classifications only; credentials and request URLs do not belong here.
type Status struct {
	Version    int               `json:"version"`
	PID        int               `json:"pid"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Characters []CharacterStatus `json:"characters"`
}

type CharacterStatus struct {
	ID              int32      `json:"id"`
	Phase           string     `json:"phase"`
	Reason          string     `json:"reason"`
	NextAttemptAt   *time.Time `json:"next_attempt_at,omitempty"`
	AttemptDeadline *time.Time `json:"attempt_deadline,omitempty"`
}

// WriteStatus atomically replaces a private snapshot. The caller owns its heartbeat.
func WriteStatus(path string, snapshot Status) error {
	if path == "" {
		return nil
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	if snapshot.PID == 0 {
		snapshot.PID = os.Getpid()
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now().UTC()
	}
	return writePrivateJSON(path, snapshot)
}

func privateDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("private regular directory required")
	}
	return nil
}

func writePrivateJSON(path string, value any) (result error) {
	directory := filepath.Dir(path)
	if err := privateDirectory(directory); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("private regular output required")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".auth-state-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
