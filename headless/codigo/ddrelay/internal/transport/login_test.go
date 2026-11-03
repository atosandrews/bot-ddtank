package transport

import (
	"bytes"
	"encoding/binary"
)

func loginBody() []byte {
	date := []byte{7, 234, 10, 14, 12, 34, 56}
	body := join(
		[]byte{0}, ints(1, 2, 3, 4, 5, 6, 7, 1, 8, 9, 10, 11, 12, 13, 14, 15, 1, 16),
		ints(17, 18), utf("Master"), ints(19), utf("Honor"), date,
		[]byte{2}, ints(20, 21), date, date, ints(22), date, []byte{1},
		ints(23, 24), utf("Title"), ints(25), []byte{0}, utf("Style&Colors"), utf("Skin"),
		ints(26), utf("Guild"), ints(27, 28), utf("Duty"), ints(29), utf("Chairman"), ints(30, 31), []byte{1},
		utf("private-question-one"), utf("private-question-two"), ints(32), utf("private-login"),
		ints(33), utf("Permission"), utf("Mission"), ints(34, 35, 36, 37), date, date,
		ints(38, 39, 40, 41), []byte{1}, ints(42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52), date,
		[]byte{1}, ints(53, 54), date, ints(55, 56, 57, 58), []byte{0}, utf("private-instance"),
	)

	return body
}

func ints(values ...int32) []byte {
	var body []byte

	for _, v := range values {
		body = binary.BigEndian.AppendUint32(body, uint32(v))
	}

	return body
}

func utf(value string) []byte {
	return append(binary.BigEndian.AppendUint16(nil, uint16(len(value))), value...)
}

func join(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
