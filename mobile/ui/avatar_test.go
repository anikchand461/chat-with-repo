package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestAvatarJPEGCropsToSquare(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 900, 300))
	for x := 300; x < 600; x++ { // only the middle third is red
		for y := 0; y < 300; y++ {
			src.Set(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}

	data, _, err := avatarJPEG(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 512*1024 {
		t.Fatalf("upload is %d bytes, over the server's limit", len(data))
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got != image.Pt(avatarSize, avatarSize) {
		t.Fatalf("size = %v", got)
	}
	// The crop is the centered square, so the edges are red too.
	if r, g, _, _ := img.At(2, avatarSize/2).RGBA(); r < 0xd000 || g > 0x3000 {
		t.Fatalf("left edge is not red: r=%x g=%x", r, g)
	}
}

func TestAvatarJPEGRejectsNonImages(t *testing.T) {
	if _, _, err := avatarJPEG(strings.NewReader("not a picture")); err == nil {
		t.Fatal("expected an error")
	}
}
