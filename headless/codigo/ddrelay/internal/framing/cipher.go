package framing

import "encoding/binary"

const (
	keySize      = 8
	checksumSeed = 119
	checksumMask = 0x7f7f
)

type cipherKey [keySize]byte

func startupKey() cipherKey {
	return cipherKey{174, 191, 86, 120, 171, 205, 239, 241}
}

func (key *cipherKey) decrypt(ciphertext []byte) []byte {
	plain := make([]byte, len(ciphertext))
	for i, b := range ciphertext {
		if i == 0 {
			plain[i] = b ^ key[0]
			continue
		}

		slot := i % keySize
		key[slot] = (key[slot] + ciphertext[i-1]) ^ byte(i)
		plain[i] = (b - ciphertext[i-1]) ^ key[slot]
	}

	return plain
}

func recoverKey(ciphertext []byte, zeroOffset int) cipherKey {
	var key cipherKey
	for i := zeroOffset; i < zeroOffset+keySize; i++ {

		value := ciphertext[i] - ciphertext[i-1]

		for j := i; j > 0; j -= keySize {
			value = (value ^ byte(j)) - ciphertext[j-1]
		}

		key[i%keySize] = value
	}

	return key
}

func encryptedLength(data []byte, key cipherKey) int {
	if len(data) < 4 {
		return 0
	}

	header := key.decrypt(data[:4])
	if binary.BigEndian.Uint16(header) != Magic {
		return 0
	}

	length := int(binary.BigEndian.Uint16(header[2:]))
	if length < HeaderSize {
		return 0
	}

	return length
}

func validChecksum(plain []byte) bool {
	sum := uint16(checksumSeed)
	for _, b := range plain[6:] {
		sum += uint16(b)
	}

	return binary.BigEndian.Uint16(plain[4:6]) == sum&checksumMask
}

func verifyKey(data []byte, key cipherKey) (bool, bool) {
	for range 2 {
		if len(data) < 4 {
			return false, true
		}

		length := encryptedLength(data, key)
		if length == 0 {
			return false, false
		}

		if len(data) < length {
			return false, true
		}

		if !validChecksum(key.decrypt(data[:length])) {
			return false, false
		}

		data = data[length:]
	}

	return true, false
}
