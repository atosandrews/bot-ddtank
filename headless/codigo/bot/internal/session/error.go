package session

import "fmt"

type Error struct {
	Code, Reason string
	Cause        error
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Reason) }

func (e *Error) Unwrap() error { return e.Cause }
