package transport

import "fmt"

type Error struct {
	Operation string
	Cause     error
}

func (e *Error) Error() string { return fmt.Sprintf("DDTank transport %s: %v", e.Operation, e.Cause) }

func (e *Error) Unwrap() error { return e.Cause }
