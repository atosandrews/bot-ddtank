package packets

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
)

const (
	maxWindPictureBytes     = 65536
	maxWindPictureDimension = 64
)

func decodeWindPicture(data []byte, fields map[string]any) {
	img, err := readWindPicture(data)
	if err != nil {
		fields["picture_glyph_error"] = err.Error()
		return
	}
	shape := windImageShape(img)

	fields["picture_shape_sha256"] = shape
	if glyph := windGlyph(shape); glyph != "" {
		fields["picture_glyph"] = glyph
		return
	}
	if glyph := matchWindShape(windShapeFeatures(img), windShapeCatalog); glyph != "" {
		fields["picture_glyph"] = glyph
		fields["picture_glyph_source"] = "normalized_shape"
	}
}

func readWindPicture(data []byte) (image.Image, error) {
	if len(data) > maxWindPictureBytes {
		return nil, &WindPictureError{Reason: "image exceeds 64 KiB"}
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, &WindPictureError{Reason: "PNG header", Cause: err}
	}
	if config.Width < 1 || config.Height < 1 || config.Width > maxWindPictureDimension || config.Height > maxWindPictureDimension {
		return nil, &WindPictureError{Reason: "image dimensions exceed 64 by 64"}
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, &WindPictureError{Reason: "PNG pixels", Cause: err}
	}

	return img, nil
}

func windPictureShape(data []byte) (string, error) {
	img, err := readWindPicture(data)
	if err != nil {
		return "", err
	}
	return windImageShape(img), nil
}

func windImageShape(img image.Image) string {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	mask := make([]byte, 0, 8+width*height*2)
	mask = binary.BigEndian.AppendUint32(mask, uint32(width))
	mask = binary.BigEndian.AppendUint32(mask, uint32(height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			mask = binary.BigEndian.AppendUint16(mask, uint16(alpha))
		}
	}

	return fmt.Sprintf("%x", sha256.Sum256(mask))
}
