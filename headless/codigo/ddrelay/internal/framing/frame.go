package framing

import (
	"bytes"
	"encoding/binary"
)

const (
	Magic      uint16 = 0x71AB
	HeaderSize        = 20
)

type SegmentKind string

const (
	FrameSegment    SegmentKind = "frame"
	UnframedSegment SegmentKind = "unframed"
)

type Header struct {
	Length     uint16
	Checksum   uint16
	Code       int16
	ClientID   int32
	Parameter1 int32
	Parameter2 int32
}

type Frame struct {
	Header Header
	Body   []byte
}

type Segment struct {
	Kind      SegmentKind
	Frame     Frame
	Raw       []byte
	Decrypted bool
	Note      string
}

func framed(data []byte) Segment {
	return Segment{
		Kind: FrameSegment,
		Frame: Frame{
			Header: Header{
				Length:     binary.BigEndian.Uint16(data[2:4]),
				Checksum:   binary.BigEndian.Uint16(data[4:6]),
				Code:       int16(binary.BigEndian.Uint16(data[6:8])),
				ClientID:   int32(binary.BigEndian.Uint32(data[8:12])),
				Parameter1: int32(binary.BigEndian.Uint32(data[12:16])),
				Parameter2: int32(binary.BigEndian.Uint32(data[16:20])),
			},
			Body: bytes.Clone(data[HeaderSize:]),
		},
		Raw: bytes.Clone(data),
	}
}

func unframed(data []byte) Segment {
	return Segment{Kind: UnframedSegment, Raw: bytes.Clone(data)}
}
