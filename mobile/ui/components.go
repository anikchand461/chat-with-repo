package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"chatwithrepo/mobile/api"
)

// Palette holds the small, fixed color set used throughout the app.
// Keeping it in one place is what makes the app feel native and
// consistent instead of a re-skinned web page.
var (
	colorBackground  = color.NRGBA{R: 0x0f, G: 0x11, B: 0x16, A: 0xff}
	colorSurface     = color.NRGBA{R: 0x1a, G: 0x1d, B: 0x24, A: 0xff}
	colorPrimary     = color.NRGBA{R: 0x4f, G: 0x8a, B: 0xf7, A: 0xff}
	colorPrimaryDark = color.NRGBA{R: 0x35, G: 0x66, B: 0xc4, A: 0xff}
	colorText        = color.NRGBA{R: 0xf2, G: 0xf3, B: 0xf5, A: 0xff}
	colorSubtleText  = color.NRGBA{R: 0x9a, G: 0xa1, B: 0xae, A: 0xff}
	colorError       = color.NRGBA{R: 0xe5, G: 0x6b, B: 0x6b, A: 0xff}
	colorBubbleUser  = color.NRGBA{R: 0x2a, G: 0x53, B: 0x9c, A: 0xff}
	colorBubbleBot   = color.NRGBA{R: 0x22, G: 0x25, B: 0x2e, A: 0xff}
	colorCodeBg      = color.NRGBA{R: 0x10, G: 0x12, B: 0x16, A: 0xff}
)

// NewTheme builds the material theme used across every screen.
func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = newShaper()
	th.Palette = material.Palette{
		Bg:         colorBackground,
		Fg:         colorText,
		ContrastBg: colorPrimary,
		ContrastFg: colorText,
	}
	return th
}

// spacer returns a fixed-height blank widget, used to add breathing
// room between stacked elements without a full layout.Inset.
func spacer(dp int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Point{Y: gtx.Dp(unit.Dp(dp))}}
	}
}

// colorScrim is the translucent backdrop shown behind the "New
// Repository" modal.
func colorScrim() color.NRGBA {
	return color.NRGBA{R: 0, G: 0, B: 0, A: 0x99}
}

// fill paints a solid rectangle across the current constraints.
func fill(gtx layout.Context, col color.NRGBA) layout.Dimensions {
	sz := gtx.Constraints.Min
	paint.FillShape(gtx.Ops, col, clip.Rect{Max: sz}.Op())
	return layout.Dimensions{Size: sz}
}

// roundedFill paints a rounded rectangle with the given size and radius.
func roundedFill(gtx layout.Context, sz image.Point, radius unit.Dp, col color.NRGBA) {
	r := gtx.Dp(radius)
	rr := clip.RRect{Rect: image.Rectangle{Max: sz}, SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, col, rr.Op(gtx.Ops))
}

// PrimaryButton renders a full-width, filled button. When loading is
// true, the label is replaced with an inline loader and the button
// stops reacting to clicks.
func PrimaryButton(th *material.Theme, btn *widget.Clickable, label string, loading bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if loading {
			gtx = gtx.Disabled()
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{Alignment: layout.Center}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					col := colorPrimary
					if loading {
						col = colorPrimaryDark
					}
					roundedFill(gtx, gtx.Constraints.Min, unit.Dp(10), col)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if loading {
							loader := material.Loader(th)
							loader.Color = colorText
							return layout.Center.Layout(gtx, loader.Layout)
						}
						lbl := material.Body1(th, label)
						lbl.Color = colorText
						lbl.Font.Weight = font.Bold
						lbl.Alignment = text.Middle
						return layout.Center.Layout(gtx, lbl.Layout)
					})
				}),
			)
		})
	}
}

// TextButton renders a plain, low-emphasis clickable text label (e.g.
// "Create Account" / "Back").
func TextButton(th *material.Theme, btn *widget.Clickable, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Color = colorPrimary
				lbl.Alignment = text.Middle
				return layout.Center.Layout(gtx, lbl.Layout)
			})
		})
	}
}

// TextField renders a labelled, boxed input on top of a surface-colored
// rounded rect, matching a native Android text field more than a plain
// underlined web input.
func TextField(th *material.Theme, editor *widget.Editor, hint string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				roundedFill(gtx, gtx.Constraints.Min, unit.Dp(10), colorSurface)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, editor, hint)
					ed.Color = colorText
					ed.HintColor = colorSubtleText
					return ed.Layout(gtx)
				})
			}),
		)
	}
}

// ErrorText renders a small red error line, or nothing if msg is empty.
func ErrorText(th *material.Theme, msg string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if msg == "" {
			return layout.Dimensions{}
		}
		lbl := material.Body2(th, msg)
		lbl.Color = colorError
		return lbl.Layout(gtx)
	}
}

// UpgradeModal renders the centered "Upgrade to Pro" dialog shown
// whenever the free plan's repo-chat or daily-question limit is hit,
// mirroring the web frontend's #upgrade-modal. Callers own the two
// Clickables and all state; this only draws.
func UpgradeModal(th *material.Theme, message string, upgradeBtn, cancelBtn *widget.Clickable, checkingOut bool, errMsg string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Constraints.Max.X * 88 / 100
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(26), authCard, authCardBorder)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								title := material.H6(th, "Upgrade to Pro")
								title.Color = colorText
								return title.Layout(gtx)
							}),
							layout.Rigid(spacer(10)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, message)
								lbl.Color = colorSubtleText
								return lbl.Layout(gtx)
							}),
							layout.Rigid(spacer(8)),
							layout.Rigid(ErrorText(th, errMsg)),
							layout.Rigid(spacer(16)),
							layout.Rigid(AuthButton(th, upgradeBtn, "Upgrade to Pro", checkingOut)),
							layout.Rigid(spacer(8)),
							layout.Rigid(AuthLink(th, cancelBtn, "Not now")),
						)
					})
				}),
			)
		})
	}
}

// IndexingModal mirrors the web frontend's dashboard create-chat
// progress panel (js/app.js pollIndexStatus / #index-progress): a short
// predefined stage message, with a done/total progress bar once one
// exists, while a chat's repository is indexed in the background - or
// the failure, with a link to the Profile page when it's the GitHub
// rate-limit message, if indexing didn't finish.
func IndexingModal(th *material.Theme, message string, done, total int, errMsg string, closeBtn, profileBtn *widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Constraints.Max.X * 88 / 100
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(26), authCard, authCardBorder)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						children := []layout.FlexChild{
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								title := material.H6(th, "Setting up your chat")
								title.Color = colorText
								return title.Layout(gtx)
							}),
							layout.Rigid(spacer(12)),
						}

						if errMsg == "" {
							children = append(children,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return dotsIndicator(gtx, authAccent, gtx.Dp(unit.Dp(4)))
										}),
										layout.Rigid(spacerX(10)),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											lbl := material.Body2(th, message)
											lbl.Color = colorSubtleText
											return lbl.Layout(gtx)
										}),
									)
								}),
							)
							if total > 0 {
								children = append(children,
									layout.Rigid(spacer(14)),
									layout.Rigid(progressBar(done, total)),
								)
							}
						} else {
							children = append(children,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body2(th, errMsg)
									lbl.Color = colorError
									return lbl.Layout(gtx)
								}),
							)
						}

						children = append(children, layout.Rigid(spacer(18)))

						if errMsg != "" {
							if isRateLimitMessage(errMsg) {
								children = append(children,
									layout.Rigid(AuthButton(th, profileBtn, "Go to profile", false)),
									layout.Rigid(spacer(8)),
								)
							}
							children = append(children, layout.Rigid(AuthSecondaryButton(th, closeBtn, "Close")))
						}

						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					})
				}),
			)
		})
	}
}

// progressBar draws a thin, rounded, percentage-filled bar - the
// mobile equivalent of the web frontend's #index-progress-bar.
func progressBar(done, total int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		h := gtx.Dp(unit.Dp(6))
		r := gtx.Dp(unit.Dp(3))

		frac := float64(done) / float64(total)
		if frac > 1 {
			frac = 1
		}
		if frac < 0 {
			frac = 0
		}

		paint.FillShape(gtx.Ops, authFieldBorder, clip.RRect{Rect: image.Rect(0, 0, w, h), NE: r, NW: r, SE: r, SW: r}.Op(gtx.Ops))
		if fw := int(float64(w) * frac); fw > 0 {
			paint.FillShape(gtx.Ops, authAccent, clip.RRect{Rect: image.Rect(0, 0, fw, h), NE: r, NW: r, SE: r, SW: r}.Op(gtx.Ops))
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
}

// isRateLimitMessage matches the web frontend's /rate limit/i.test(message)
// check (js/app.js) - the backend sends this exact wording when GitHub's
// API limit is hit (github/client.py).
func isRateLimitMessage(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "rate limit")
}

// upgradeMessageFor mirrors the web frontend's showUpgradeModal()
// switch (js/app.js) so both clients show identical copy for the
// same "reason" the backend sends.
func upgradeMessageFor(reason string) string {
	switch reason {
	case "repo_limit":
		return "You've reached the free limit of 2 repository chats. Upgrade to Pro for unlimited chats."
	case "daily_questions", "daily_limit":
		return "You've reached today's free question limit. Upgrade to Pro for unlimited questions."
	default:
		return "This feature requires ChatWithRepo Pro."
	}
}

// Bubble renders one chat message as a rounded bubble aligned to the
// right (user, green gradient) or left (assistant, dark bordered card).
// Assistant text goes through the Markdown renderer; user text is shown
// as typed.
func Bubble(th *material.Theme, role, content string, meta *api.ResponseMeta) layout.Widget {
	isUser := role == "user"
	align := layout.W
	fg := authTitle
	if isUser {
		align = layout.E
		fg = authBtnText
	}

	return func(gtx layout.Context) layout.Dimensions {
		maxWidth := gtx.Constraints.Max.X * 88 / 100
		return align.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = maxWidth
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					sz := gtx.Constraints.Min
					if !isUser {
						borderedRRect(gtx, sz, unit.Dp(20), authCard, authCardBorder)
						return layout.Dimensions{Size: sz}
					}
					r := gtx.Dp(unit.Dp(20))
					st := clip.RRect{Rect: image.Rectangle{Max: sz}, SE: r, SW: r, NE: r, NW: r}.Push(gtx.Ops)
					paint.LinearGradientOp{
						Stop1: f32.Pt(0, 0), Color1: authBtnLeft,
						Stop2: f32.Pt(float32(sz.X), float32(sz.Y)), Color2: authBtnRight,
					}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					st.Pop()
					return layout.Dimensions{Size: sz}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						md := func(gtx layout.Context) layout.Dimensions {
							return renderMarkdown(gtx, th, content, fg, !isUser)
						}
						if isUser || meta == nil {
							return md(gtx)
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(md),
							layout.Rigid(responseFooter(th, meta)),
						)
					})
				}),
			)
		})
	}
}

// emojiFace is the bundled color-emoji font (set by main via SetEmojiFont).
// Gio's default fonts and the OS font scan don't cover emoji, so without it
// they render as empty boxes.
var emojiFace text.FontFace

// SetEmojiFont registers an emoji TTF (Noto Color Emoji) as a fallback font.
func SetEmojiFont(ttf []byte) {
	face, err := opentype.Parse(ttf)
	if err != nil {
		return
	}
	emojiFace = text.FontFace{Font: font.Font{Typeface: "Noto Color Emoji"}, Face: face}
}

// newShaper builds the text shaper: the Go fonts (regular/bold/mono) plus
// the emoji font. Glyphs missing from the primary font fall back to it.
func newShaper() *text.Shaper {
	coll := gofont.Collection()
	if emojiFace.Face != nil {
		coll = append(coll, emojiFace)
	}
	return text.NewShaper(text.WithCollection(coll))
}

// formatSeconds renders a duration like the web UI: 4.2s, 12s.
func formatSeconds(d time.Duration) string {
	sec := d.Seconds()
	if sec < 10 {
		return fmt.Sprintf("%.1fs", sec)
	}
	return fmt.Sprintf("%.0fs", sec)
}

// responseFooter is the line under an assistant answer: a divider, a clock
// icon and "Responded in 4.2s · first word 1.8s".
func responseFooter(th *material.Theme, meta *api.ResponseMeta) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		label := func(txt string, col color.NRGBA, bold bool) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				l := material.Label(th, authSp(12), txt)
				l.Color = col
				if bold {
					l.Font.Weight = font.Bold
				}
				return l.Layout(gtx)
			}
		}

		rest := ""
		if meta.First > 0 {
			rest += " · first word " + formatSeconds(meta.First)
		}
		if meta.Interrupted {
			rest += " · interrupted"
		}

		return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					w, h := gtx.Constraints.Max.X, gtx.Dp(unit.Dp(1))
					paint.FillShape(gtx.Ops, authCardBorder, clip.Rect{Max: image.Pt(w, h)}.Op())
					return layout.Dimensions{Size: image.Pt(w, h+gtx.Dp(unit.Dp(9)))}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return clockIcon(gtx, authLink, gtx.Dp(unit.Dp(13)))
						}),
						layout.Rigid(spacerX(6)),
						layout.Rigid(label("Responded in ", authHint, false)),
						layout.Rigid(label(formatSeconds(meta.Total), authBody, true)),
						layout.Rigid(label(rest, authHint, false)),
					)
				}),
			)
		})
	}
}

// clockIcon draws a small outlined clock (circle with two hands).
func clockIcon(gtx layout.Context, col color.NRGBA, d int) layout.Dimensions {
	stroke := float32(gtx.Dp(unit.Dp(1.4)))
	r := float32(d) / 2
	c := f32.Pt(r, r)

	circle := clip.Ellipse{
		Min: image.Pt(int(stroke/2), int(stroke/2)),
		Max: image.Pt(d-int(stroke/2), d-int(stroke/2)),
	}.Path(gtx.Ops)
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: circle, Width: stroke}.Op())

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(c.X, c.Y-r*0.55))
	p.LineTo(c)
	p.LineTo(f32.Pt(c.X+r*0.45, c.Y+r*0.25))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: p.End(), Width: stroke}.Op())

	return layout.Dimensions{Size: image.Pt(d, d)}
}
