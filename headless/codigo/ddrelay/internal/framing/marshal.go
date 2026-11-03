package framing

import "encoding/binary"

func Marshal(frame Frame) ([]byte, error) {
	length := HeaderSize + len(frame.Body)
	if length > 65535 {
		return nil, &RelayError{Reason: "frame exceeds 65535 bytes"}
	}
	data := make([]byte, length)
	binary.BigEndian.PutUint16(data, Magic)
	binary.BigEndian.PutUint16(data[2:], uint16(length))
	binary.BigEndian.PutUint16(data[6:], uint16(frame.Header.Code))
	binary.BigEndian.PutUint32(data[8:], uint32(frame.Header.ClientID))
	binary.BigEndian.PutUint32(data[12:], uint32(frame.Header.Parameter1))
	binary.BigEndian.PutUint32(data[16:], uint32(frame.Header.Parameter2))
	copy(data[HeaderSize:], frame.Body)
	sum := uint16(checksumSeed)
	for _, b := range data[6:] {
		sum += uint16(b)
	}
	binary.BigEndian.PutUint16(data[4:], sum&checksumMask)
	return data, nil
}

func (key *cipherKey) encrypt(plain []byte) []byte {
	wire := make([]byte, len(plain))
	for i, b := range plain {
		if i == 0 {
			wire[i] = b ^ key[0]
			continue
		}
		slot := i % keySize
		key[slot] = (key[slot] + wire[i-1]) ^ byte(i)
		wire[i] = (b ^ key[slot]) + wire[i-1]
	}
	return wire
}
