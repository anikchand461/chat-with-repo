package ui

import (
	"image"
	"strings"
	"sync"

	"gioui.org/layout"
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
	backBtn  widget.Clickable

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
			s.scrollPending = true
			reason := resp.Reason
			if reason == "" {
				reason = "daily_questions"
			}
			s.showUpgrade = true
			s.upgradeMsg = upgradeMessageFor(reason)
			s.checkoutErr = ""
		default:
			s.messages = append(s.messages, api.Message{Role: "assistant", Content: resp.Answer})
			s.scrollPending = true
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
	for s.backBtn.Clicked(gtx) {
		s.app.ShowDashboard()
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
		s.list.ScrollToEnd = true
	}

	// Once a daily-question limit hits, further asking is blocked
	// until the user upgrades — don't let them keep hammering a send
	// button that will only come back with the same limit message.
	asking := snap.asking || snap.showUpgrade

	content := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.topBar(th, snap.title, snap.branch)),
		layout.Flexed(1, s.messageList(th, snap.messages, snap.loadingHist)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ErrorText(th, snap.errMsg)(gtx)
		}),
		layout.Rigid(s.inputRow(th, asking)),
	)

	if !snap.showUpgrade {
		return content
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { return content }),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return fill(gtx, colorScrim())
		}),
		layout.Stacked(UpgradeModal(th, snap.upgradeMsg, &s.upgradeBtn, &s.upgradeCancel, snap.checkingOut, snap.checkoutErr)),
	)
}

func (s *ChatScreen) topBar(th *material.Theme, title, branch string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return fill(gtx, colorSurface)
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(TextButton(th, &s.backBtn, "‹ Back")),
						layout.Rigid(spacerX(8)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									t := material.Body1(th, title)
									t.Color = colorText
									return t.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									b := material.Caption(th, "branch: "+branch)
									b.Color = colorSubtleText
									return b.Layout(gtx)
								}),
							)
						}),
					)
				})
			}),
		)
	}
}

func (s *ChatScreen) messageList(th *material.Theme, messages []api.Message, loadingHist bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if loadingHist && len(messages) == 0 {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, "Loading conversation…")
				lbl.Color = colorSubtleText
				return lbl.Layout(gtx)
			})
		}
		if len(messages) == 0 {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, "Ask anything about this repository — architecture, files, functions, or bugs.")
				lbl.Color = colorSubtleText
				return lbl.Layout(gtx)
			})
		}
		return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return s.list.Layout(gtx, len(messages), func(gtx layout.Context, i int) layout.Dimensions {
				m := messages[i]
				return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, Bubble(th, m.Role, m.Content))
			})
		})
	}
}

func (s *ChatScreen) inputRow(th *material.Theme, asking bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return fill(gtx, colorSurface)
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
						layout.Flexed(1, TextField(th, &s.question, "Ask about this repo…")),
						layout.Rigid(spacerX(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							w := gtx.Dp(unit.Dp(84))
							gtx.Constraints.Max.X = w
							gtx.Constraints.Min.X = w
							return PrimaryButton(th, &s.sendBtn, "Send", asking)(gtx)
						}),
					)
				})
			}),
		)
	}
}

// spacerX returns a fixed-width blank widget for horizontal gaps.
func spacerX(dp int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(dp))}}
	}
}
