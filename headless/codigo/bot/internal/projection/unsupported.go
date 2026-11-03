package projection

type UnsupportedProjectionError struct{}

func (e *UnsupportedProjectionError) Error() string { return "packet has no state projection" }

func (e *UnsupportedProjectionError) Unwrap() error { return nil }
