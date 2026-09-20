package ui

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png" // registers the PNG decoder for SetLogo

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Palette for the Login / Register screens, sampled from the website.
var (
	authBgTop       = color.NRGBA{R: 0x0b, G: 0x3d, B: 0x2e, A: 0xff}
	authBgBottom    = color.NRGBA{R: 0x05, G: 0x10, B: 0x0d, A: 0xff}
	authGlow        = color.NRGBA{R: 0x2e, G: 0x4a, B: 0x3f, A: 0xff}
	authCard        = color.NRGBA{R: 0x0f, G: 0x1c, B: 0x18, A: 0xff}
	authCardBorder  = color.NRGBA{R: 0x1f, G: 0x33, B: 0x2c, A: 0xff}
	authField       = color.NRGBA{R: 0x08, G: 0x14, B: 0x11, A: 0xff}
	authFieldBorder = color.NRGBA{R: 0x22, G: 0x36, B: 0x2f, A: 0xff}
	authTitle       = color.NRGBA{R: 0xe8, G: 0xf7, B: 0xf0, A: 0xff}
	authBody        = color.NRGBA{R: 0xa9, G: 0xbd, B: 0xb5, A: 0xff}
	authLabel       = color.NRGBA{R: 0xb4, G: 0xd4, B: 0xc8, A: 0xff}
	authHint        = color.NRGBA{R: 0x5f, G: 0x74, B: 0x6c, A: 0xff}
	authAccent      = color.NRGBA{R: 0x10, G: 0xa3, B: 0x72, A: 0xff}
	authLink        = color.NRGBA{R: 0x1f, G: 0xb2, B: 0x84, A: 0xff}
	authBtnLeft     = color.NRGBA{R: 0x5a, G: 0xdc, B: 0xa8, A: 0xff}
	authBtnRight    = color.NRGBA{R: 0x10, G: 0xa3, B: 0x72, A: 0xff}
	authBtnText     = color.NRGBA{R: 0x04, G: 0x1a, B: 0x12, A: 0xff}
	authIconBg      = color.NRGBA{R: 0x08, G: 0x14, B: 0x11, A: 0xff}
	authIconFg      = color.NRGBA{R: 0xd5, G: 0xe6, B: 0xdf, A: 0xff}
)

var logoOp, githubOp *paint.ImageOp

// SetLogo decodes the ChatWithRepo logo (PNG bytes, supplied by main so
// the asset stays in the mobile/ folder). A bad image just hides the logo.
func SetLogo(pngData []byte) {
	img, _, err := image.Decode(bytes.NewReader(pngData))
	if err != nil {
		return
	}
	op := paint.NewImageOp(img)
	logoOp = &op
}

// SetGitHubIcon decodes assets/github.png. If the data is missing or
// invalid the badge is drawn empty (still tappable).
func SetGitHubIcon(pngData []byte) {
	img, _, err := image.Decode(bytes.NewReader(pngData))
	if err != nil {
		return
	}
	op := paint.NewImageOp(img)
	githubOp = &op
}

// authS scales spacing, type and control heights on the auth screens so
// the whole form fits the viewport. It is set once per frame by
// authScreen (the UI runs on a single goroutine).
var authS float32 = 1

// authScale derives the scale from the available viewport: 1.0 on a
// normal phone (>= ~720dp tall), shrinking gently on short/narrow ones.
func authScale(gtx layout.Context) float32 {
	h := float32(gtx.Constraints.Max.Y) / gtx.Metric.PxPerDp
	w := float32(gtx.Constraints.Max.X) / gtx.Metric.PxPerDp
	s := h / 720
	if ws := w / 360; ws < s {
		s = ws
	}
	if s > 1 {
		s = 1
	}
	if s < 0.68 {
		s = 0.68
	}
	return s
}

// authDp scales a dp value by the current auth scale.
func authDp(v float32) unit.Dp { return unit.Dp(v * authS) }

// authSp scales a text size by the current auth scale.
func authSp(v float32) unit.Sp { return unit.Sp(v * (0.85 + 0.15*authS)) }

// authGap is a scaled fixed-height spacer.
func authGap(dp int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Point{Y: gtx.Dp(authDp(float32(dp)))}}
	}
}

// authScreen lays out the shared Login/Register chrome: gradient
// background, logo + wordmark, and a rounded card containing body.
// Nothing scrolls: everything is sized from the viewport so it fits.
func authScreen(gtx layout.Context, th *material.Theme, title, subtitle string, body layout.Widget) layout.Dimensions {
	authS = authScale(gtx)
	authBackground(gtx)
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Cap card width on tablets / desktop; keep side gutters.
		maxW := gtx.Dp(unit.Dp(440))
		if w := gtx.Constraints.Max.X - 2*gtx.Dp(authDp(20)); w < maxW {
			maxW = w
		}
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = maxW, maxW
		gtx.Constraints.Min.Y = 0
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(authLogo),
			layout.Rigid(authGap(10)),
			layout.Rigid(authWordmark(th)),
			layout.Rigid(authGap(18)),
			layout.Rigid(authCardWidget(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, authSp(28), title)
						l.Color = authTitle
						l.Font.Weight = font.Bold
						return l.Layout(gtx)
					}),
					layout.Rigid(authGap(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, authSp(15), subtitle)
						l.Color = authBody
						return l.Layout(gtx)
					}),
					layout.Rigid(authGap(16)),
					layout.Rigid(body),
				)
			})),
		)
	})
}

// authBackground paints the dark green vertical gradient plus the soft
// lighter glow in the bottom-right corner.
func authBackground(gtx layout.Context) {
	sz := gtx.Constraints.Max
	defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
	paint.LinearGradientOp{
		Stop1:  f32.Pt(0, 0),
		Color1: authBgTop,
		Stop2:  f32.Pt(0, float32(sz.Y)*0.55),
		Color2: authBgBottom,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	// Stacked translucent discs approximate a radial glow.
	cx, cy := sz.X*85/100, sz.Y*95/100
	maxR := float64(sz.X) * 0.7
	const steps = 14
	for i := 0; i < steps; i++ {
		r := int(maxR * (1 - float64(i)/steps))
		c := authGlow
		c.A = 9
		st := clip.Ellipse{Min: image.Pt(cx-r, cy-r), Max: image.Pt(cx+r, cy+r)}.Push(gtx.Ops)
		paint.ColorOp{Color: c}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		st.Pop()
	}
}

func authLogo(gtx layout.Context) layout.Dimensions {
	if logoOp == nil {
		return layout.Dimensions{}
	}
	return widget.Image{Src: *logoOp, Fit: widget.Contain, Position: layout.Center}.Layout(
		constrainHeight(gtx, authDp(68)))
}

func constrainHeight(gtx layout.Context, h unit.Dp) layout.Context {
	px := gtx.Dp(h)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = px, px
	return gtx
}

// authWordmark draws "Chat" in white and "WithRepo" in green, centered.
func authWordmark(th *material.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		part := func(s string, c color.NRGBA) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				l := material.Label(th, authSp(28), s)
				l.Color = c
				l.Font.Weight = font.Bold
				l.MaxLines = 1
				return l.Layout(gtx)
			}
		}
		gtx.Constraints.Min.X = 0
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(part("Chat", authTitle)),
			layout.Rigid(part("WithRepo", authAccent)),
		)
	}
}

// authCardWidget wraps w in the large rounded, 1px-bordered card.
func authCardWidget(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(26), authCard, authCardBorder)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(authDp(20)).Layout(gtx, w)
			}),
		)
	}
}

// borderedRRect fills a rounded rect with a 1dp border.
func borderedRRect(gtx layout.Context, sz image.Point, radius unit.Dp, fillCol, borderCol color.NRGBA) {
	r := gtx.Dp(radius)
	roundedFill(gtx, sz, radius, borderCol)
	b := gtx.Dp(unit.Dp(1))
	if b < 1 {
		b = 1
	}
	inner := image.Rectangle{Min: image.Pt(b, b), Max: sz.Sub(image.Pt(b, b))}
	paint.FillShape(gtx.Ops, fillCol, clip.RRect{Rect: inner, SE: r - b, SW: r - b, NE: r - b, NW: r - b}.Op(gtx.Ops))
}

// AuthField renders a bold label above a large rounded input.
func AuthField(th *material.Theme, editor *widget.Editor, label, hint string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Label(th, authSp(14), label)
				l.Color = authLabel
				l.Font.Weight = font.Bold
				return l.Layout(gtx)
			}),
			layout.Rigid(authGap(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{Alignment: layout.W}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(18), authField, authFieldBorder)
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						h := gtx.Dp(authDp(52))
						gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, h)
						gtx.Constraints.Max.Y = h
						return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx,
							func(gtx layout.Context) layout.Dimensions {
								// Measure one text line, then size the editor to exactly
								// that line and centre it, so text, placeholder and
								// cursor share the same vertical centre.
								size := authSp(16)
								m := op.Record(gtx.Ops)
								probe := material.Label(th, size, "Ag")
								pg := gtx
								pg.Constraints.Min = image.Point{}
								lh := probe.Layout(pg).Size.Y
								m.Stop()

								gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = lh, lh
								defer op.Offset(image.Pt(0, (h-lh)/2)).Push(gtx.Ops).Pop()
								ed := material.Editor(th, editor, hint)
								ed.TextSize = size
								ed.Color = authTitle
								ed.HintColor = authHint
								ed.SelectionColor = color.NRGBA{R: 0x10, G: 0xa3, B: 0x72, A: 0x66}
								dims := ed.Layout(gtx)
								dims.Size.Y = h
								return dims
							})
					}),
				)
			}),
		)
	}
}

// AuthButton is the full-width green gradient call-to-action.
func AuthButton(th *material.Theme, btn *widget.Clickable, label string, loading bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if loading {
			gtx = gtx.Disabled()
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{Alignment: layout.Center}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					sz := gtx.Constraints.Min
					r := gtx.Dp(unit.Dp(16))
					c1, c2 := authBtnLeft, authBtnRight
					if loading {
						c1.A, c2.A = 0xa0, 0xa0
					}
					st := clip.RRect{Rect: image.Rectangle{Max: sz}, SE: r, SW: r, NE: r, NW: r}.Push(gtx.Ops)
					paint.LinearGradientOp{
						Stop1: f32.Pt(0, 0), Color1: c1,
						Stop2: f32.Pt(float32(sz.X), float32(sz.Y)), Color2: c2,
					}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					st.Pop()
					return layout.Dimensions{Size: sz}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Top: authDp(14), Bottom: authDp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if loading {
							loader := material.Loader(th)
							loader.Color = authBtnText
							return layout.Center.Layout(gtx, loader.Layout)
						}
						l := material.Label(th, authSp(17), label)
						l.Color = authBtnText
						l.Font.Weight = font.Bold
						l.Alignment = text.Middle
						return l.Layout(gtx)
					})
				}),
			)
		})
	}
}

// AuthLink is a centered, underlined teal text link.
func AuthLink(th *material.Theme, btn *widget.Clickable, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(authDp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(th, authSp(16), label)
					l.Color = authLink
					l.MaxLines = 1
					dims := l.Layout(gtx)
					line := image.Rect(0, dims.Size.Y-gtx.Dp(unit.Dp(1)), dims.Size.X, dims.Size.Y)
					paint.FillShape(gtx.Ops, authLink, clip.Rect(line).Op())
					return dims
				})
			})
		})
	}
}

// repoURL is opened when the GitHub badge is tapped.
const repoURL = "https://github.com/shreyaghorui222004/chat-with-repo"

// AuthGitHubButton is the round, clickable GitHub badge at the bottom of
// the card. It draws assets/github.png and opens the repository in the
// system browser when tapped.
func AuthGitHubButton(btn *widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		for btn.Clicked(gtx) {
			go func() { _ = openURL(repoURL) }()
		}
		d := gtx.Dp(authDp(44))
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Pt(d, d)
			gtx.Constraints.Max = gtx.Constraints.Min
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := image.Pt(d, d)
				borderedRRect(gtx, sz, unit.Dp(22*authS), authIconBg, authFieldBorder)
				if githubOp != nil {
					pad := d * 24 / 100
					in := gtx
					in.Constraints.Min = image.Pt(d-2*pad, d-2*pad)
					in.Constraints.Max = in.Constraints.Min
					defer op.Offset(image.Pt(pad, pad)).Push(gtx.Ops).Pop()
					widget.Image{Src: *githubOp, Fit: widget.Contain, Position: layout.Center}.Layout(in)
				}
				return layout.Dimensions{Size: sz}
			})
		})
	}
}

// AuthSecondaryButton is the outlined, full-width low-emphasis button.
func AuthSecondaryButton(th *material.Theme, btn *widget.Clickable, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{Alignment: layout.Center}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(16), authCard, authCardBorder)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Top: authDp(13), Bottom: authDp(13)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, authSp(16), label)
						l.Color = authTitle
						l.Font.Weight = font.Bold
						l.Alignment = text.Middle
						return l.Layout(gtx)
					})
				}),
			)
		})
	}
}
