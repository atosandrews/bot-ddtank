package headless

import (
	"encoding/binary"
	"time"
)

func timestamp(now time.Time) []byte {
	data := make([]byte, 7)
	binary.BigEndian.PutUint16(data, uint16(now.Year()))
	data[2] = byte(now.Month())
	data[3] = byte(now.Day())
	data[4] = byte(now.Hour())
	data[5] = byte(now.Minute())
	data[6] = byte(now.Second())

	return data
}
