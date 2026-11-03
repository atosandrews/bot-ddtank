package value

func Copy[T any](source *T) *T {
	if source == nil {
		return nil
	}

	value := *source

	return &value
}

func Of[T any](value T) *T { return &value }
