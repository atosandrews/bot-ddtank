package app

import "fmt"

type RuntimeError struct {
	Operation string
	Cause     error
}

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("bot runtime %s: %v", e.Operation, e.Cause)
}

func (e *RuntimeError) Unwrap() error { return e.Cause }
