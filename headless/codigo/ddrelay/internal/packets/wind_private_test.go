package packets

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateWindPictureVariants(t *testing.T) {
	directory := os.Getenv("DDTANK_WIND_PICTURE_DIR")
	if directory == "" {
		t.Skip("private wind images not configured")
	}

	for _, test := range []struct {
		imageSHA256 string
		glyph       string
	}{
		{"09e7fe657d8d54b2f58ab4a251d31a72735ff01627a75cad8cf9fbc92a751c74", "7"},
		{"cbf1bf93406e03e3e0444b08f0b8e1399668ebe325f54378c451f3c1fd659363", "8"},
		{"e216b1627149cbfbe575cdfe3b9f746301f6cba6153e25ddd22402b38c5f52b8", "0"},
	} {
		t.Run(test.glyph, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(directory, test.imageSHA256+".png"))
			if err != nil {
				t.Fatal(err)
			}

			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != test.imageSHA256 {
				t.Fatal("private image hash mismatch")
			}

			fields := make(map[string]any)
			decodeWindPicture(data, fields)
			if fields["picture_glyph"] != test.glyph {
				t.Fatalf("glyph=%v, want %s", fields["picture_glyph"], test.glyph)
			}
		})
	}
}

func TestPrivateOctoberWindPictureVariants(t *testing.T) {
	directory := os.Getenv("DDTANK_OCT25_WIND")
	if directory == "" {
		t.Skip("private wind images not configured")
	}

	for _, test := range []struct {
		imageSHA256 string
		glyph       string
	}{
		{"53199f59fdcad98a9162d7cc3e38b9cbef4194bf1d535dd4be89120eadd8e9c6", "9"},
		{"789e4a72041fd2b803d176102a9c4b4da8102acb029db103fce6922531dabd74", "4"},
		{"84cf7b471ece6b572285a7904b8fdd9ad614a65723659f79ef691aff36d2222c", "3"},
		{"b2179d4a2513e70a1ed6c85c0bdba85bb2207e3c676bae35e799fcafcfe54212", "5"},
		{"e7aac29d0c3505848ff808c770a01ead7cafa06580d38ada7e455e665eaf5502", "0"},
		{"ead3d655e52260ac380414eceba2999c696c88096833a29b6f43d1475f60b2b3", "1"},
	} {
		t.Run(test.glyph, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(directory, test.imageSHA256+".png"))
			if err != nil {
				t.Fatal(err)
			}

			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != test.imageSHA256 {
				t.Fatal("private image hash mismatch")
			}

			fields := make(map[string]any)
			decodeWindPicture(data, fields)
			if fields["picture_glyph"] != test.glyph {
				t.Fatalf("glyph=%v, want %s", fields["picture_glyph"], test.glyph)
			}
		})
	}
}

func TestPrivateUnseenWindVariants(t *testing.T) {
	directory := os.Getenv("DDTANK_LATEST_WIND")
	if directory == "" {
		t.Skip("private unseen wind images not configured")
	}
	for _, test := range []struct{ hash, glyph string }{
		{"6b3b7b5564e0c5fd287ed794e8d4993cfdf84a3ecbea2f25f9ff0fe191d38ad9", "1"},
		{"9920232f1115fc3894152fd94f7bf3d725b7874f8e76c5d2bc6d2759581cff0f", "2"},
		{"9d85029f6560ba8a2e98863887ee9008a21fd7ea51e9b7d2a91930e986332451", "."},
		{"a1def4cb5bf0461c52068c374bb9d677bc638eae31067711644267b7339e4f51", "8"},
	} {
		t.Run(test.glyph, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(directory, test.hash+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != test.hash {
				t.Fatal("image hash mismatch")
			}
			img, err := readWindPicture(data)
			if err != nil {
				t.Fatal(err)
			}
			shape := windImageShape(img)
			if windGlyph(shape) != "" {
				t.Fatal("unseen variant added to exact lookup")
			}
			for _, template := range windShapeCatalog {
				if template.ShapeSHA256 == shape {
					t.Fatal("held-out image leaked into shape catalog")
				}
			}
			fields := make(map[string]any)
			decodeWindPicture(data, fields)
			if fields["picture_glyph"] != test.glyph || fields["picture_glyph_source"] != "normalized_shape" {
				t.Fatalf("glyph=%v source=%v", fields["picture_glyph"], fields["picture_glyph_source"])
			}
		})
	}
}

func TestPrivateWindShapeDescriptors(t *testing.T) {
	root := os.Getenv("DDTANK_WIND_RESEARCH")
	if root == "" {
		t.Skip("private wind corpus not configured")
	}
	files, err := filepath.Glob(filepath.Join(root, "wind-*", "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, path := range files {
		if filepath.Base(path) == "sheet.png" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := readWindPicture(data)
		if err != nil {
			t.Fatal(err)
		}
		shape := windImageShape(img)
		for _, template := range windShapeCatalog {
			if template.ShapeSHA256 != shape {
				continue
			}
			if windShapeFeatures(img) != template.Pixels {
				t.Fatalf("descriptor differs for %s", shape)
			}
			seen[shape] = true
		}
	}
	if len(seen) != len(windShapeCatalog) {
		t.Fatalf("verified %d of %d descriptors", len(seen), len(windShapeCatalog))
	}
}
