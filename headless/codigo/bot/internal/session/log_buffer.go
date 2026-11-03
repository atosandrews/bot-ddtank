package session

import "unicode/utf8"

type logBuffer struct{ data []byte }

func (b *logBuffer) Write(data []byte) (int, error) {
	if !utf8.Valid(data) || len(data) > 8192-len(b.data) {
		return 0, &Error{Code: "log_limit", Reason: "logs require UTF-8 and at most 8 KiB per operation"}
	}

	b.data = append(b.data, data...)

	return len(data), nil
}

func (b *logBuffer) take() string { value := string(b.data); b.data = b.data[:0]; return value }
