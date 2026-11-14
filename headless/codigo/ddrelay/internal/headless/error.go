package headless

import (
	"strconv"
	"strings"
	"time"
)

type Error struct {
	Operation  string
	Reason     string
	Cause      error `json:"-"`
	Kind       string
	Stage      string
	Host       string
	Path       string
	StatusCode int
	RayID      string
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	// Cause may contain a full URL, cookies or an upstream response. Keep it
	// available to errors.Is/As without copying it into ordinary log messages.
	parts := []string{"headless " + e.Operation + ": " + e.Reason}
	for _, field := range []struct{ key, value string }{
		{"kind", e.Kind}, {"stage", e.Stage}, {"host", e.Host}, {"path", e.Path}, {"ray", e.RayID},
	} {
		if field.value != "" {
			parts = append(parts, field.key+"="+field.value)
		}
	}
	if e.StatusCode != 0 {
		parts = append(parts, "status="+strconv.Itoa(e.StatusCode))
	}
	return strings.Join(parts, " ")
}

func (e *Error) Unwrap() error {
	return e.Cause
}
