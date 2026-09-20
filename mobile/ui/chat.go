package ui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"chatwithrepo/mobile/api"
)

// ChatScreen shows one conversation: its message history, a bottom
// input row, and a back button to return to the Dashboard.
type ChatScreen struct {
	app *App

	mu            sync.Mutex
	chatID        string
	title         string
	branch        string
	messages      []api.Message
	loadingHist   bool
	asking        bool
	errMsg        string
	scrollPending bool

	list     widget.List
	question widget.Editor
	sendBtn  widget.Clickable
	// Left drawer (UI-goroutine only).
	showMenu    bool
	menuProg    float32
	menuLast    time.Time
	menuBtn     widget.Clickable
	menuScrim   widget.Clickable
	menuSheet   widget.Clickable
	newChatBtn  widget.Clickable
	githubBtn   widget.Clickable
	dashBtn     widget.Clickable
	logoutBtn   widget.Clickable
	sideList    widget.List
	sideChatBtn []widget.Clickable

	// "Upgrade to Pro" modal, shown when an ask comes back with
	// UpgradeRequired (e.g. the free plan's daily question limit —
	// see js/app.js chat.html ask handler for the web equivalent).
	showUpgrade   bool
	upgradeMsg    string
	upgradeBtn    widget.Clickable
	upgradeCancel widget.Clickable
	checkingOut   bool
	checkoutErr   string
}

func newChatScreen(app *App) *ChatScreen {
	s := &ChatScreen{app: app}
	s.list.Axis = layout.Vertical
	s.sideList.Axis = layout.Vertical
	// Follow new content while the user is at the bottom; scrolling up
	// (Position.BeforeEnd) releases it, so they are never dragged back.
	s.list.ScrollToEnd = true
	s.question.SingleLine = false
	return s
}

// Open resets the screen for chatID and loads its message history.
func (s *ChatScreen) Open(chatID, title, branch string) {
	s.mu.Lock()
	s.chatID = chatID
	s.title = title
	s.branch = branch
	s.messages = nil
	s.errMsg = ""
	s.loadingHist = true
	s.showUpgrade = false
	s.checkoutErr = ""
	s.mu.Unlock()
	s.question.SetText("")
	s.app.Window.Invalidate()

	go func() {
		msgs, err := s.app.Client.Messages(chatID)
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		s.mu.Lock()
		s.loadingHist = false
		if err != nil {
			s.errMsg = err.Error()
		} else {
			s.messages = msgs
			s.scrollPending = true
		}
		s.mu.Unlock()
		s.app.Window.Invalidate()
	}()
}

func (s *ChatScreen) submitAsk() {
	question := strings.TrimSpace(s.question.Text())
	if question == "" {
		return
	}
	s.mu.Lock()
	chatID := s.chatID
	s.messages = append(s.messages, api.Message{Role: "user", Content: question})
	s.asking = true
	s.errMsg = ""
	s.scrollPending = true
	s.mu.Unlock()
	s.question.SetText("")
	s.app.Window.Invalidate()

	go func() {
		resp, err := s.app.Client.Ask(chatID, question)
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		s.mu.Lock()
		s.asking = false
		switch {
		case err != nil:
			s.errMsg = err.Error()
		case resp.UpgradeRequired:
			// Mirrors the web frontend's chat.html ask handler: show
			// the limit message inline, then surface the upgrade
			// modal instead of pretending an answer came back.
			msg := resp.Message
			if msg == "" {
				msg = "You've hit a free-plan limit."
			}
			s.messages = append(s.messages, api.Message{
				Role:    "assistant",
				Content: "⚠️ " + msg + "\n\nUpgrade to Pro to continue chatting.",
			})
			reason := resp.Reason
			if reason == "" {
				reason = "daily_questions"
			}
			s.showUpgrade = true
			s.upgradeMsg = upgradeMessageFor(reason)
			s.checkoutErr = ""
		default:
			s.messages = append(s.messages, api.Message{Role: "assistant", Content: resp.Answer})
		}
		s.mu.Unlock()
		s.app.Window.Invalidate()
	}()
}

func (s *ChatScreen) closeUpgradeModal() {
	s.mu.Lock()
	s.showUpgrade = false
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

// startCheckout mirrors DashboardScreen.startCheckout: fetch a hosted
// payment session and open it in the system browser.
func (s *ChatScreen) startCheckout() {
	s.mu.Lock()
	s.checkingOut = true
	s.checkoutErr = ""
	s.mu.Unlock()
	s.app.Window.Invalidate()

	go func() {
		url, err := s.app.Client.CreateCheckout()
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		s.mu.Lock()
		s.checkingOut = false
		if err != nil {
			s.checkoutErr = err.Error()
		}
		s.mu.Unlock()
		s.app.Window.Invalidate()

		if err != nil {
			return
		}
		if openErr := openURL(url); openErr != nil {
			s.mu.Lock()
			s.checkoutErr = "Couldn't open a browser automatically. Payment link: " + url
			s.mu.Unlock()
			s.app.Window.Invalidate()
		}
	}()
}

// chatSnapshot is everything Layout needs to draw one frame, read out
// under s.mu in one shot.
type chatSnapshot struct {
	title, branch string
	messages      []api.Message
	loadingHist   bool
	asking        bool
	errMsg        string
	scrollPending bool

	showUpgrade bool
	upgradeMsg  string
	checkingOut bool
	checkoutErr string
}

func (s *ChatScreen) snapshot() chatSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	messages := make([]api.Message, len(s.messages))
	copy(messages, s.messages)
	scrollPending := s.scrollPending
	s.scrollPending = false
	return chatSnapshot{
		title:         s.title,
		branch:        s.branch,
		messages:      messages,
		loadingHist:   s.loadingHist,
		asking:        s.asking,
		errMsg:        s.errMsg,
		scrollPending: scrollPending,
		showUpgrade:   s.showUpgrade,
		upgradeMsg:    s.upgradeMsg,
		checkingOut:   s.checkingOut,
		checkoutErr:   s.checkoutErr,
	}
}

// Layout draws the Chat screen and processes its own clicks.
func (s *ChatScreen) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	authS = authScale(gtx)
	stepProgress(gtx, s.showMenu, &s.menuProg, &s.menuLast)

	for s.menuBtn.Clicked(gtx) {
		s.showMenu = true
	}
	for s.menuScrim.Clicked(gtx) {
		s.showMenu = false
	}
	for s.menuSheet.Clicked(gtx) {
	}
	for s.githubBtn.Clicked(gtx) {
		s.showMenu = false
		go func() { _ = openURL(repoURL) }()
	}
	for s.dashBtn.Clicked(gtx) {
		s.showMenu = false
		s.app.ShowDashboard()
	}
	for s.logoutBtn.Clicked(gtx) {
		s.showMenu = false
		s.app.Logout()
	}
	for s.newChatBtn.Clicked(gtx) {
		// Reuse the Dashboard's existing Create Chat flow (and its
		// free-plan limit check) rather than duplicating it here.
		s.showMenu = false
		s.app.ShowDashboard()
		s.app.Dashboard.attemptOpenModal()
	}

	dash := s.app.Dashboard.snapshot()
	if len(s.sideChatBtn) != len(dash.chats) {
		s.sideChatBtn = make([]widget.Clickable, len(dash.chats))
	}
	s.mu.Lock()
	curID := s.chatID
	s.mu.Unlock()
	for i := range s.sideChatBtn {
		if !s.sideChatBtn[i].Clicked(gtx) {
			continue
		}
		s.showMenu = false
		c := dash.chats[i]
		switch {
		case string(c.ChatID) == curID:
		case !dash.isPro && i >= freeChatLimit:
			s.app.ShowDashboard()
			s.app.Dashboard.openUpgradeModal("repo_limit")
		default:
			s.app.ShowChat(string(c.ChatID), c.Title, c.Branch)
		}
		break
	}

	for s.sendBtn.Clicked(gtx) {
		s.submitAsk()
	}
	for s.upgradeBtn.Clicked(gtx) {
		s.startCheckout()
	}
	for s.upgradeCancel.Clicked(gtx) {
		s.closeUpgradeModal()
	}

	snap := s.snapshot()
	if snap.scrollPending {
		// Explicit "go to the bottom": history loaded or the user sent.
		s.list.Position.BeforeEnd = false
		s.list.ScrollToEnd = true
	}

	// Once a daily-question limit hits, further asking is blocked
	// until the user upgrades — don't let them keep hammering a send
	// button that will only come back with the same limit message.
	asking := snap.asking || snap.showUpgrade

	authBackground(gtx)
	gtx.Constraints.Min = gtx.Constraints.Max
	content := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.topBar(th, snap.title, snap.branch)),
		layout.Flexed(1, s.messageList(th, snap.messages, snap.loadingHist, snap.asking)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if snap.errMsg == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Bottom: unit.Dp(6)}.Layout(gtx, ErrorText(th, snap.errMsg))
		}),
		layout.Rigid(s.inputRow(th, asking)),
	)

	if snap.showUpgrade {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return content }),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return fill(gtx, colorScrim()) }),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				return UpgradeModal(th, snap.upgradeMsg, &s.upgradeBtn, &s.upgradeCancel, snap.checkingOut, snap.checkoutErr)(gtx)
			}),
		)
	}
	if s.showMenu || s.menuProg > 0 {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return content }),
			layout.Expanded(s.sidebar(th, dash.chats, dash.isPro, curID)),
		)
	}
	return content
}

func (s *ChatScreen) topBar(th *material.Theme, title, branch string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(10), Bottom: unit.Dp(8)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return hamburgerButton(gtx, &s.menuBtn)
					}),
					layout.Rigid(spacerX(12)),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(th, authSp(17), title)
								l.Color = authTitle
								l.Font.Weight = font.Bold
								l.MaxLines = 1
								l.Truncator = "…"
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(th, authSp(12), "Branch: "+branch)
								l.Color = authBody
								l.MaxLines = 1
								return l.Layout(gtx)
							}),
						)
					}),
				)
			})
	}
}

func (s *ChatScreen) messageList(th *material.Theme, messages []api.Message, loadingHist, typing bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if loadingHist && len(messages) == 0 {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, authSp(15), "Loading conversation…")
				lbl.Color = authBody
				return lbl.Layout(gtx)
			})
		}
		if len(messages) == 0 && !typing {
			return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, authSp(15), "Ask anything about this repository — architecture, files, functions, or bugs.")
				lbl.LineHeightScale = 1.25
				lbl.Color = authBody
				return lbl.Layout(gtx)
			})
		}
		return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			n := len(messages)
			if typing {
				n++
			}
			return s.list.Layout(gtx, n, func(gtx layout.Context, i int) layout.Dimensions {
				if i == len(messages) {
					return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, typingBubble)
				}
				m := messages[i]
				return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, Bubble(th, m.Role, m.Content))
			})
		})
	}
}

// inputRow is the bottom composer: a rounded multi-line field and a round
// green send button.
func (s *ChatScreen) inputRow(th *material.Theme, asking bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(8), Bottom: unit.Dp(14)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
					layout.Flexed(1, s.composerField(th)),
					layout.Rigid(spacerX(10)),
					layout.Rigid(s.sendButton(th, asking)),
				)
			})
	}
}

func (s *ChatScreen) composerField(th *material.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		minH := gtx.Dp(authDp(48))
		padX := gtx.Dp(unit.Dp(18))
		size := authSp(16)

		// Measure one line so the editor can be capped at ~4 lines and
		// centred vertically when it is a single line.
		pg := gtx
		pg.Constraints.Min = image.Point{}
		m := op.Record(gtx.Ops)
		lh := material.Label(th, size, "Ag").Layout(pg).Size.Y
		m.Stop()

		inner := gtx
		inner.Constraints.Min = image.Point{}
		inner.Constraints.Max.X -= 2 * padX
		inner.Constraints.Max.Y = 4 * lh

		rec := op.Record(gtx.Ops)
		ed := material.Editor(th, &s.question, "Ask about this repository…")
		ed.TextSize = size
		ed.Color = authTitle
		ed.HintColor = authHint
		ed.SelectionColor = color.NRGBA{R: 0x10, G: 0xa3, B: 0x72, A: 0x66}
		edDims := ed.Layout(inner)
		call := rec.Stop()

		h := edDims.Size.Y + 2*gtx.Dp(unit.Dp(12))
		if h < minH {
			h = minH
		}
		sz := image.Pt(gtx.Constraints.Max.X, h)
		borderedRRect(gtx, sz, unit.Dp(24), authField, authFieldBorder)
		off := op.Offset(image.Pt(padX, (h-edDims.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		return layout.Dimensions{Size: sz}
	}
}

func (s *ChatScreen) sendButton(th *material.Theme, asking bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if asking {
			gtx = gtx.Disabled()
		}
		d := gtx.Dp(authDp(48))
		gtx.Constraints.Min, gtx.Constraints.Max = image.Pt(d, d), image.Pt(d, d)
		return s.sendBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			c1, c2 := authBtnLeft, authBtnRight
			if asking {
				c1.A, c2.A = 0xa0, 0xa0
			}
			st := clip.Ellipse{Max: image.Pt(d, d)}.Push(gtx.Ops)
			paint.LinearGradientOp{
				Stop1: f32.Pt(0, 0), Color1: c1,
				Stop2: f32.Pt(float32(d), float32(d)), Color2: c2,
			}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			st.Pop()

			if asking {
				g := gtx
				ld := d / 2
				g.Constraints.Min, g.Constraints.Max = image.Pt(ld, ld), image.Pt(ld, ld)
				defer op.Offset(image.Pt(d/4, d/4)).Push(gtx.Ops).Pop()
				loader := material.Loader(th)
				loader.Color = authBtnText
				loader.Layout(g)
				return layout.Dimensions{Size: image.Pt(d, d)}
			}

			// Send arrow (paper-plane style).
			cx, cy, r := float32(d)/2, float32(d)/2, float32(d)*0.24
			var p clip.Path
			p.Begin(gtx.Ops)
			p.MoveTo(f32.Pt(cx+r, cy))
			p.LineTo(f32.Pt(cx-r, cy-r*0.9))
			p.LineTo(f32.Pt(cx-r*0.4, cy))
			p.LineTo(f32.Pt(cx-r, cy+r*0.9))
			p.Close()
			paint.FillShape(gtx.Ops, authBtnText, clip.Outline{Path: p.End()}.Op())
			return layout.Dimensions{Size: image.Pt(d, d)}
		})
	}
}

// sidebar is the left slide-in drawer: New Chat, previous chats (from the
// Dashboard's existing chat list) and the GitHub / Dashboard / Logout actions.
func (s *ChatScreen) sidebar(th *material.Theme, chats []api.Chat, isPro bool, curID string) layout.Widget {
	const rowH = 52
	action := func(btn *widget.Clickable, icon bool, label string, col color.NRGBA) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			h := gtx.Dp(unit.Dp(rowH))
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := image.Pt(gtx.Constraints.Max.X, h)
				return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					centerY(gtx, h, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								d := gtx.Dp(unit.Dp(22))
								if icon && githubOp != nil {
									g := gtx
									g.Constraints.Min, g.Constraints.Max = image.Pt(d, d), image.Pt(d, d)
									widget.Image{Src: *githubOp, Fit: widget.Contain, Position: layout.Center}.Layout(g)
								}
								if !icon {
									return layout.Dimensions{}
								}
								return layout.Dimensions{Size: image.Pt(d+gtx.Dp(unit.Dp(14)), d)}
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(th, unit.Sp(16), label)
								l.Color = col
								l.Font.Weight = font.Bold
								return l.Layout(gtx)
							}),
						)
					})
					return layout.Dimensions{Size: sz}
				})
			})
		}
	}

	chatRow := func(i int) layout.Widget {
		c := chats[i]
		selected := string(c.ChatID) == curID
		locked := !isPro && i >= freeChatLimit
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return s.sideChatBtn[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					h := gtx.Dp(unit.Dp(56))
					sz := image.Pt(gtx.Constraints.Max.X, h)
					if selected {
						borderedRRect(gtx, sz, unit.Dp(14), color.NRGBA{R: 0x10, G: 0x3a, B: 0x2c, A: 0xff}, color.NRGBA{R: 0x1a, G: 0x5a, B: 0x44, A: 0xff})
					}
					return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						centerY(gtx, h, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(th, unit.Sp(15), c.Title)
											l.Color = authTitle
											if locked {
												l.Color = authBody
											}
											l.Font.Weight = font.Bold
											l.MaxLines = 1
											l.Truncator = "…"
											return l.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(th, unit.Sp(12), "Branch: "+c.Branch)
											l.Color = authHint
											l.MaxLines = 1
											return l.Layout(gtx)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if !locked {
										return layout.Dimensions{}
									}
									l := material.Label(th, unit.Sp(12), "Pro")
									l.Color = authLink
									l.Font.Weight = font.Bold
									return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, l.Layout)
								}),
							)
						})
						return layout.Dimensions{Size: sz}
					})
				})
			})
		}
	}

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		gtx.Constraints.Min = size
		e := s.menuProg * s.menuProg * (3 - 2*s.menuProg)

		dw := gtx.Dp(unit.Dp(300))
		if lim := size.X * 84 / 100; dw > lim {
			dw = lim
		}

		scrim := colorScrim()
		scrim.A = uint8(float32(scrim.A) * e)
		s.menuScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return fill(gtx, scrim)
		})

		x := -int(float32(dw) * (1 - e))
		defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
		pg := gtx
		pg.Constraints.Min, pg.Constraints.Max = image.Pt(dw, size.Y), image.Pt(dw, size.Y)
		return s.menuSheet.Layout(pg, func(gtx layout.Context) layout.Dimensions {
			r := gtx.Dp(unit.Dp(28))
			paint.FillShape(gtx.Ops, authCardBorder, clip.RRect{Rect: image.Rectangle{Max: gtx.Constraints.Max}, NE: r, SE: r}.Op(gtx.Ops))
			paint.FillShape(gtx.Ops, authCard, clip.RRect{Rect: image.Rect(0, 0, dw-gtx.Dp(unit.Dp(1)), size.Y), NE: r, SE: r}.Op(gtx.Ops))

			layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(20), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return AuthButton(th, &s.newChatBtn, "+ New Chat", false)(gtx)
					}),
					layout.Rigid(spacer(18)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, unit.Sp(12), "PREVIOUS CHATS")
						l.Color = authHint
						l.Font.Weight = font.Bold
						return layout.Inset{Left: unit.Dp(4), Bottom: unit.Dp(8)}.Layout(gtx, l.Layout)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if len(chats) == 0 {
							l := material.Label(th, unit.Sp(14), "No chats yet.")
							l.Color = authBody
							return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, l.Layout)
						}
						return s.sideList.Layout(gtx, len(chats), func(gtx layout.Context, i int) layout.Dimensions {
							return chatRow(i)(gtx)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						h := gtx.Dp(unit.Dp(1))
						w := gtx.Constraints.Max.X
						paint.FillShape(gtx.Ops, authCardBorder, clip.Rect{Max: image.Pt(w, h)}.Op())
						return layout.Dimensions{Size: image.Pt(w, h+gtx.Dp(unit.Dp(8)))}
					}),
					layout.Rigid(action(&s.githubBtn, true, "GitHub", authTitle)),
					layout.Rigid(action(&s.dashBtn, false, "Dashboard", authTitle)),
					layout.Rigid(action(&s.logoutBtn, false, "Logout", colorError)),
				)
			})
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	}
}

// spacerX returns a fixed-width blank widget for horizontal gaps.
func spacerX(dp int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(dp))}}
	}
}

// stepProgress animates the sidebar opening and closing.
func stepProgress(
	gtx layout.Context,
	open bool,
	progress *float32,
	last *time.Time,
) {
	target := float32(0)
	if open {
		target = 1
	}

	const speed = float32(0.18)

	if *progress < target {
		*progress += speed
		if *progress > target {
			*progress = target
		}
	} else if *progress > target {
		*progress -= speed
		if *progress < target {
			*progress = target
		}
	}

	*last = gtx.Now

	if *progress != target {
		gtx.Execute(op.InvalidateCmd{
			At: gtx.Now.Add(16 * time.Millisecond),
		})
	}
}

// hamburgerButton draws the hamburger menu button.
func hamburgerButton(
	gtx layout.Context,
	btn *widget.Clickable,
) layout.Dimensions {
	const sizeDp = 44

	d := gtx.Dp(unit.Dp(sizeDp))

	gtx.Constraints.Min = image.Pt(d, d)
	gtx.Constraints.Max = image.Pt(d, d)

	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		r := gtx.Dp(unit.Dp(12))

		paint.FillShape(
			gtx.Ops,
			authField,
			clip.RRect{
				Rect: image.Rectangle{
					Max: image.Pt(d, d),
				},
				NE: r,
				NW: r,
				SE: r,
				SW: r,
			}.Op(gtx.Ops),
		)

		lineColor := authTitle
		lineWidth := gtx.Dp(unit.Dp(18))
		lineHeight := gtx.Dp(unit.Dp(2))
		x := (d - lineWidth) / 2

		for _, y := range []int{
			d/2 - gtx.Dp(unit.Dp(7)),
			d / 2,
			d/2 + gtx.Dp(unit.Dp(7)),
		} {
			paint.FillShape(
				gtx.Ops,
				lineColor,
				clip.Rect{
					Min: image.Pt(x, y),
					Max: image.Pt(x+lineWidth, y+lineHeight),
				}.Op(),
			)
		}

		return layout.Dimensions{
			Size: image.Pt(d, d),
		}
	})
}

// typingBubble is the assistant "…" indicator: three dots whose scale and
// opacity ripple in sequence. It is only laid out while a request is in
// flight and asks Gio for the next frame each time it draws, so the
// animation runs at the display's frame rate and stops the moment the
// bubble leaves the list.
func typingBubble(gtx layout.Context) layout.Dimensions {
	gtx.Execute(op.InvalidateCmd{})
	t := float64(gtx.Now.UnixNano()%int64(time.Hour)) / float64(time.Second)

	maxR := gtx.Dp(unit.Dp(4.5))
	gap := gtx.Dp(unit.Dp(7))
	padX, padY := gtx.Dp(unit.Dp(18)), gtx.Dp(unit.Dp(15))
	w := 2*padX + 3*2*maxR + 2*gap
	h := 2*padY + 2*maxR
	sz := image.Pt(w, h)
	borderedRRect(gtx, sz, unit.Dp(20), authCard, authCardBorder)

	for i := 0; i < 3; i++ {
		wave := (math.Sin(2*math.Pi*(t/1.2-float64(i)*0.16)) + 1) / 2
		r := int(float64(maxR) * (0.65 + 0.35*wave))
		c := authLink
		c.A = uint8(90 + 165*wave)
		cx := padX + maxR + i*(2*maxR+gap)
		cy := h / 2
		paint.FillShape(gtx.Ops, c, clip.Ellipse{Min: image.Pt(cx-r, cy-r), Max: image.Pt(cx+r, cy+r)}.Op(gtx.Ops))
	}
	return layout.Dimensions{Size: sz}
}
