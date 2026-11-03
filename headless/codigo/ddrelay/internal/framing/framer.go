package framing

import (
	"bytes"
	"encoding/binary"
)

var magicBytes = binary.BigEndian.AppendUint16(nil, Magic)

type Framer struct {
	buffer []byte
}

func NewFramer() *Framer {
	return &Framer{}
}

func (f *Framer) Push(data []byte) []Segment {
	f.buffer = append(f.buffer, data...)

	var segments []Segment
	for {
		segment, complete := f.next()
		if !complete {
			break
		}

		segments = append(segments, segment)
		f.buffer = f.buffer[len(segment.Raw):]
	}

	f.buffer = bytes.Clone(f.buffer)

	return segments
}

func (f *Framer) Drain() []Segment {
	if len(f.buffer) == 0 {
		return nil
	}

	segment := unframed(f.buffer)
	f.buffer = nil

	return []Segment{segment}
}

func (f *Framer) next() (Segment, bool) {
	if len(f.buffer) < len(magicBytes) {
		return Segment{}, false
	}

	if !bytes.HasPrefix(f.buffer, magicBytes) {
		return f.nextUnframed(), true
	}

	if len(f.buffer) < HeaderSize {
		return Segment{}, false
	}

	length := int(binary.BigEndian.Uint16(f.buffer[2:4]))
	if length < HeaderSize {
		return unframed(f.buffer[:len(magicBytes)]), true
	}

	if len(f.buffer) < length {
		return Segment{}, false
	}

	return framed(f.buffer[:length]), true
}

func (f *Framer) nextUnframed() Segment {
	next := bytes.Index(f.buffer[1:], magicBytes)
	if next >= 0 {
		return unframed(f.buffer[:next+1])
	}

	end := len(f.buffer)
	if f.buffer[end-1] == magicBytes[0] {
		end--
	}

	return unframed(f.buffer[:end])
}
