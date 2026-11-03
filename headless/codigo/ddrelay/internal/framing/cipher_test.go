package framing

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestCipherVectors(t *testing.T) {
	tests := []struct {
		name       string
		key        cipherKey
		ciphertext string
		plaintext  string
		next       cipherKey
	}{
		{
			name:       "first packet",
			key:        cipherKey{3, 19, 47, 61, 89, 103, 131, 157},
			ciphertext: "72a1731e916753306b517aa0bf70c2781b2b71f5f6a693ed",
			plaintext:  "71ab0018002b002a00000000000000000000000001020304",
			next:       cipherKey{0xa3, 0x10, 0x46, 0x84, 0, 0xb2, 0xee, 0x5e},
		},
		{
			name:       "continued key with packet index reset",
			key:        cipherKey{0xa3, 0x10, 0x46, 0x84, 0, 0xb2, 0xee, 0x5e},
			ciphertext: "d21a7c9b3ad29847292ec888b344487eeed04863fd97266e303ee807",
			plaintext:  "71ab001c0071005e0000000000000000000000000001020304050607",
			next:       cipherKey{0xc6, 0x0b, 0xac, 0x18, 0x9a, 0x9b, 0x8d, 0x4b},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := hex.DecodeString(tt.ciphertext)
			if err != nil {
				t.Fatal(err)
			}
			plaintext, err := hex.DecodeString(tt.plaintext)
			if err != nil {
				t.Fatal(err)
			}
			original := bytes.Clone(ciphertext)
			key := tt.key
			if got := key.decrypt(ciphertext); !bytes.Equal(got, plaintext) {
				t.Fatalf("plaintext = %x, want %x", got, plaintext)
			}
			if key != tt.next {
				t.Fatalf("next key = %x, want %x", key, tt.next)
			}
			if !bytes.Equal(ciphertext, original) {
				t.Fatal("ciphertext mutated")
			}
			for _, offset := range []int{8, 12} {
				if got := recoverKey(ciphertext, offset); got != tt.key {
					t.Fatalf("recovered key at offset %d = %x, want %x", offset, got, tt.key)
				}
			}
		})
	}
}
