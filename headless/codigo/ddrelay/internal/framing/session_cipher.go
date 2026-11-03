package framing

type SessionCipher struct {
	key cipherKey
}

func NewSessionCipher(key [8]byte) *SessionCipher {
	return &SessionCipher{key: cipherKey(key)}
}

func NewSessionFramer(key [8]byte) *EncryptedFramer {
	return &EncryptedFramer{key: cipherKey(key), locked: true}
}

func (c *SessionCipher) Encode(frame Frame) ([]byte, error) {
	plain, err := Marshal(frame)
	if err != nil {
		return nil, err
	}

	return c.key.encrypt(plain), nil
}

func EncodeLogin(frame Frame) ([]byte, error) {
	if frame.Header.Code != 1 {
		return nil, &RelayError{Reason: "startup encryption requires a login frame"}
	}

	cipher := NewSessionCipher([8]byte(startupKey()))

	return cipher.Encode(frame)
}
