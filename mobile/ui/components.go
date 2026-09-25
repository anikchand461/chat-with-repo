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
	"gioui.org/op"
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
// as typed. sources/chipBtns (assistant only) draw the "which files this
// answer's context came from" chip row above the text, same as the web
// frontend's source chips - chipBtns must be pre-sized to len(sources) by
// the caller (see ChatScreen.sourceBtns) since Gio's Clickable needs a
// stable identity across frames to detect taps. copyBtn (assistant only,
// nil for user messages) draws a "copy response" button at the bottom,
// same as the web frontend; copied swaps its icon to a checkmark briefly
// after a tap - the caller (ChatScreen) owns both the click handling and
// the "still showing checkmark" timing.
func Bubble(th *material.Theme, role, content string, meta *api.ResponseMeta, sources []string, chipBtns []widget.Clickable, copyBtn *widget.Clickable, copied bool) layout.Widget {
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
						children := []layout.FlexChild{}
						if !isUser && len(sources) > 0 && len(chipBtns) == len(sources) {
							children = append(children,
								layout.Rigid(sourceChipsRow(th, sources, chipBtns)),
								layout.Rigid(spacer(10)),
							)
						}
						children = append(children, layout.Rigid(md))
						if !isUser && meta != nil {
							children = append(children, layout.Rigid(responseFooter(th, meta)))
						}
						if !isUser && copyBtn != nil {
							children = append(children,
								layout.Rigid(spacer(8)),
								layout.Rigid(copyButtonRow(th, copyBtn, copied)),
							)
						}
						if len(children) == 1 {
							return md(gtx)
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					})
				}),
			)
		})
	}
}

// sourceColors gives each file extension a distinct, recognizable color for
// its chip's dot - not exact brand logos (no bundled icon set), same
// approach and palette as the web frontend's SOURCE_COLORS (js/app.js).
var sourceColors = map[string]color.NRGBA{
	"py":         hexColor("#3776AB"),
	"js":         hexColor("#F7DF1E"),
	"mjs":        hexColor("#F7DF1E"),
	"cjs":        hexColor("#F7DF1E"),
	"jsx":        hexColor("#61DAFB"),
	"ts":         hexColor("#3178C6"),
	"tsx":        hexColor("#3178C6"),
	"go":         hexColor("#00ADD8"),
	"mod":        hexColor("#00ADD8"),
	"sum":        hexColor("#00ADD8"),
	"java":       hexColor("#EA2D2E"),
	"rb":         hexColor("#CC342D"),
	"php":        hexColor("#777BB4"),
	"c":          hexColor("#5C6BC0"),
	"h":          hexColor("#5C6BC0"),
	"cpp":        hexColor("#00599C"),
	"cc":         hexColor("#00599C"),
	"hpp":        hexColor("#00599C"),
	"cs":         hexColor("#9B4F96"),
	"rs":         hexColor("#DEA584"),
	"kt":         hexColor("#7F52FF"),
	"swift":      hexColor("#F05138"),
	"scala":      hexColor("#DC322F"),
	"dart":       hexColor("#0175C2"),
	"vue":        hexColor("#42B883"),
	"svelte":     hexColor("#FF3E00"),
	"html":       hexColor("#E34F26"),
	"htm":        hexColor("#E34F26"),
	"css":        hexColor("#1572B6"),
	"scss":       hexColor("#CC6699"),
	"sass":       hexColor("#CC6699"),
	"json":       hexColor("#8A8A8A"),
	"yaml":       hexColor("#CB171E"),
	"yml":        hexColor("#CB171E"),
	"toml":       hexColor("#9C4221"),
	"md":         hexColor("#4A5568"),
	"mdx":        hexColor("#4A5568"),
	"sql":        hexColor("#336791"),
	"sh":         hexColor("#4EAA25"),
	"bash":       hexColor("#4EAA25"),
	"dockerfile": hexColor("#2496ED"),
	"xml":        hexColor("#0060AC"),
	"ini":        hexColor("#6B7280"),
	"cfg":        hexColor("#6B7280"),
	"txt":        hexColor("#6B7280"),
}

var defaultSourceColor = hexColor("#6B7280")

// hexColor parses a "#RRGGBB" literal into an opaque color.NRGBA. Panics on
// a malformed literal, which only happens if sourceColors above has a typo
// - i.e. a programmer error caught immediately, not bad input at runtime.
func hexColor(hex string) color.NRGBA {
	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		panic("ui: invalid hexColor literal " + hex)
	}
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

func fileExtension(path string) string {
	name := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		name = path[i+1:]
	}
	if i := strings.LastIndex(name, "."); i > 0 {
		return strings.ToLower(name[i+1:])
	}
	return strings.ToLower(name)
}

func fileBaseName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// searchingPanel is the animated placeholder shown from the moment an
// answer's sources are known until its first token arrives - "Searching
// the codebase…" with a pulsing dot, followed by each source path
// revealing itself in turn (mirrors the web frontend's morphed typing
// indicator, js/app.js showSources()). since is when the sources became
// known (ChatScreen.searchStart); the reveal timing is derived from it on
// every frame rather than stored per-panel, since this whole widget is
// rebuilt fresh each time messageList runs.
func searchingPanel(th *material.Theme, sources []string, since time.Time) layout.Widget {
	const revealEvery = 90 * time.Millisecond
	const fadeIn = 220 * time.Millisecond

	return func(gtx layout.Context) layout.Dimensions {
		maxWidth := gtx.Constraints.Max.X * 88 / 100
		return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = maxWidth

			gtx.Execute(op.InvalidateCmd{}) // keep animating (pulse + reveal) until replaced
			elapsed := gtx.Now.Sub(since)
			if elapsed < 0 {
				elapsed = 0
			}

			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return dotsIndicator(gtx, authAccent, gtx.Dp(unit.Dp(3.5)))
						}),
						layout.Rigid(spacerX(10)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(th, authSp(13), "Searching the codebase…")
							l.Color = authHint
							l.Font.Weight = font.Bold
							return l.Layout(gtx)
						}),
					)
				}),
			}

			for i, path := range sources {
				revealAt := time.Duration(i) * revealEvery
				if elapsed < revealAt {
					break // later files haven't revealed yet either
				}
				fade := float32(1)
				if in := elapsed - revealAt; in < fadeIn {
					fade = float32(in) / float32(fadeIn)
				}
				path := path
				children = append(children,
					layout.Rigid(spacer(6)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return searchingFileRow(gtx, th, path, fade)
					}),
				)
			}

			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(20), authCard, authCardBorder)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(14), Bottom: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					})
				}),
			)
		})
	}
}

// searchingFileRow draws one file path row inside searchingPanel, its
// background/border/text alpha driven by fade (0..1) for the reveal-in
// effect (mirrors the web frontend's sources-in fade/slide keyframe).
func searchingFileRow(gtx layout.Context, th *material.Theme, path string, fade float32) layout.Dimensions {
	textCol := authBody
	textCol.A = uint8(float32(textCol.A) * fade)
	bg, border := authField, authFieldBorder
	bg.A = uint8(float32(bg.A) * fade)
	border.A = uint8(float32(border.A) * fade)

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(8), bg, border)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Label(th, authSp(11.5), path)
				l.Color = textCol
				l.Font.Typeface = "Go Mono"
				l.MaxLines = 1
				l.Truncator = "…"
				return l.Layout(gtx)
			})
		}),
	)
}

// sourceChipsRow lays out one tappable chip per source file - a colored dot
// plus its filename - wrapping onto further rows as needed to fit the
// bubble's width. Tapping a chip is handled by the caller (ChatScreen.Layout
// checks chipBtns[i].Clicked(gtx) and opens the file on GitHub); this only
// draws.
func sourceChipsRow(th *material.Theme, sources []string, chipBtns []widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.Widget, len(sources))
		for i, path := range sources {
			path, btn := path, &chipBtns[i]
			col, ok := sourceColors[fileExtension(path)]
			if !ok {
				col = defaultSourceColor
			}
			children[i] = func(gtx layout.Context) layout.Dimensions {
				return sourceChip(gtx, th, btn, col, fileBaseName(path))
			}
		}
		return flowWrap(gtx, gtx.Dp(unit.Dp(6)), gtx.Dp(unit.Dp(6)), children)
	}
}

// sourceChip draws one pill: a colored dot and the filename, on a subtly
// raised, bordered background - tappable via btn.
func sourceChip(gtx layout.Context, th *material.Theme, btn *widget.Clickable, dot color.NRGBA, label string) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(999), authField, authFieldBorder)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(10), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							d := gtx.Dp(unit.Dp(7))
							st := clip.Ellipse{Max: image.Pt(d, d)}.Push(gtx.Ops)
							paint.ColorOp{Color: dot}.Add(gtx.Ops)
							paint.PaintOp{}.Add(gtx.Ops)
							st.Pop()
							return layout.Dimensions{Size: image.Pt(d, d)}
						}),
						layout.Rigid(spacerX(6)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(th, authSp(11.5), label)
							l.Color = authBody
							l.Font.Typeface = "Go Mono"
							l.MaxLines = 1
							return l.Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

// flowWrap lays children out left to right, wrapping onto a new row
// whenever the next one would exceed the available width - the mobile
// equivalent of the web frontend's flex-wrap chip row (CSS has no
// direct Gio counterpart).
func flowWrap(gtx layout.Context, gapX, gapY int, children []layout.Widget) layout.Dimensions {
	maxW := gtx.Constraints.Max.X

	type placed struct {
		call op.CallOp
		pos  image.Point
		sz   image.Point
	}
	items := make([]placed, 0, len(children))

	x, y, rowH := 0, 0, 0
	for _, child := range children {
		cgtx := gtx
		cgtx.Constraints.Min = image.Point{}
		m := op.Record(gtx.Ops)
		dims := child(cgtx)
		call := m.Stop()

		if x > 0 && x+dims.Size.X > maxW {
			x = 0
			y += rowH + gapY
			rowH = 0
		}
		items = append(items, placed{call: call, pos: image.Pt(x, y), sz: dims.Size})
		x += dims.Size.X + gapX
		if dims.Size.Y > rowH {
			rowH = dims.Size.Y
		}
	}

	for _, it := range items {
		off := op.Offset(it.pos).Push(gtx.Ops)
		it.call.Add(gtx.Ops)
		off.Pop()
	}

	total := y + rowH
	if len(items) == 0 {
		total = 0
	}
	return layout.Dimensions{Size: image.Pt(maxW, total)}
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
// copyButtonRow draws the small "Copy" action under an assistant answer,
// mirroring the web frontend's copy button in the same spot. Tapping is
// handled by the caller (ChatScreen.Layout checks copyBtn.Clicked(gtx) and
// writes to the clipboard); copied just controls which icon+label to show.
func copyButtonRow(th *material.Theme, copyBtn *widget.Clickable, copied bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return copyBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					d := gtx.Dp(unit.Dp(13))
					if copied {
						return checkGlyph(gtx, authLink, d)
					}
					return copyGlyph(gtx, authHint, d)
				}),
				layout.Rigid(spacerX(6)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "Copy"
					if copied {
						label = "Copied"
					}
					l := material.Label(th, authSp(12), label)
					l.Color = authHint
					if copied {
						l.Color = authLink
					}
					return l.Layout(gtx)
				}),
			)
		})
	}
}

// copyGlyph draws a small two-rectangle "copy" icon.
func copyGlyph(gtx layout.Context, col color.NRGBA, d int) layout.Dimensions {
	stroke := float32(gtx.Dp(unit.Dp(1.3)))
	r := gtx.Dp(unit.Dp(2))
	inset := d * 3 / 10
	frontSz := d - inset

	back := clip.RRect{Rect: image.Rect(0, 0, frontSz, frontSz), SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: back.Path(gtx.Ops), Width: stroke}.Op())

	off := op.Offset(image.Pt(inset, inset)).Push(gtx.Ops)
	front := clip.RRect{Rect: image.Rect(0, 0, frontSz, frontSz), SE: r, SW: r, NE: r, NW: r}
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: front.Path(gtx.Ops), Width: stroke}.Op())
	off.Pop()

	return layout.Dimensions{Size: image.Pt(d, d)}
}

// checkGlyph draws a small plain checkmark (no circle - copyButtonRow
// already pairs it with a "Copied" label, unlike checkCircleIcon's
// standalone status-box use).
func checkGlyph(gtx layout.Context, col color.NRGBA, d int) layout.Dimensions {
	stroke := float32(gtx.Dp(unit.Dp(1.6)))
	w, h := float32(d), float32(d)

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(w*0.12, h*0.55))
	p.LineTo(f32.Pt(w*0.4, h*0.82))
	p.LineTo(f32.Pt(w*0.9, h*0.2))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: p.End(), Width: stroke}.Op())

	return layout.Dimensions{Size: image.Pt(d, d)}
}

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

// warningTriangleIcon draws a triangle outline with an exclamation mark -
// the status-box icon for a warning/attention state (mirrors the web
// frontend's amber warning-triangle SVG on profile.html).
func warningTriangleIcon(gtx layout.Context, col color.NRGBA, d int) layout.Dimensions {
	stroke := float32(gtx.Dp(unit.Dp(1.5)))
	w, h := float32(d), float32(d)

	var tri clip.Path
	tri.Begin(gtx.Ops)
	tri.MoveTo(f32.Pt(w/2, h*0.06))
	tri.LineTo(f32.Pt(w*0.95, h*0.92))
	tri.LineTo(f32.Pt(w*0.05, h*0.92))
	tri.Close()
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: tri.End(), Width: stroke}.Op())

	var mark clip.Path
	mark.Begin(gtx.Ops)
	mark.MoveTo(f32.Pt(w/2, h*0.38))
	mark.LineTo(f32.Pt(w/2, h*0.64))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: mark.End(), Width: stroke}.Op())

	dr := stroke * 0.7
	dc := f32.Pt(w/2, h*0.78)
	dot := clip.Ellipse{
		Min: image.Pt(int(dc.X-dr), int(dc.Y-dr)),
		Max: image.Pt(int(dc.X+dr), int(dc.Y+dr)),
	}.Op(gtx.Ops)
	paint.FillShape(gtx.Ops, col, dot)

	return layout.Dimensions{Size: image.Pt(d, d)}
}

// checkCircleIcon draws a circled checkmark - the status-box icon for an
// "all good" state (GitHub token configured, Pro plan).
func checkCircleIcon(gtx layout.Context, col color.NRGBA, d int) layout.Dimensions {
	stroke := float32(gtx.Dp(unit.Dp(1.5)))
	r := float32(d)/2 - stroke/2
	c := f32.Pt(float32(d)/2, float32(d)/2)

	circle := clip.Ellipse{
		Min: image.Pt(int(stroke/2), int(stroke/2)),
		Max: image.Pt(d-int(stroke/2), d-int(stroke/2)),
	}.Path(gtx.Ops)
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: circle, Width: stroke}.Op())

	var check clip.Path
	check.Begin(gtx.Ops)
	check.MoveTo(f32.Pt(c.X-r*0.45, c.Y))
	check.LineTo(f32.Pt(c.X-r*0.1, c.Y+r*0.35))
	check.LineTo(f32.Pt(c.X+r*0.5, c.Y-r*0.3))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: check.End(), Width: stroke}.Op())

	return layout.Dimensions{Size: image.Pt(d, d)}
}
