package contract

import "fmt"

type Error struct {
	Field, Reason string
	Cause         error
}

func (e *Error) Error() string { return fmt.Sprintf("contract %s: %s", e.Field, e.Reason) }

func (e *Error) Unwrap() error { return e.Cause }
