package app

type ProxyConfigError struct {
	Reason string
	Cause  error
}

func (e *ProxyConfigError) Error() string { return "proxy: " + e.Reason }

func (e *ProxyConfigError) Unwrap() error { return e.Cause }
