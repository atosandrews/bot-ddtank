package framing

import (
	"bytes"
	"sync"
)

type Relay struct {
	mu          sync.Mutex
	input       *EncryptedFramer
	output      cipherKey
	translating bool
	failed      bool
}

func NewRelay() *Relay { return &Relay{input: NewEncryptedFramer()} }

func (r *Relay) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.ready()
}

func (r *Relay) ready() bool {
	return !r.failed && r.input.locked && !r.input.startup && r.input.plain == nil && r.input.opaque == "" && len(r.input.buffer) == 0
}

func (r *Relay) Forward(data []byte) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.failed {
		return nil, &RelayError{Reason: "stream has failed"}
	}
	segments := r.input.Push(data)
	if !r.translating {
		return bytes.Clone(data), nil
	}
	if r.input.opaque != "" || r.input.plain != nil || !r.input.locked {
		r.failed = true
		return nil, &RelayError{Reason: "client cipher became invalid after command insertion"}
	}
	var wire []byte
	for _, segment := range segments {
		if segment.Kind != FrameSegment || !segment.Decrypted {
			r.failed = true
			return nil, &RelayError{Reason: "unverified data after command insertion"}
		}
		wire = append(wire, r.output.encrypt(segment.Raw)...)
	}
	return wire, nil
}

func (r *Relay) Inject(frame Frame) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.ready() {
		return nil, &RelayError{Reason: "requires a verified session key and a complete client packet boundary"}
	}
	plain, err := Marshal(frame)
	if err != nil {
		return nil, err
	}
	if !r.translating {
		r.output = r.input.key
		r.translating = true
	}
	return r.output.encrypt(plain), nil
}

func (r *Relay) Finish() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.translating && len(r.input.buffer) != 0 {
		return &RelayError{Reason: "client stream ended inside an encrypted packet"}
	}
	return nil
}
