package commands

import "fmt"

type EncodingError struct {
	Field  string
	Reason string
	Cause  error
}

func (e *EncodingError) Error() string {
	return fmt.Sprintf("encoding command field %s: %s", e.Field, e.Reason)
}

func (e *EncodingError) Unwrap() error { return e.Cause }
