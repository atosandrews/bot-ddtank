package app

import "fmt"

type ConfigurationError struct {
	Field  string
	Reason string
	Cause  error
}

func (e *ConfigurationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("bot configuration %s: %v", e.Field, e.Cause)
	}

	return fmt.Sprintf("bot configuration %s: %s", e.Field, e.Reason)
}

func (e *ConfigurationError) Unwrap() error { return e.Cause }
