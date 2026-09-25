package ui

import (
	"fmt"
	"image"
	"image/color"
	"net/url"
	"strings"
	"sync"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/key"
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
	streaming     bool // an assistant message is being written token by token
	lastPaint     time.Time
	errMsg        string
	scrollPending bool

	// sourceBtns[i] holds one Clickable per messages[i].Sources entry - a
	// UI-only, Gio-specific parallel structure, grown/resized once per
	// frame in Layout, never touched from the background goroutines above.
	sourceBtns [][]widget.Clickable
	// searchStart is when the in-flight answer's sources became known
	// (setSources), driving the "Searching the codebase…" reveal
	// animation shown until the first token arrives - see messageList.
	searchStart time.Time

	list     widget.List
	question widget.Editor
	sendBtn  widget.Clickable
	// Left drawer (UI-goroutine only).
	uiReset     bool // consumed by Layout
	overlay     overlayAnim
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
	gen := s.app.Generation()
	s.mu.Lock()
	s.chatID = chatID
	s.title = title
	s.branch = branch
	s.messages = nil
	s.sourceBtns = nil
	s.searchStart = time.Time{}
	s.streaming = false
	s.errMsg = ""
	s.loadingHist = true
	s.showUpgrade = false
	s.checkoutErr = ""
	s.mu.Unlock()
	s.question.SetText("")
	s.app.Window.Invalidate()

	go func() {
		msgs, err := s.app.Client.Messages(chatID)
		if !s.app.IsCurrent(gen) {
			return
		}
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		s.mu.Lock()
		if !s.app.IsCurrent(gen) || s.chatID != chatID {
			s.mu.Unlock()
			return
		}
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

// appendToken adds a streamed piece of the answer to the assistant message
// being written, creating it on the first token (which also hides the
// typing dots).
func (s *ChatScreen) appendToken(gen uint64, chatID, token string) {
	s.mu.Lock()
	if !s.app.IsCurrent(gen) || s.chatID != chatID {
		s.mu.Unlock()
		return
	}
	if !s.streaming {
		s.streaming = true
		s.messages = append(s.messages, api.Message{Role: "assistant"})
	}
	s.messages[len(s.messages)-1].Content += token

	// Repaint at most every ~30ms; tokens can arrive faster than that.
	repaint := time.Since(s.lastPaint) > 30*time.Millisecond
	if repaint {
		s.lastPaint = time.Now()
	}
	s.mu.Unlock()

	if repaint {
		s.app.Window.Invalidate()
	}
}

// setSources attaches the real files retrieval used to the assistant
// message being written, creating it on first arrival exactly like
// appendToken - the "sources" event always arrives before any token, so
// this is normally what creates the message shell for a streamed answer
// (a short-circuited exact-file/smalltalk answer never sends one, and
// appendToken creates the shell itself in that case).
func (s *ChatScreen) setSources(gen uint64, chatID string, sources []string) {
	s.mu.Lock()
	if !s.app.IsCurrent(gen) || s.chatID != chatID {
		s.mu.Unlock()
		return
	}
	if !s.streaming {
		s.streaming = true
		s.messages = append(s.messages, api.Message{Role: "assistant"})
	}
	s.messages[len(s.messages)-1].Sources = sources
	s.searchStart = time.Now()
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

func (s *ChatScreen) submitAsk() {
	question := strings.TrimSpace(s.question.Text())
	if question == "" {
		return
	}
	gen := s.app.Generation()
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
		res, err := s.app.Client.AskStream(chatID, question,
			func(tok string) { s.appendToken(gen, chatID, tok) },
			func(srcs []string) { s.setSources(gen, chatID, srcs) },
		)
		if !s.app.IsCurrent(gen) {
			return
		}
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		s.mu.Lock()
		if !s.app.IsCurrent(gen) || s.chatID != chatID {
			s.mu.Unlock()
			return
		}
		s.asking = false
		wasStreaming := s.streaming
		s.streaming = false
		switch {
		case err != nil:
			s.errMsg = err.Error()
			// Keep any partial answer that was already written.
		case res.UpgradeRequired:
			// Mirrors the web frontend's chat.html ask handler: show
			// the limit message inline, then surface the upgrade
			// modal instead of pretending an answer came back.
			msg := res.Message
			if msg == "" {
				msg = "You've hit a free-plan limit."
			}
			s.messages = append(s.messages, api.Message{
				Role:    "assistant",
				Content: "⚠️ " + msg + "\n\nUpgrade to Pro to continue chatting.",
			})
			reason := res.Reason
			if reason == "" {
				reason = "daily_questions"
			}
			s.showUpgrade = true
			s.upgradeMsg = upgradeMessageFor(reason)
			s.checkoutErr = ""
		case wasStreaming:
			// Show "Responded in Xs" under the finished answer. The server
			// stores the same value, so it also appears after a reload and
			// on the web.
			meta := res.Meta
			s.messages[len(s.messages)-1].Meta = &meta
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
	gen := s.app.Generation()
	s.mu.Lock()
	s.checkingOut = true
	s.checkoutErr = ""
	s.mu.Unlock()
	s.app.Window.Invalidate()

	go func() {
		url, err := s.app.Client.CreateCheckout()
		if !s.app.IsCurrent(gen) {
			return
		}
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		s.mu.Lock()
		if !s.app.IsCurrent(gen) {
			s.mu.Unlock()
			return
		}
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
	streaming     bool
	errMsg        string
	scrollPending bool
	searchStart   time.Time

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
		streaming:     s.streaming,
		errMsg:        s.errMsg,
		scrollPending: scrollPending,
		searchStart:   s.searchStart,
		showUpgrade:   s.showUpgrade,
		upgradeMsg:    s.upgradeMsg,
		checkingOut:   s.checkingOut,
		checkoutErr:   s.checkoutErr,
	}
}

// Layout draws the Chat screen and processes its own clicks.
func (s *ChatScreen) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	authS = authScale(gtx)

	s.mu.Lock()
	doReset := s.uiReset
	s.uiReset = false
	s.mu.Unlock()
	if doReset {
		s.showMenu, s.menuProg = false, 0
		s.overlay = overlayAnim{}
		s.question.SetText("")
		s.list = widget.List{List: layout.List{Axis: layout.Vertical, ScrollToEnd: true}}
		s.sideList = widget.List{List: layout.List{Axis: layout.Vertical}}
		s.sideChatBtn = nil
	}

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
			// Never assume the chat is ready - Dashboard.openChat checks
			// first (and retries a failed one), same as tapping it from
			// the dashboard's own "Previous chats" list.
			s.app.ShowDashboard()
			s.app.Dashboard.openChat(c)
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

	// Keep sourceBtns in lockstep with snap.messages, and process taps -
	// resized here (UI thread only, once a frame) rather than at every
	// place s.messages can grow, so it can never drift out of sync with
	// however that ends up happening.
	var repoOwner, repoName string
	for _, c := range dash.chats {
		if string(c.ChatID) == curID {
			repoOwner, repoName = c.Owner, c.Repo
			break
		}
	}
	for len(s.sourceBtns) < len(snap.messages) {
		s.sourceBtns = append(s.sourceBtns, nil)
	}
	for i, m := range snap.messages {
		if len(s.sourceBtns[i]) != len(m.Sources) {
			s.sourceBtns[i] = make([]widget.Clickable, len(m.Sources))
		}
		for j := range s.sourceBtns[i] {
			if !s.sourceBtns[i][j].Clicked(gtx) {
				continue
			}
			url := githubFileURL(repoOwner, repoName, snap.branch, m.Sources[j])
			go func(u string) { _ = openURL(u) }(url)
		}
	}

	// Once a daily-question limit hits, further asking is blocked
	// until the user upgrades — don't let them keep hammering a send
	// button that will only come back with the same limit message.
	asking := snap.asking || snap.showUpgrade

	authBackground(gtx)
	gtx.Constraints.Min = gtx.Constraints.Max
	// Android Back: close the sidebar/dialog, otherwise return to the
	// Dashboard (instead of leaving the app).
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameBack})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			switch {
			case snap.showUpgrade:
				s.closeUpgradeModal()
			case s.showMenu:
				s.showMenu = false
			default:
				s.app.ShowDashboard()
			}
		}
	}

	// Step after this frame's clicks have updated showMenu (see Dashboard):
	// otherwise the opening frame requests no follow-up and, without mouse
	// movement to keep redrawing (i.e. on a phone), the sidebar never slides in.
	stepProgress(gtx, s.showMenu, &s.menuProg, &s.menuLast)

	content := withInsets(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(s.topBar(th, snap.title, snap.branch)),
			layout.Flexed(1, s.messageList(th, snap.messages, s.sourceBtns, snap.searchStart, snap.loadingHist, snap.asking && !snap.streaming)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if snap.errMsg == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Bottom: unit.Dp(6)}.Layout(gtx, ErrorText(th, snap.errMsg))
			}),
			layout.Rigid(s.inputRow(th, asking)),
		)
	})

	s.overlay.step(gtx, snap.showUpgrade)
	if s.overlay.visible() {
		dialog := UpgradeModal(th, snap.upgradeMsg, &s.upgradeBtn, &s.upgradeCancel, snap.checkingOut, snap.checkoutErr)
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return content }),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return s.overlay.draw(gtx, dialog) }),
		)
	}
	if s.showMenu || s.menuProg > 0 {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return content }),
			layout.Expanded(s.sidebar(th, dash.chats, dash.isPro, curID, dash.email)),
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

func (s *ChatScreen) messageList(th *material.Theme, messages []api.Message, sourceBtns [][]widget.Clickable, searchStart time.Time, loadingHist, typing bool) layout.Widget {
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
				// The "sources" event always arrives before any answer
				// text - while content is still empty but sources are
				// known, show the animated "Searching the codebase…"
				// panel instead of an oddly-empty answer bubble. The
				// moment the first token lands, content becomes non-empty
				// and this flips over to the real, growing Bubble below.
				if m.Role == "assistant" && m.Content == "" && len(m.Sources) > 0 {
					return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, searchingPanel(th, m.Sources, searchStart))
				}
				var chips []widget.Clickable
				if i < len(sourceBtns) {
					chips = sourceBtns[i]
				}
				return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, Bubble(th, m.Role, m.Content, m.Meta, m.Sources, chips))
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

			pressOverlay(gtx, &s.sendBtn, image.Pt(d, d), d/2, pressDark)
			if asking {
				// Round spinner while waiting for the answer.
				ld := d / 2
				g := gtx
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
func (s *ChatScreen) sidebar(th *material.Theme, chats []api.Chat, isPro bool, curID, email string) layout.Widget {
	const rowH = 52
	// action draws a sidebar row: icon (GitHub/Logout - see
	// mobile/assets/{github,logout}.png) beside its label. Dashboard has
	// no icon asset, so it's passed nil and just gets the label.
	action := func(btn *widget.Clickable, icon *paint.ImageOp, label string, col color.NRGBA) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			h := gtx.Dp(unit.Dp(rowH))
			return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := image.Pt(gtx.Constraints.Max.X, h)
				pressOverlay(gtx, btn, sz, gtx.Dp(unit.Dp(14)), pressLight)
				return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					centerY(gtx, h, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								d := gtx.Dp(unit.Dp(20))
								if icon != nil {
									g := gtx
									g.Constraints.Min, g.Constraints.Max = image.Pt(d, d), image.Pt(d, d)
									widget.Image{Src: *icon, Fit: widget.Contain, Position: layout.Center}.Layout(g)
								}
								if icon == nil {
									return layout.Dimensions{}
								}
								return layout.Dimensions{Size: image.Pt(d+gtx.Dp(unit.Dp(12)), d)}
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
					pressOverlay(gtx, &s.sideChatBtn[i], sz, gtx.Dp(unit.Dp(14)), pressLight)
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

		dw := gtx.Dp(unit.Dp(250))
		if lim := size.X * 76 / 100; dw > lim {
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

			layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(20) + sysInsets.Top, Bottom: unit.Dp(16) + sysInsets.Bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(greeting(th, email)),
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
					layout.Rigid(action(&s.githubBtn, githubOp, "GitHub", authTitle)),
					layout.Rigid(action(&s.dashBtn, nil, "Dashboard", authTitle)),
					layout.Rigid(action(&s.logoutBtn, logoutOp, "Logout", colorError)),
				)
			})
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	}
}

// githubFileURL mirrors the web frontend's githubFileUrl() (js/app.js):
// github.com/{owner}/{repo}/blob/{branch}/{path}, each path segment (and
// the branch) individually percent-encoded so filenames with spaces/etc.
// still produce a valid URL. Empty owner/repo (e.g. the chat's owner/repo
// couldn't be found in the Dashboard's chat list yet) still returns a URL
// - openURL failing on it is no worse than not opening anything.
func githubFileURL(owner, repo, branch, path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	if branch == "" {
		branch = "main"
	}
	return fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s",
		url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(branch), strings.Join(segments, "/"))
}

// spacerX returns a fixed-width blank widget for horizontal gaps.
func spacerX(dp int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(dp))}}
	}
}

// stepProgress eases *prog toward 1 (open) or 0 (closed) in ~220ms of real
// time (independent of the display's frame rate) and keeps requesting
// frames only while it is still moving.
func stepProgress(gtx layout.Context, open bool, prog *float32, last *time.Time) {
	const duration = 220 * time.Millisecond

	target := float32(0)
	if open {
		target = 1
	}
	dt := gtx.Now.Sub(*last)
	*last = gtx.Now
	if dt <= 0 || dt > 50*time.Millisecond {
		dt = time.Second / 60
	}
	if *prog == target {
		return
	}
	step := float32(dt) / float32(duration)
	if *prog < target {
		*prog += step
		if *prog > target {
			*prog = target
		}
	} else {
		*prog -= step
		if *prog < target {
			*prog = target
		}
	}
	if *prog != target {
		gtx.Execute(op.InvalidateCmd{})
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

		pressOverlay(gtx, btn, image.Pt(d, d), r, pressLight)

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

// typingBubble is the assistant "…" indicator shown while a request is in
// flight; see dotsIndicator for the animation.
func typingBubble(gtx layout.Context) layout.Dimensions {
	maxR := gtx.Dp(unit.Dp(4.5))
	padX, padY := gtx.Dp(unit.Dp(18)), gtx.Dp(unit.Dp(15))
	w := 2*padX + 3*2*maxR + 2*(maxR*3/2)
	sz := image.Pt(w, 2*padY+2*maxR)
	borderedRRect(gtx, sz, unit.Dp(20), authCard, authCardBorder)
	defer op.Offset(image.Pt(padX, padY)).Push(gtx.Ops).Pop()
	dotsIndicator(gtx, authLink, maxR)
	return layout.Dimensions{Size: sz}
}

// reset wipes the open conversation and everything derived from it.
func (s *ChatScreen) reset() {
	s.mu.Lock()
	s.chatID, s.title, s.branch = "", "", ""
	s.messages = nil
	s.sourceBtns = nil
	s.searchStart = time.Time{}
	s.loadingHist = false
	s.asking = false
	s.streaming = false
	s.errMsg = ""
	s.scrollPending = false
	s.showUpgrade = false
	s.upgradeMsg = ""
	s.checkingOut = false
	s.checkoutErr = ""
	s.uiReset = true
	s.mu.Unlock()
}
