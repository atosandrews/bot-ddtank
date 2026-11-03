package app

import (
	"time"

	"ddbot/relay/internal/capture"
)

type Recorder interface {
	Write(capture.Record) error
	Destination() string
	Close() error
}

func openRecorder(config ProxyConfig, now time.Time) (Recorder, error) {
	if config.RecordAuto {
		return capture.CreateNew(config.CaptureDir, now)
	}

	if config.RecordPath != "" {
		return capture.OpenAppend(config.RecordPath)
	}

	return discardRecorder{}, nil
}
