package projection

import "fmt"

type FieldError struct{ Name string }

func (e *FieldError) Error() string {
	return fmt.Sprintf("decoded field %s has an unexpected type", e.Name)
}

func (e *FieldError) Unwrap() error { return nil }
