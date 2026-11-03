package captcha

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
)

const imageHeight = 64

func imageTensor(encoded string) ([]float32, int, error) {
	if len(encoded) == 0 || len(encoded) > 88<<10 {
		return nil, 0, &Error{Operation: "image", Reason: "image exceeds the game packet size"}
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, 0, &Error{Operation: "image", Reason: "invalid base64 image", Cause: err}
	}

	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, 0, &Error{Operation: "image", Reason: "expected a PNG challenge", Cause: err}
	}

	if config.Width < 4 || config.Height < 4 || config.Width > 512 || config.Height > 128 {
		return nil, 0, &Error{Operation: "image", Reason: "unsupported challenge dimensions"}
	}

	width := config.Width * imageHeight / config.Height
	if width > 1024 {
		return nil, 0, &Error{Operation: "image", Reason: "challenge aspect ratio exceeds the limit"}
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, &Error{Operation: "image", Reason: "cannot decode PNG challenge", Cause: err}
	}

	values := make([]float32, width*imageHeight)
	for y := range imageHeight {
		sy := max(0, (float64(y)+0.5)*float64(config.Height)/imageHeight-0.5)
		y0 := int(sy)
		y1 := min(y0+1, config.Height-1)
		fy := sy - float64(y0)
		for x := range width {
			sx := max(0, (float64(x)+0.5)*float64(config.Width)/float64(width)-0.5)
			x0 := int(sx)
			x1 := min(x0+1, config.Width-1)
			fx := sx - float64(x0)
			top := grayPixel(img, x0, y0)*(1-fx) + grayPixel(img, x1, y0)*fx
			bottom := grayPixel(img, x0, y1)*(1-fx) + grayPixel(img, x1, y1)*fx
			values[y*width+x] = float32(math.Round(top*(1-fy)+bottom*fy)/127.5 - 1)
		}
	}

	return values, width, nil
}

func grayPixel(img image.Image, x, y int) float64 {
	c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	a := float64(c.A) / 255
	gray := (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) * a

	return gray + 255*(1-a)
}
