package packets

import "image"

const (
	windShapeWidth       = 16
	windShapeHeight      = 24
	windShapeSamples     = 4
	windShapeLevels      = 15
	maxWindShapeDistance = 0.14
	minWindShapeMargin   = 0.025
)

func windShapeFeatures(img image.Image) string {
	bounds := img.Bounds()
	left, top, right, bottom := bounds.Max.X, bounds.Max.Y, bounds.Min.X, bounds.Min.Y
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha > 32767 {
				left, top = min(left, x), min(top, y)
				right, bottom = max(right, x+1), max(bottom, y+1)
			}
		}
	}
	if right-left < 3 || bottom-top < 3 {
		return ""
	}

	pixels := make([]byte, windShapeWidth*windShapeHeight)
	const maximum = windShapeSamples * windShapeSamples * 65535
	for gy := range windShapeHeight {
		for gx := range windShapeWidth {
			total := uint32(0)
			for sy := range windShapeSamples {
				for sx := range windShapeSamples {
					x := left + (2*(gx*windShapeSamples+sx)+1)*(right-left)/(2*windShapeWidth*windShapeSamples)
					y := top + (2*(gy*windShapeSamples+sy)+1)*(bottom-top)/(2*windShapeHeight*windShapeSamples)
					_, _, _, alpha := img.At(x, y).RGBA()
					total += alpha
				}
			}
			pixels[gy*windShapeWidth+gx] = 'a' + byte((total*windShapeLevels+maximum/2)/maximum)
		}
	}
	return string(pixels)
}

func matchWindShape(pixels string, templates []windShapeTemplate) string {
	if len(pixels) != windShapeWidth*windShapeHeight {
		return ""
	}
	distances := make(map[string]float64)
	for _, template := range templates {
		if len(template.Pixels) != len(pixels) {
			continue
		}
		total := 0
		for i := range pixels {
			difference := int(pixels[i]) - int(template.Pixels[i])
			total += max(difference, -difference)
		}
		distance := float64(total) / float64(windShapeLevels*len(pixels))
		previous, exists := distances[template.Glyph]
		if !exists || distance < previous {
			distances[template.Glyph] = distance
		}
	}

	best, second := 1.0, 1.0
	glyph := ""
	for candidate, distance := range distances {
		if distance < best {
			second, best = best, distance
			glyph = candidate
		} else if distance < second {
			second = distance
		}
	}
	if best > maxWindShapeDistance || second-best < minWindShapeMargin {
		return ""
	}
	return glyph
}
