package fyneui

import (
	"bytes"
	"container/list"
	"image"
	"image/png"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

func TestEmbeddedFontCoversChineseReadingAndShipsLicense(t *testing.T) {
	font, err := sfnt.Parse(chineseFontBytes)
	if err != nil {
		t.Fatalf("embedded font does not parse: %v", err)
	}
	for _, character := range "北邮人论坛中文阅读选择复制图片回复，。ABC0123" {
		index, err := font.GlyphIndex(nil, character)
		if err != nil || index == 0 {
			t.Errorf("embedded font has no glyph for %q: index=%d error=%v", character, index, err)
		}
	}
	if !strings.Contains(fontLicense, "SIL OPEN FONT LICENSE Version 1.1") {
		t.Fatal("the distributable application must include its font license")
	}
}

func TestImageThumbnailPreservesAspectRatioWithoutUpscaling(t *testing.T) {
	for _, dimensions := range [][2]int{{2000, 1000}, {1000, 2000}, {20, 10}} {
		source := image.NewRGBA(image.Rect(0, 0, dimensions[0], dimensions[1]))
		thumbnail := imageThumbnail(source, 400, 300)
		bounds := thumbnail.Bounds()
		if bounds.Dx() > 400 || bounds.Dy() > 300 || bounds.Dx() > dimensions[0] || bounds.Dy() > dimensions[1] {
			t.Fatalf("thumbnail dimensions are not bounded: %v -> %v", dimensions, bounds)
		}
		if bounds.Dx()*dimensions[1] != bounds.Dy()*dimensions[0] {
			t.Fatalf("aspect ratio changed: %v -> %v", dimensions, bounds)
		}
	}
}

func TestImageCacheReusesDecodedImagesAndEvictsOldEntries(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 8, 4))); err != nil {
		t.Fatal(err)
	}
	reads := 0
	r := &contentRenderer{
		cache: make(map[string]*list.Element), lru: list.New(),
		fetch: func(string) ([]byte, error) {
			reads++
			return encoded.Bytes(), nil
		},
	}
	for i := 0; i < 2; i++ {
		if _, err := r.readImage("first"); err != nil {
			t.Fatal(err)
		}
	}
	if reads != 1 {
		t.Fatalf("reading the same image should reuse the cache, got %d source reads", reads)
	}
	for i := 0; i < imageCacheCount; i++ {
		if _, err := r.readImage(string(rune('a' + i))); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.cache) != imageCacheCount || r.cache["first"] != nil {
		t.Fatalf("decoded image cache did not evict its oldest item: %d entries", len(r.cache))
	}
}
