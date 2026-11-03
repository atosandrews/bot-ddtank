package app

import "ddbot/relay/internal/capture"

type discardRecorder struct{}

func (discardRecorder) Write(capture.Record) error {
	return nil
}

func (discardRecorder) Destination() string {
	return "disabled"
}

func (discardRecorder) Close() error {
	return nil
}
