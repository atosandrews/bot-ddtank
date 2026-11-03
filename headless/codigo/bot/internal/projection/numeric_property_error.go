package projection

import "fmt"

type NumericPropertyError struct {
	Property string
	Reason   string
	Cause    error
}

func (e *NumericPropertyError) Error() string {
	return fmt.Sprintf("%s property: %s", e.Property, e.Reason)
}

func (e *NumericPropertyError) Unwrap() error { return e.Cause }
