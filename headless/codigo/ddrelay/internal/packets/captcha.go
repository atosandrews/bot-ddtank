package packets

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func captchaServer(r *reader, f map[string]any) error {
	f["kind"] = r.i8("kind")
	required := r.boolean("required")
	f["required"] = required
	if !required {
		return r.err
	}

	f["delay_type"] = r.i8("delay_type")
	f["prompt"] = r.utf("prompt")
	if r.err != nil {
		return r.err
	}

	data := r.take("image", len(r.data)-r.offset)
	if len(data) == 0 {
		r.fail("image", ErrInvalid)

		return r.err
	}

	digest := sha256.Sum256(data)
	f["image_base64"] = base64.StdEncoding.EncodeToString(data)
	f["image_sha256"] = hex.EncodeToString(digest[:])

	return r.err
}
