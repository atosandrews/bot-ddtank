package framing

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestEncryptedFramer(t *testing.T) {
	first := testPacket(42, []byte{1, 2, 3, 4})
	second := testPacket(94, bytes.Repeat([]byte{0xff, 0, 0x71, 0xab}, 100))
	login := testPacket(1, []byte("synthetic login"))
	key := cipherKey{3, 19, 47, 61, 89, 103, 131, 157}
	encrypted := encryptPackets(key, first, second)
	withLogin := append(encryptPackets(startupKey(), login), encrypted...)
	want := []Segment{decryptedSegment(first), decryptedSegment(second)}

	tests := []struct {
		name  string
		data  []byte
		chunk int
		want  []Segment
	}{
		{name: "coalesced", data: encrypted, chunk: len(encrypted), want: want},
		{name: "bytewise", data: encrypted, chunk: 1, want: want},
		{name: "split frames", data: encrypted, chunk: 13, want: want},
		{name: "startup to session", data: withLogin, chunk: 7, want: append([]Segment{decryptedSegment(login)}, want...)},
		{name: "coalesced login and session", data: withLogin, chunk: len(withLogin), want: append([]Segment{decryptedSegment(login)}, want...)},
		{name: "plaintext", data: first, chunk: 1, want: []Segment{framed(first)}},
		{name: "policy then plaintext", data: append([]byte("<policy-file-request/>\x00"), first...), chunk: len(first) + 23, want: []Segment{unframed([]byte("<policy-file-request/>\x00")), framed(first)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := bytes.Clone(tt.data)
			framer := NewEncryptedFramer()
			var got []Segment
			for i := 0; i < len(tt.data); i += tt.chunk {
				got = append(got, framer.Push(tt.data[i:min(i+tt.chunk, len(tt.data))])...)
			}
			got = append(got, framer.Drain()...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("segments = %#v, want %#v", got, tt.want)
			}
			if !bytes.Equal(original, tt.data) {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestEncryptedFramerRejectsUnverifiedData(t *testing.T) {
	first := testPacket(42, []byte{1, 2, 3, 4})
	second := testPacket(94, []byte{5, 6, 7, 8})
	key := cipherKey{3, 19, 47, 61, 89, 103, 131, 157}
	corrupt := encryptPackets(key, first, second)
	corrupt[len(corrupt)-1] ^= 1
	nonzero := bytes.Clone(first)
	for i := 8; i < HeaderSize; i++ {
		nonzero[i] = byte(i)
	}
	sum := uint16(119)
	for _, b := range nonzero[6:] {
		sum += uint16(b)
	}
	binary.BigEndian.PutUint16(nonzero[4:], sum&0x7f7f)

	tests := []struct {
		name string
		data []byte
	}{
		{name: "only one candidate frame", data: encryptPackets(key, first)},
		{name: "truncated second frame", data: encryptPackets(key, first, second)[:len(first)+10]},
		{name: "second checksum corrupt", data: corrupt},
		{name: "zero field hypothesis wrong", data: encryptPackets(key, nonzero, second)},
		{name: "flash policy", data: []byte("<policy-file-request/>\x00")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			framer := NewEncryptedFramer()
			got := append(framer.Push(tt.data), framer.Drain()...)
			var raw []byte
			for _, segment := range got {
				if segment.Kind != UnframedSegment || segment.Decrypted {
					t.Fatalf("unverified segment = %#v", segment)
				}
				raw = append(raw, segment.Raw...)
			}
			if !bytes.Equal(raw, tt.data) {
				t.Fatal("unverified bytes lost or changed")
			}
		})
	}
}

func TestEncryptedFramerStopsAfterFailure(t *testing.T) {
	first := testPacket(42, []byte{1, 2, 3, 4})
	second := testPacket(94, []byte{5, 6, 7, 8})
	third := testPacket(21, []byte{9, 10, 11, 12})
	key := cipherKey{3, 19, 47, 61, 89, 103, 131, 157}
	data := encryptPackets(key, first, second, third)
	cut := len(first) + len(second)
	for _, name := range []string{"gap", "corruption"} {
		t.Run(name, func(t *testing.T) {
			framer := NewEncryptedFramer()
			if got := framer.Push(data[:cut]); !reflect.DeepEqual(got, []Segment{decryptedSegment(first), decryptedSegment(second)}) {
				t.Fatalf("initial packets = %#v", got)
			}
			tail := bytes.Clone(data[cut:])
			if name == "gap" {
				if got := framer.Gap(); len(got) != 0 {
					t.Fatalf("gap output = %#v", got)
				}
			} else {
				tail[len(tail)-1] ^= 1
			}
			for range 2 {
				got := framer.Push(tail)
				if len(got) != 1 || got[0].Kind != UnframedSegment || got[0].Note == "" || !bytes.Equal(got[0].Raw, tail) {
					t.Fatalf("failed stream output = %#v", got)
				}
			}
		})
	}
}

func encryptPackets(key cipherKey, packets ...[]byte) []byte {
	var result []byte
	for _, packet := range packets {
		ciphertext := make([]byte, len(packet))
		for i, plain := range packet {
			if i == 0 {
				ciphertext[i] = plain ^ key[0]
				continue
			}
			key[i%8] = byte((int(key[i%8]) + int(ciphertext[i-1])) ^ i)
			ciphertext[i] = byte(int(plain^key[i%8]) + int(ciphertext[i-1]))
		}
		result = append(result, ciphertext...)
	}
	return result
}

func decryptedSegment(packet []byte) Segment {
	segment := framed(packet)
	segment.Decrypted = true
	return segment
}
