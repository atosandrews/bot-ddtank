package framing

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRelayPreservesCipherAfterInsertion(t *testing.T) {
	key := cipherKey{3, 19, 47, 61, 89, 103, 131, 157}
	first, second, third := testPacket(4, nil), testPacket(6, nil), testPacket(91, []byte{12, 0})
	original := encryptPackets(key, first, second, third)
	for _, chunk := range []int{1, 7, len(third)} {
		relay := NewRelay()
		prefix, err := relay.Forward(original[:len(first)+len(second)])
		if err != nil || !bytes.Equal(prefix, original[:len(first)+len(second)]) || !relay.Ready() {
			t.Fatalf("initial relay: %v", err)
		}
		injected, err := relay.Inject(Frame{Header: Header{Code: 94}, Body: []byte{0, 0, 0, 7}})
		if err != nil {
			t.Fatal(err)
		}
		wire := append(prefix, injected...)
		rest := original[len(first)+len(second):]
		for i := 0; i < len(rest); i += chunk {
			next, err := relay.Forward(rest[i:min(i+chunk, len(rest))])
			if err != nil {
				t.Fatal(err)
			}
			wire = append(wire, next...)
		}
		decoded := NewEncryptedFramer().Push(wire)
		if len(decoded) != 4 || decoded[2].Frame.Header.Code != 94 || !bytes.Equal(decoded[3].Raw, third) {
			t.Fatalf("decoded = %#v", decoded)
		}
	}
}

func TestRelayRefusesUnsafeInsertion(t *testing.T) {
	key := cipherKey{3, 19, 47, 61, 89, 103, 131, 157}
	packets := encryptPackets(key, testPacket(4, nil), testPacket(6, nil), testPacket(91, []byte{12, 0}))
	relay := NewRelay()
	if _, err := relay.Inject(Frame{}); err == nil {
		t.Fatal("injected before key recovery")
	}
	if _, err := relay.Forward(packets[:41]); err != nil {
		t.Fatal(err)
	}
	if _, err := relay.Inject(Frame{}); err == nil {
		t.Fatal("injected into partial packet")
	}
	if _, err := relay.Forward(packets[41:]); err != nil {
		t.Fatal(err)
	}
	if _, err := relay.Inject(Frame{Header: Header{Code: 94}}); err != nil {
		t.Fatal(err)
	}
	if _, err := relay.Forward(bytes.Repeat([]byte{0xff}, 20)); err == nil {
		t.Fatal("corruption forwarded after insertion")
	}
}

func TestRelayAllowsConcurrentInspectionAndInjection(t *testing.T) {
	key := cipherKey{3, 19, 47, 61, 89, 103, 131, 157}
	relay := NewRelay()
	if _, err := relay.Forward(encryptPackets(key, testPacket(4, nil), testPacket(6, nil))); err != nil {
		t.Fatal(err)
	}

	var workers sync.WaitGroup
	var failures atomic.Int64

	for i := 0; i < 32; i++ {
		workers.Add(2)
		go func() {
			defer workers.Done()
			if !relay.Ready() {
				failures.Add(1)
			}
		}()
		go func() {
			defer workers.Done()
			if _, err := relay.Inject(Frame{Header: Header{Code: 12}, Body: []byte{0}}); err != nil {
				failures.Add(1)
			}
		}()
	}

	workers.Wait()

	if failures.Load() != 0 {
		t.Fatalf("failures = %d", failures.Load())
	}
}
