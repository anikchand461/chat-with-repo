package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
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

// Bubble renders one chat message as a rounded, colored bubble aligned
// to the right (user) or left (assistant), with basic Markdown-ish
// rendering of the content (bold, inline code, fenced code blocks).
func Bubble(th *material.Theme, role, content string) layout.Widget {
	isUser := role == "user"
	bg := colorBubbleBot
	if isUser {
		bg = colorBubbleUser
	}
	align := layout.W
	if isUser {
		align = layout.E
	}

	return func(gtx layout.Context) layout.Dimensions {
		maxWidth := gtx.Constraints.Max.X * 82 / 100
		return align.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = maxWidth
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					roundedFill(gtx, gtx.Constraints.Min, unit.Dp(14), bg)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return renderContent(gtx, th, content)
					})
				}),
			)
		})
	}
}

// renderContent does a small, dependency-free pass over the message
// text: fenced code blocks get a monospace block with a dark
// background, everything else is rendered as wrapped body text with
// **bold** markers stripped-and-applied. This is intentionally basic —
// it is not a full Markdown renderer, just enough for readable answers.
func renderContent(gtx layout.Context, th *material.Theme, raw string) layout.Dimensions {
	blocks := splitCodeBlocks(raw)

	children := make([]layout.FlexChild, 0, len(blocks))
	for _, b := range blocks {
		b := b
		if b.code {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							roundedFill(gtx, gtx.Constraints.Min, unit.Dp(8), colorCodeBg)
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, b.text)
								lbl.Font.Typeface = "monospace"
								lbl.Color = colorText
								return lbl.Layout(gtx)
							})
						}),
					)
				})
			}))
			continue
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, stripBold(b.text))
			lbl.Color = colorText
			return lbl.Layout(gtx)
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

type contentBlock struct {
	text string
	code bool
}

// splitCodeBlocks separates ```fenced``` sections from normal text.
func splitCodeBlocks(raw string) []contentBlock {
	parts := strings.Split(raw, "```")
	blocks := make([]contentBlock, 0, len(parts))
	for i, p := range parts {
		p = strings.Trim(p, "\n")
		if p == "" {
			continue
		}
		blocks = append(blocks, contentBlock{text: p, code: i%2 == 1})
	}
	if len(blocks) == 0 {
		return []contentBlock{{text: raw}}
	}
	return blocks
}

// stripBold removes ** / __ markers. Gio's material.Label doesn't do
// inline rich text without a lot more plumbing, so for this minimal
// client we just clean the markers rather than mixing weights
// mid-line.
func stripBold(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	return s
}