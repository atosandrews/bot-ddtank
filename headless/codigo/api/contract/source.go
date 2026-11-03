package contract

func validateSource(id, source string, limit int) error {
	if len(id) == 0 || len(id) > 128 || len(source) == 0 || len(source) > limit {
		return &Error{Field: "script", Reason: "invalid request ID or source length"}
	}

	return nil
}
