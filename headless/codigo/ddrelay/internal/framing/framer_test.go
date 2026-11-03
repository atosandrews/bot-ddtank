package framing

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestFramer(t *testing.T) {
	packet := testPacket(42, []byte{1, 2, 3})
	header := Header{Length: 23, Checksum: binary.BigEndian.Uint16(packet[4:6]), Code: 42}
	wantFrame := Segment{Kind: FrameSegment, Raw: packet, Frame: Frame{Header: header, Body: []byte{1, 2, 3}}}

	tests := []struct {
		name   string
		chunks [][]byte
		want   []Segment
	}{
		{name: "whole", chunks: [][]byte{packet}, want: []Segment{wantFrame}},
		{name: "split magic and header", chunks: [][]byte{packet[:1], packet[1:3], packet[3:19], packet[19:]}, want: []Segment{wantFrame}},
		{name: "coalesced", chunks: [][]byte{bytes.Repeat(packet, 2)}, want: []Segment{wantFrame, wantFrame}},
		{name: "junk", chunks: [][]byte{append([]byte{9, 8}, packet...)}, want: []Segment{{Kind: UnframedSegment, Raw: []byte{9, 8}}, wantFrame}},
		{name: "partial at end", chunks: [][]byte{packet[:10]}, want: []Segment{{Kind: UnframedSegment, Raw: packet[:10]}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			framer := NewFramer()
			var got []Segment
			for _, chunk := range tt.chunks {
				got = append(got, framer.Push(chunk)...)
			}
			got = append(got, framer.Drain()...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("segments = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func testPacket(code int16, body []byte) []byte {
	packet := make([]byte, HeaderSize+len(body))
	binary.BigEndian.PutUint16(packet, Magic)
	binary.BigEndian.PutUint16(packet[2:], uint16(len(packet)))
	binary.BigEndian.PutUint16(packet[6:], uint16(code))
	copy(packet[HeaderSize:], body)
	sum := uint16(119)
	for _, b := range packet[6:] {
		sum += uint16(b)
	}
	binary.BigEndian.PutUint16(packet[4:], sum&0x7f7f)

	return packet
}
