package framing

import "bytes"

type EncryptedFramer struct {
	buffer  []byte
	key     cipherKey
	locked  bool
	startup bool
	plain   *Framer
	opaque  string
}

func NewEncryptedFramer() *EncryptedFramer {
	return &EncryptedFramer{startup: true}
}

func (f *EncryptedFramer) Push(data []byte) []Segment {
	if f.plain != nil {
		return f.plain.Push(data)
	}
	if f.opaque != "" {
		return rawSegments(data, f.opaque)
	}

	f.buffer = append(f.buffer, data...)
	var segments []Segment
	for len(f.buffer) > 0 {
		if !f.locked {
			ready, pending := f.detect()
			if f.plain != nil {
				segments = append(segments, f.plain.Push(f.buffer)...)
				f.buffer = nil
				break
			}
			if pending {
				break
			}
			if !ready {
				if f.startup {
					f.plain = NewFramer()
					segments = append(segments, f.plain.Push(f.buffer)...)
					f.buffer = nil
					break
				}
				segments = append(segments, f.fail("encryption key unavailable or unconfirmed")...)
				break
			}
		}

		if len(f.buffer) < 4 {
			break
		}
		length := encryptedLength(f.buffer, f.key)
		if length == 0 {
			segments = append(segments, f.fail("encrypted header invalid; decryption stopped")...)
			break
		}
		if len(f.buffer) < length {
			break
		}

		key := f.key
		plain := key.decrypt(f.buffer[:length])
		if !validChecksum(plain) {
			segments = append(segments, f.fail("encrypted checksum invalid; decryption stopped")...)
			break
		}

		segment := framed(plain)
		segment.Decrypted = true
		segments = append(segments, segment)
		f.buffer = f.buffer[length:]
		f.key = key
		if f.startup && segment.Frame.Header.Code == 1 {
			f.locked = false
		}
		f.startup = false
	}

	f.buffer = bytes.Clone(f.buffer)

	return segments
}

func (f *EncryptedFramer) Drain() []Segment {
	if f.plain != nil {
		return f.plain.Drain()
	}

	segments := rawSegments(f.buffer, "incomplete packet or unconfirmed encryption key")
	f.buffer = nil

	return segments
}

func (f *EncryptedFramer) Gap() []Segment {
	segments := f.Drain()
	f.key = cipherKey{}
	f.locked = false
	if f.plain == nil {
		f.opaque = "decryption unavailable after a reassembly gap"
	}

	return segments
}

func (f *EncryptedFramer) detect() (bool, bool) {
	if len(f.buffer) < 4 {
		return false, true
	}
	if f.startup && bytes.HasPrefix(f.buffer, magicBytes) {
		f.plain = NewFramer()
		return false, false
	}
	if len(f.buffer) < HeaderSize {
		return false, true
	}

	if f.startup {
		key := startupKey()
		length := encryptedLength(f.buffer, key)
		if length > len(f.buffer) {
			return false, true
		}
		if length > 0 && validChecksum(key.decrypt(f.buffer[:length])) {
			f.key = startupKey()
			f.locked = true
			return true, false
		}
	}

	var candidate cipherKey
	found, pending := false, false
	for _, offset := range []int{8, 12} {
		key := recoverKey(f.buffer, offset)
		valid, incomplete := verifyKey(f.buffer, key)
		pending = pending || incomplete
		if !valid {
			continue
		}
		if found && candidate != key {
			return false, false
		}
		candidate, found = key, true
	}
	if !found {
		return false, pending
	}

	f.key, f.locked, f.startup = candidate, true, false

	return true, false
}

func (f *EncryptedFramer) fail(reason string) []Segment {
	segments := rawSegments(f.buffer, reason)
	f.buffer = nil
	f.key = cipherKey{}
	f.locked = false
	f.opaque = reason

	return segments
}

func rawSegments(data []byte, note string) []Segment {
	if len(data) == 0 {
		return nil
	}

	return []Segment{{Kind: UnframedSegment, Raw: bytes.Clone(data), Note: note}}
}
