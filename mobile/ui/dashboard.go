package ui

import (
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"chatwithrepo/mobile/api"
)

// freeChatLimit mirrors FREE_CHAT_LIMIT in the web frontend's
// js/app.js: free-plan users may keep this many repository chats.
// Chats beyond it (by creation order, same as the list the backend
// returns) are locked until the user upgrades.
const freeChatLimit = 2

// DashboardScreen shows the user's existing chats and lets them start
// a new one by pointing at a GitHub owner/repo/branch.
type DashboardScreen struct {
	app *App

	mu       sync.Mutex
	loading  bool
	errMsg   string
	chats    []api.Chat
	chatBtns []widget.Clickable
	isPro    bool

	list      widget.List
	logoutBtn widget.Clickable
	newBtn    widget.Clickable

	// "New Repository" modal
	showModal bool
	owner     widget.Editor
	repo      widget.Editor
	branch    widget.Editor
	createBtn widget.Clickable
	cancelBtn widget.Clickable
	creating  bool
	formErr   string

	// "Upgrade to Pro" modal, shown instead of the create form (or
	// instead of opening a locked chat) once the free limit is hit.
	showUpgrade   bool
	upgradeMsg    string
	upgradeBtn    widget.Clickable
	upgradeCancel widget.Clickable
	checkingOut   bool
	checkoutErr   string
}

func newDashboardScreen(app *App) *DashboardScreen {
	s := &DashboardScreen{app: app}
	s.list.Axis = layout.Vertical
	s.owner.SingleLine = true
	s.repo.SingleLine = true
	s.branch.SingleLine = true
	s.branch.SetText("main")
	return s
}

// Reload fetches the chat list in the background.
func (s *DashboardScreen) Reload() {
	s.mu.Lock()
	s.loading = true
	s.errMsg = ""
	s.mu.Unlock()
	s.app.Window.Invalidate()

	go func() {
		chats, err := s.app.Client.ListChats()
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		// Plan status is best-effort: if it fails to load we treat
		// the account as free (the safer default — worst case an
		// already-Pro user briefly sees a lock that clears on retry,
		// rather than a free user bypassing the limit).
		status, statusErr := s.app.Client.PaymentStatus()

		s.mu.Lock()
		s.loading = false
		if err != nil {
			s.errMsg = err.Error()
		} else {
			s.chats = chats
			s.chatBtns = make([]widget.Clickable, len(chats))
			s.errMsg = ""
		}
		s.isPro = statusErr == nil && status.IsPro
		s.mu.Unlock()
		s.app.Window.Invalidate()
	}()
}

// locked reports whether the chat at index i is beyond the free plan
// limit and should be gated behind an upgrade rather than opened.
func (s *DashboardScreen) locked(i int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.isPro && i >= freeChatLimit
}

// attemptOpenModal is the tap handler for "New Repository". It mirrors
// the web frontend's soft check (js/app.js setupDashboard): a free
// user who already has freeChatLimit chats never sees the create
// form at all — they go straight to the upgrade prompt instead of
// filling in owner/repo just to be rejected on submit.
func (s *DashboardScreen) attemptOpenModal() {
	s.mu.Lock()
	atLimit := !s.isPro && len(s.chats) >= freeChatLimit
	s.mu.Unlock()

	if atLimit {
		s.openUpgradeModal("repo_limit")
		return
	}
	s.openModal()
}

func (s *DashboardScreen) openModal() {
	s.mu.Lock()
	s.showModal = true
	s.formErr = ""
	s.owner.SetText("")
	s.repo.SetText("")
	s.branch.SetText("main")
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

func (s *DashboardScreen) closeModal() {
	s.mu.Lock()
	s.showModal = false
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

// openUpgradeModal shows the "Upgrade to Pro" dialog in place of
// whatever else would normally happen (opening the create form, or
// opening a locked chat).
func (s *DashboardScreen) openUpgradeModal(reason string) {
	s.mu.Lock()
	s.showModal = false
	s.showUpgrade = true
	s.upgradeMsg = upgradeMessageFor(reason)
	s.checkoutErr = ""
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

func (s *DashboardScreen) closeUpgradeModal() {
	s.mu.Lock()
	s.showUpgrade = false
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

// startCheckout asks the backend for a hosted payment session (the
// same POST /payment/checkout the web frontend's "Upgrade to Pro"
// button calls) and opens the returned URL in the system browser.
func (s *DashboardScreen) startCheckout() {
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

func (s *DashboardScreen) submitCreate() {
	owner := strings.TrimSpace(s.owner.Text())
	repo := strings.TrimSpace(s.repo.Text())
	branch := strings.TrimSpace(s.branch.Text())
	if branch == "" {
		branch = "main"
	}
	if owner == "" || repo == "" {
		s.mu.Lock()
		s.formErr = "Owner and repository are required."
		s.mu.Unlock()
		s.app.Window.Invalidate()
		return
	}

	s.mu.Lock()
	s.creating = true
	s.formErr = ""
	s.mu.Unlock()
	s.app.Window.Invalidate()

	go func() {
		resp, err := s.app.Client.CreateChat(owner, repo, branch)
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		s.mu.Lock()
		s.creating = false
		if err != nil {
			s.formErr = err.Error()
			s.mu.Unlock()
			s.app.Window.Invalidate()
			return
		}
		s.mu.Unlock()

		// Hard-check safety net: even though attemptOpenModal already
		// screens this out, the backend is the source of truth (e.g.
		// another client created chats in the meantime). A 200 here
		// does not guarantee a chat actually got created — treat
		// UpgradeRequired as the real outcome instead of navigating
		// into a chat that was never indexed.
		if resp.UpgradeRequired {
			s.Reload()
			reason := resp.Reason
			if reason == "" {
				reason = "repo_limit"
			}
			s.openUpgradeModal(reason)
			return
		}

		s.mu.Lock()
		s.showModal = false
		s.mu.Unlock()

		title := owner + "/" + repo
		s.Reload()
		s.app.ShowChat(string(resp.ChatID), title, branch)
	}()
}

// dashboardSnapshot is everything Layout needs to draw one frame,
// read out under s.mu in one shot.
type dashboardSnapshot struct {
	loading  bool
	errMsg   string
	chats    []api.Chat
	chatBtns []widget.Clickable
	isPro    bool

	showModal bool
	creating  bool
	formErr   string

	showUpgrade bool
	upgradeMsg  string
	checkingOut bool
	checkoutErr string
}

func (s *DashboardScreen) snapshot() dashboardSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	// chatBtns is a copy of the slice header only (same backing array).
	// Reload() always assigns a brand-new backing array rather than
	// mutating in place, so reading through this header afterwards
	// without holding s.mu is safe and keeps each widget.Clickable's
	// identity stable across frames (required for click tracking).
	return dashboardSnapshot{
		loading:     s.loading,
		errMsg:      s.errMsg,
		chats:       s.chats,
		chatBtns:    s.chatBtns,
		isPro:       s.isPro,
		showModal:   s.showModal,
		creating:    s.creating,
		formErr:     s.formErr,
		showUpgrade: s.showUpgrade,
		upgradeMsg:  s.upgradeMsg,
		checkingOut: s.checkingOut,
		checkoutErr: s.checkoutErr,
	}
}

// Layout draws the Dashboard screen and processes its own clicks.
func (s *DashboardScreen) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	for s.logoutBtn.Clicked(gtx) {
		s.app.Logout()
	}
	for s.newBtn.Clicked(gtx) {
		s.attemptOpenModal()
	}

	snap := s.snapshot()
	loading, errMsg, chats, chatBtns, showModal, creating, formErr :=
		snap.loading, snap.errMsg, snap.chats, snap.chatBtns, snap.showModal, snap.creating, snap.formErr

	for i := range chatBtns {
		if i >= len(chats) {
			break
		}
		if chatBtns[i].Clicked(gtx) {
			if s.locked(i) {
				s.openUpgradeModal("repo_limit")
				break
			}
			c := chats[i]
			s.app.ShowChat(string(c.ChatID), c.Title, c.Branch)
			break
		}
	}

	for s.upgradeBtn.Clicked(gtx) {
		s.startCheckout()
	}
	for s.upgradeCancel.Clicked(gtx) {
		s.closeUpgradeModal()
	}

	for s.createBtn.Clicked(gtx) {
		s.submitCreate()
	}
	for s.cancelBtn.Clicked(gtx) {
		s.closeModal()
	}

	content := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.topBar(th)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(PrimaryButton(th, &s.newBtn, "New Repository", false)),
					layout.Rigid(spacer(16)),
					layout.Rigid(ErrorText(th, errMsg)),
					layout.Flexed(1, s.chatList(th, loading, chats, chatBtns, snap.isPro)),
				)
			})
		}),
	)

	if !showModal && !snap.showUpgrade {
		return content
	}

	overlay := s.modal(th, creating, formErr)
	if snap.showUpgrade {
		overlay = UpgradeModal(th, snap.upgradeMsg, &s.upgradeBtn, &s.upgradeCancel, snap.checkingOut, snap.checkoutErr)
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { return content }),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return fill(gtx, colorScrim())
		}),
		layout.Stacked(overlay),
	)
}

func (s *DashboardScreen) topBar(th *material.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return fill(gtx, colorSurface)
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							title := material.H6(th, "ChatWithRepo")
							title.Color = colorText
							return title.Layout(gtx)
						}),
						layout.Rigid(TextButton(th, &s.logoutBtn, "Logout")),
					)
				})
			}),
		)
	}
}

func (s *DashboardScreen) chatList(th *material.Theme, loading bool, chats []api.Chat, chatBtns []widget.Clickable, isPro bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if loading && len(chats) == 0 {
			lbl := material.Body2(th, "Loading chats…")
			lbl.Color = colorSubtleText
			return lbl.Layout(gtx)
		}
		if len(chats) == 0 {
			lbl := material.Body2(th, "No chats yet. Create one to index a repository.")
			lbl.Color = colorSubtleText
			return lbl.Layout(gtx)
		}
		return s.list.Layout(gtx, len(chats), func(gtx layout.Context, i int) layout.Dimensions {
			c := chats[i]
			// Beyond the free limit, a chat was never indexed (see
			// submitCreate's UpgradeRequired handling) — mark it
			// locked instead of letting it open into an empty,
			// unusable conversation.
			locked := !isPro && i >= freeChatLimit
			return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return chatBtns[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							roundedFill(gtx, gtx.Constraints.Min, unit.Dp(10), colorSurface)
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												lbl := material.Body1(th, c.Title)
												lbl.Color = colorText
												if locked {
													lbl.Color = colorSubtleText
												}
												return lbl.Layout(gtx)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												lbl := material.Caption(th, "branch: "+c.Branch)
												lbl.Color = colorSubtleText
												return lbl.Layout(gtx)
											}),
										)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !locked {
											return layout.Dimensions{}
										}
										return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											lbl := material.Caption(th, "🔒 Pro")
											lbl.Color = colorPrimary
											return lbl.Layout(gtx)
										})
									}),
								)
							})
						}),
					)
				})
			})
		})
	}
}

func (s *DashboardScreen) modal(th *material.Theme, creating bool, formErr string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Constraints.Max.X * 88 / 100
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					roundedFill(gtx, gtx.Constraints.Min, unit.Dp(14), colorSurface)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								title := material.H6(th, "New Repository")
								title.Color = colorText
								return title.Layout(gtx)
							}),
							layout.Rigid(spacer(16)),
							layout.Rigid(TextField(th, &s.owner, "Owner (e.g. octocat)")),
							layout.Rigid(spacer(10)),
							layout.Rigid(TextField(th, &s.repo, "Repository")),
							layout.Rigid(spacer(10)),
							layout.Rigid(TextField(th, &s.branch, "Branch (default: main)")),
							layout.Rigid(spacer(8)),
							layout.Rigid(ErrorText(th, formErr)),
							layout.Rigid(spacer(16)),
							layout.Rigid(PrimaryButton(th, &s.createBtn, "Create", creating)),
							layout.Rigid(spacer(8)),
							layout.Rigid(TextButton(th, &s.cancelBtn, "Cancel")),
						)
					})
				}),
			)
		})
	}
}
