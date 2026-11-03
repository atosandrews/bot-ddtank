package packets

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"ddbot/relay/internal/capture"
)

func TestWindPictureShapeIgnoresColorAndPNGEncoding(t *testing.T) {
	mask := []byte{0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 128, 128, 255, 255, 0, 0}
	want := fmt.Sprintf("%x", sha256.Sum256(mask))
	for _, tt := range []struct {
		name        string
		color       color.NRGBA
		compression png.CompressionLevel
	}{
		{"red", color.NRGBA{R: 255}, png.DefaultCompression},
		{"green", color.NRGBA{G: 255}, png.NoCompression},
		{"blue", color.NRGBA{B: 255}, png.BestCompression},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
			for i, alpha := range []uint8{0, 128, 255, 0} {
				c := tt.color
				c.A = alpha
				img.SetNRGBA(i%2, i/2, c)
			}
			data := encodeWindPNG(t, img, tt.compression)
			got, err := windPictureShape(data)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("shape = %q, want %q", got, want)
			}
			body := append([]byte{241, 7}, data...)
			expected := map[string]any{
				"subcommand": uint8(241), "event": "WIND_PIC", "picture_id": int8(7),
				"picture_bytes": len(data), "picture_sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "picture_shape_sha256": want,
			}
			assertFight(t, decodeFight(capture.ServerToClient, body), body, expected)
		})
	}
}

func TestWindPictureRejectsMalformedAndOversizedImages(t *testing.T) {
	valid := encodeWindPNG(t, image.NewNRGBA(image.Rect(0, 0, 2, 2)), png.DefaultCompression)
	corrupt := bytes.Clone(valid)
	corrupt[len(corrupt)-1] ^= 1
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"empty", nil}, {"not PNG", []byte("synthetic picture")},
		{"truncated", valid[:len(valid)-1]}, {"CRC failure", corrupt},
		{"wide", encodeWindPNG(t, image.NewNRGBA(image.Rect(0, 0, 65, 2)), png.DefaultCompression)},
		{"tall", encodeWindPNG(t, image.NewNRGBA(image.Rect(0, 0, 2, 65)), png.DefaultCompression)},
		{"oversized input", make([]byte, 65537)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			shape, err := windPictureShape(tt.data)
			var pictureError *WindPictureError
			if !errors.As(err, &pictureError) || shape != "" {
				t.Fatalf("shape = %q, error = %v", shape, err)
			}
		})
	}
}

func encodeWindPNG(t *testing.T, img image.Image, compression png.CompressionLevel) []byte {
	t.Helper()
	var data bytes.Buffer
	encoder := png.Encoder{CompressionLevel: compression}
	if err := encoder.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
