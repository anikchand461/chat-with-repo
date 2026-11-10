package ui

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg" // also registers the JPEG decoder for photos and Google pictures
	"io"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	xdraw "golang.org/x/image/draw"
)

// avatarSize is the side of the square JPEG we upload, same as the web app.
const avatarSize = 256

// avatarMaxFile caps how much of a picked file is read (phone photos are a
// few MB; anything bigger isn't a photo we want to decode).
const avatarMaxFile = 25 << 20

var (
	avatarBg     = profileOkBg
	avatarBorder = profileOkBorder
	avatarFg     = color.NRGBA{R: 0x4a, G: 0xde, B: 0x80, A: 0xc0}
)

// avatarJPEG center-crops the picked picture to a square, shrinks it to
// avatarSize and re-encodes it as JPEG, so uploads stay small.
func avatarJPEG(r io.Reader) ([]byte, image.Image, error) {
	raw, err := io.ReadAll(io.LimitReader(r, avatarMaxFile+1))
	if err != nil {
		return nil, nil, err
	}
	if len(raw) > avatarMaxFile {
		return nil, nil, errors.New("That picture is too large.")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, errors.New("Pick a JPEG or PNG picture.")
	}

	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side).Add(b.Min).Add(image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2))

	dst := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	// White under transparent PNGs, like the web app.
	xdraw.Draw(dst, dst.Bounds(), image.White, image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, xdraw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 88}); err != nil {
		return nil, nil, err
	}
	return out.Bytes(), dst, nil
}

// decodeAvatar turns downloaded picture bytes into an image op, or nil.
func decodeAvatar(data []byte) *paint.ImageOp {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	op := paint.NewImageOp(img)
	return &op
}

// avatarCircle draws the profile picture in a ringed circle of diameter d,
// or the default silhouette when src is nil.
func avatarCircle(gtx layout.Context, src *paint.ImageOp, d int) layout.Dimensions {
	sz := image.Pt(d, d)
	paint.FillShape(gtx.Ops, avatarBorder, clip.Ellipse(image.Rectangle{Max: sz}).Op(gtx.Ops))

	ring := max(d/40, 2)
	inner := image.Rect(ring, ring, d-ring, d-ring)
	defer clip.Ellipse(inner).Push(gtx.Ops).Pop()

	if src != nil {
		in := gtx
		in.Constraints = layout.Exact(inner.Size())
		defer op.Offset(inner.Min).Push(gtx.Ops).Pop()
		widget.Image{Src: *src, Fit: widget.Cover, Position: layout.Center}.Layout(in)
		return layout.Dimensions{Size: sz}
	}

	paint.Fill(gtx.Ops, avatarBg)
	// Head and shoulders, same proportions as the web profile's SVG.
	at := func(v int) int { return v * d / 84 }
	paint.FillShape(gtx.Ops, avatarFg, clip.Ellipse(image.Rect(at(27), at(18), at(57), at(48))).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, avatarFg, clip.Ellipse(image.Rect(at(14), at(53), at(70), at(99))).Op(gtx.Ops))
	return layout.Dimensions{Size: sz}
}
