package redirect

import (
	"encoding/binary"
	"testing"

	"golang.org/x/sys/unix"
)

func TestOriginalDestinationRequiresIPv4GamePort(t *testing.T) {
	for _, tt := range []struct {
		name   string
		family uint16
		ip     [4]byte
		port   uint16
		want   string
	}{
		{"game", unix.AF_INET, [4]byte{192, 0, 2, 1}, 1401, "192.0.2.1:1401"},
		{"wrong port", unix.AF_INET, [4]byte{192, 0, 2, 1}, 443, ""},
		{"listener", unix.AF_INET, [4]byte{127, 0, 0, 1}, 1401, ""},
		{"IPv6", unix.AF_INET6, [4]byte{192, 0, 2, 1}, 1401, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, unix.SizeofSockaddrInet4)
			binary.NativeEndian.PutUint16(data, tt.family)
			binary.BigEndian.PutUint16(data[2:], tt.port)
			copy(data[4:], tt.ip[:])
			got, err := decodeDestination(data, []uint16{1401})
			if tt.want == "" {
				if err == nil {
					t.Fatal("accepted invalid destination")
				}

				return
			}

			if err != nil || got != tt.want {
				t.Fatalf("destination = %q, err = %v", got, err)
			}
		})
	}
}
