package transport

import "fmt"

type ControlError struct {
	Code, Reason string
	Cause        error
}

func (e *ControlError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Reason) }

func (e *ControlError) Unwrap() error { return e.Cause }
