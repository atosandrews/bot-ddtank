package projection

import "math"

type fields struct {
	data map[string]any
	err  error
}

func (f *fields) integer(name string) int32 {
	switch v := f.data[name].(type) {
	case int64:
		if v >= math.MinInt32 && v <= math.MaxInt32 {
			return int32(v)
		}
		f.err = &FieldError{Name: name}
		return 0
	case uint64:
		if v <= math.MaxInt32 {
			return int32(v)
		}
		f.err = &FieldError{Name: name}
		return 0
	case int32:
		return v
	case int8:
		return int32(v)
	case uint8:
		return int32(v)
	default:
		f.err = &FieldError{Name: name}
		return 0
	}
}

func (f *fields) text(name string) string {
	v, ok := f.data[name].(string)
	if !ok {
		f.err = &FieldError{Name: name}
	}

	return v
}

func (f *fields) flag(name string) bool {
	v, ok := f.data[name].(bool)
	if !ok {
		f.err = &FieldError{Name: name}
	}

	return v
}

func (f *fields) list(name string) []map[string]any {
	if rows, ok := f.data[name].([]any); ok {
		result := make([]map[string]any, 0, len(rows))

		for _, row := range rows {
			object, ok := row.(map[string]any)
			if !ok {
				f.err = &FieldError{Name: name}

				return nil
			}

			result = append(result, object)
		}

		return result
	}

	v, ok := f.data[name].([]map[string]any)
	if !ok {
		f.err = &FieldError{Name: name}
	}

	return v
}

func integerEqual(v any, want int32) bool {
	f := &fields{data: map[string]any{"value": v}}
	got := f.integer("value")

	return f.err == nil && got == want
}

func (f *fields) signedBytes(name string) []int8 {
	if value, ok := f.data[name].([]int8); ok {
		return value
	}

	rows, ok := f.data[name].([]any)
	if !ok {
		f.err = &FieldError{Name: name}

		return nil
	}

	result := make([]int8, len(rows))

	for i, row := range rows {
		child := &fields{data: map[string]any{"value": row}}
		n := child.integer("value")
		if child.err != nil || n < math.MinInt8 || n > math.MaxInt8 {
			f.err = &FieldError{Name: name}

			return nil
		}

		result[i] = int8(n)
	}

	return result
}
