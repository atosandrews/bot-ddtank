package action

import "fmt"

type InvalidCommandError struct {
	Name, Reason string
	Cause        error
}

func (e *InvalidCommandError) Error() string {
	return fmt.Sprintf("invalid command %s: %s", e.Name, e.Reason)
}

func (e *InvalidCommandError) Unwrap() error { return e.Cause }
