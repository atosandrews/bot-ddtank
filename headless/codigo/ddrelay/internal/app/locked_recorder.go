package app

import (
	"sync"

	"ddbot/relay/internal/capture"
)

type lockedRecorder struct {
	mu       sync.Mutex
	recorder Recorder
}

func (r *lockedRecorder) Write(record capture.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recorder.Write(record)
}

func (r *lockedRecorder) Destination() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recorder.Destination()
}

func (r *lockedRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recorder.Close()
}
