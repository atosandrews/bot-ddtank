package packets

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
	"unicode/utf8"
)

const maxEntries = 10000

type reader struct {
	data   []byte
	offset int
	err    error
}

func (r *reader) take(field string, n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.data)-r.offset {
		r.err = &DecodeError{Field: field, Offset: r.offset, Err: io.ErrUnexpectedEOF}
		return nil
	}
	data := r.data[r.offset : r.offset+n]
	r.offset += n
	return data
}

func (r *reader) fail(field string, err error) {
	if r.err == nil {
		r.err = &DecodeError{Field: field, Offset: r.offset, Err: err}
	}
}

func (r *reader) u8(field string) uint8 {
	data := r.take(field, 1)
	if data == nil {
		return 0
	}
	return data[0]
}

func (r *reader) i8(field string) int8 { return int8(r.u8(field)) }

func (r *reader) boolean(field string) bool { return r.u8(field) != 0 }

func (r *reader) u16(field string) uint16 {
	data := r.take(field, 2)
	if data == nil {
		return 0
	}
	return binary.BigEndian.Uint16(data)
}

func (r *reader) i32(field string) int32 {
	data := r.take(field, 4)
	if data == nil {
		return 0
	}
	return int32(binary.BigEndian.Uint32(data))
}

func (r *reader) long(field string) int64 {
	high := int64(r.i32(field + ".high"))
	low := int64(uint32(r.i32(field + ".low")))
	if high < 0 {
		if high == math.MinInt32 && low != 0 {
			r.fail(field, ErrInvalid)
			return 0
		}
		return high*(1<<32) - low
	}
	return high*(1<<32) + low
}

func (r *reader) utf(field string) string {
	length := int(r.u16(field + ".length"))
	data := r.take(field, length)
	if !utf8.Valid(data) {
		r.fail(field, fmt.Errorf("%w: invalid UTF-8", ErrInvalid))
		return ""
	}
	return string(data)
}

func (r *reader) count(field string, minBytes int) int {
	count := int(r.i32(field))
	if r.err != nil {
		return 0
	}
	if count < 0 || count > maxEntries || minBytes > 0 && count > (len(r.data)-r.offset)/minBytes {
		r.fail(field, fmt.Errorf("%w: count exceeds remaining body or limit", ErrInvalid))
		return 0
	}
	return count
}

func (r *reader) date(field string) string {
	year := int(r.u16(field + ".year"))
	month := time.Month(r.u8(field + ".month"))
	day := int(r.u8(field + ".day"))
	hour := int(r.u8(field + ".hour"))
	minute := int(r.u8(field + ".minute"))
	second := int(r.u8(field + ".second"))
	value := time.Date(year, month, day, hour, minute, second, 0, time.UTC)
	if value.Year() != year || value.Month() != month || value.Day() != day || value.Hour() != hour || value.Minute() != minute || value.Second() != second {
		r.fail(field, fmt.Errorf("%w: invalid date components", ErrInvalid))
	}
	return value.Format("2006-01-02T15:04:05")
}
