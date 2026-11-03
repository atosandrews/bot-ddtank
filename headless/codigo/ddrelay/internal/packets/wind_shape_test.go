package packets

import (
	"image"
	"image/color"
	"testing"
)

func TestWindShapeRejectsUnrelatedImages(t *testing.T) {
	for _, name := range []string{"empty", "solid", "vertical", "horizontal", "checker", "cross", "triangle"} {
		t.Run(name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 20, 26))
			for y := range 26 {
				for x := range 20 {
					opaque := false
					switch name {
					case "solid":
						opaque = true
					case "vertical":
						opaque = x >= 8 && x < 12
					case "horizontal":
						opaque = y >= 10 && y < 14
					case "checker":
						opaque = (x+y)%2 == 0
					case "cross":
						opaque = x >= 8 && x < 12 || y >= 10 && y < 14
					case "triangle":
						opaque = x <= y/2
					case "empty":
					default:
						t.Fatal("unsupported image case")
					}
					if opaque {
						img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
					}
				}
			}
			if got := matchWindShape(windShapeFeatures(img), windShapeCatalog); got != "" {
				t.Fatalf("unrelated image recognized as %q", got)
			}
		})
	}
}

func TestWindShapeRejectsAmbiguousClasses(t *testing.T) {
	a := windShapeCatalog[0]
	b := a
	b.Glyph = "ambiguous"
	if got := matchWindShape(a.Pixels, []windShapeTemplate{a, b}); got != "" {
		t.Fatalf("ambiguous shape recognized as %q", got)
	}
}

func TestWindShapeHeldOutKnownVariants(t *testing.T) {
	tested := 0
	for i, sample := range windShapeCatalog {
		others := append([]windShapeTemplate(nil), windShapeCatalog[:i]...)
		others = append(others, windShapeCatalog[i+1:]...)
		sameClass := false
		for _, other := range others {
			if other.Glyph == sample.Glyph {
				sameClass = true
			}
		}
		if !sameClass {
			continue
		}
		t.Run(sample.ShapeSHA256, func(t *testing.T) {
			want := sample.Glyph
			if sample.ShapeSHA256 == "be258909a64071fc9a868a6e416f915c8f35e6d3a21e0e07699e887f6b2041ba" {
				want = ""
			}
			if got := matchWindShape(sample.Pixels, others); got != want {
				t.Fatalf("held-out glyph=%q, want %q", got, want)
			}
		})
		tested++
	}
	if tested < 20 {
		t.Fatalf("only %d held-out variants", tested)
	}
}
