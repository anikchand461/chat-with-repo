package ui

import (
	"strings"
	"sync"
	"time"

	"image"
	"image/color"

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
	email    string // from the existing /auth/me call; "" if unavailable

	list      widget.List
	logoutBtn widget.Clickable
	newBtn    widget.Clickable

	// Hamburger menu / bottom sheet (UI-goroutine only, no locking).
	uiReset     bool // consumed by Layout
	overlay     overlayAnim
	overlayKind int // 1 = create form, 2 = upgrade
	showMenu    bool
	menuProg    float32 // 0 closed .. 1 open, animated
	menuLast    time.Time
	menuBtn     widget.Clickable
	menuScrim   widget.Clickable
	menuSheet   widget.Clickable
	menuProfile widget.Clickable
	menuManual  widget.Clickable
	menuWeb     widget.Clickable
	menuGithub  widget.Clickable

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

	// "Setting up your chat" progress modal, shown whenever a chat isn't
	// actually ready yet - a brand new chat, one being re-indexed, or an
	// existing one whose status turns out not to be "ready" when tapped.
	// Mirrors the web frontend's dashboard progress panel (js/app.js
	// pollIndexStatus / #index-progress). indexSeq is bumped every time a
	// new wait starts, so a stale poll loop from a previous chat notices
	// and stops instead of clobbering a newer one's state.
	indexSeq        uint64
	showIndexing    bool
	indexChatID     string
	indexTitle      string
	indexBranch     string
	indexMsg        string
	indexErr        string
	indexDone       int
	indexTotal      int
	indexCloseBtn   widget.Clickable
	indexProfileBtn widget.Clickable
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
	gen := s.app.Generation()
	s.mu.Lock()
	s.loading = true
	s.errMsg = ""
	s.mu.Unlock()
	s.app.Window.Invalidate()

	go func() {
		chats, err := s.app.Client.ListChats()
		if !s.app.IsCurrent(gen) {
			return
		}
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		// Plan status is best-effort: if it fails to load we treat
		// the account as free (the safer default — worst case an
		// already-Pro user briefly sees a lock that clears on retry,
		// rather than a free user bypassing the limit).
		status, statusErr := s.app.Client.PaymentStatus()
		// Best-effort: only used for the greeting, so a failure just
		// hides it.
		email := ""
		if me, meErr := s.app.Client.Me(); meErr == nil {
			email = me.Email
		}

		s.mu.Lock()
		if !s.app.IsCurrent(gen) { // re-check: Me/PaymentStatus took time
			s.mu.Unlock()
			return
		}
		s.loading = false
		if err != nil {
			s.errMsg = err.Error()
		} else {
			s.chats = chats
			s.chatBtns = make([]widget.Clickable, len(chats))
			s.errMsg = ""
		}
		s.isPro = statusErr == nil && status.IsPro
		if email != "" {
			s.email = email
		}
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

	gen := s.app.Generation()
	s.mu.Lock()
	s.creating = true
	s.formErr = ""
	s.mu.Unlock()
	s.app.Window.Invalidate()

	go func() {
		resp, err := s.app.Client.CreateChat(owner, repo, branch)
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

		if !s.app.IsCurrent(gen) {
			return
		}
		s.mu.Lock()
		s.showModal = false
		s.mu.Unlock()

		title := owner + "/" + repo
		s.Reload()

		if resp.AlreadyIndexed {
			// Reopening a chat that's already indexed - nothing to wait for.
			s.app.ShowChat(string(resp.ChatID), title, branch)
			return
		}

		// New (or re-indexing) chat: the backend builds it in the
		// background. Show the same progress the web frontend does and
		// jump in automatically once it's ready.
		seq := s.beginIndexingWait(string(resp.ChatID), title, branch, "Setting up your chat…")
		s.pollIndexStatus(seq, string(resp.ChatID), title, branch)
	}()
}

// beginIndexingWait shows the "Setting up your chat" modal and returns
// the sequence number this wait owns. Bumping indexSeq invalidates any
// older in-flight poll loop (see pollIndexStatus), so opening a second
// chat while the first is still being checked can't have the two race
// and clobber each other's state.
func (s *DashboardScreen) beginIndexingWait(chatID, title, branch, initialMsg string) uint64 {
	s.mu.Lock()
	s.indexSeq++
	seq := s.indexSeq
	s.showModal = false
	s.showIndexing = true
	s.indexChatID = chatID
	s.indexTitle = title
	s.indexBranch = branch
	s.indexMsg = initialMsg
	s.indexErr = ""
	s.indexDone, s.indexTotal = 0, 0
	s.mu.Unlock()
	s.app.Window.Invalidate()
	return seq
}

func (s *DashboardScreen) closeIndexingWait() {
	s.mu.Lock()
	s.indexSeq++ // invalidates any poll loop still running for this wait
	s.showIndexing = false
	s.mu.Unlock()
	s.app.Window.Invalidate()
}

// pollIndexStatus polls GET /chat/{id}/index-status roughly every 1.5s
// (matching the web frontend) until it reports "ready" (opens the
// chat), "error" (shows it, with a Profile link when it's the GitHub
// rate-limit message), or ~10 minutes pass. seq must match s.indexSeq
// on every check - if it doesn't, a newer wait has taken over and this
// loop stops touching shared state.
func (s *DashboardScreen) pollIndexStatus(seq uint64, chatID, title, branch string) {
	gen := s.app.Generation()

	go func() {
		const maxAttempts = 400 // ~10 min at 1.5s/poll - generous for a large repo
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if attempt > 0 {
				time.Sleep(1500 * time.Millisecond)
			}
			if !s.app.IsCurrent(gen) {
				return
			}
			s.mu.Lock()
			stale := s.indexSeq != seq
			s.mu.Unlock()
			if stale {
				return
			}

			status, err := s.app.Client.IndexStatus(chatID)
			if !s.app.IsCurrent(gen) {
				return
			}
			if err == api.ErrUnauthorized {
				s.app.HandleUnauthorized()
				return
			}
			if err != nil {
				continue // transient network hiccup - keep polling
			}

			s.mu.Lock()
			if s.indexSeq != seq {
				s.mu.Unlock()
				return
			}
			switch status.Status {
			case "ready":
				s.showIndexing = false
				s.mu.Unlock()
				s.Reload()
				s.app.ShowChat(chatID, title, branch)
				return
			case "error":
				s.indexMsg = "Couldn't finish indexing"
				s.indexErr = status.Message
				s.mu.Unlock()
				s.app.Window.Invalidate()
				return
			default:
				s.indexMsg = status.Message
				s.indexDone, s.indexTotal = status.Done, status.Total
				s.mu.Unlock()
				s.app.Window.Invalidate()
			}
		}

		s.mu.Lock()
		if s.indexSeq == seq {
			s.indexMsg = "Couldn't finish indexing"
			s.indexErr = "This is taking longer than expected. It may still finish in the background - check back in a bit, or open the chat again."
		}
		s.mu.Unlock()
		s.app.Window.Invalidate()
	}()
}

// openChat is the tap handler for a chat in "Previous chats". It never
// assumes the chat is ready: it checks first, same as the web
// frontend's chat.html guard. If indexing had failed, opening the chat
// counts as the deliberate action that retries it (via the same
// /chat/create path the create form uses) - a passive status check
// alone must never retry on its own, or a permanent failure like "no
// GitHub token configured" would silently retry forever.
func (s *DashboardScreen) openChat(c api.Chat) {
	gen := s.app.Generation()
	chatID, title, branch := string(c.ChatID), c.Title, c.Branch
	seq := s.beginIndexingWait(chatID, title, branch, "Checking status…")

	go func() {
		status, err := s.app.Client.IndexStatus(chatID)
		if !s.app.IsCurrent(gen) {
			return
		}
		if err == api.ErrUnauthorized {
			s.app.HandleUnauthorized()
			return
		}
		if err == nil && status.Status == "ready" {
			s.mu.Lock()
			if s.indexSeq == seq {
				s.showIndexing = false
			}
			s.mu.Unlock()
			s.app.ShowChat(chatID, title, branch)
			return
		}
		if err == nil && status.Status == "error" && c.Owner != "" && c.Repo != "" {
			_, _ = s.app.Client.CreateChat(c.Owner, c.Repo, branch)
		}

		s.mu.Lock()
		stale := s.indexSeq != seq
		s.mu.Unlock()
		if stale {
			return
		}
		s.pollIndexStatus(seq, chatID, title, branch)
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
	email    string

	showModal bool
	creating  bool
	formErr   string

	showUpgrade bool
	upgradeMsg  string
	checkingOut bool
	checkoutErr string

	showIndexing bool
	indexMsg     string
	indexErr     string
	indexDone    int
	indexTotal   int
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
		loading:      s.loading,
		errMsg:       s.errMsg,
		chats:        s.chats,
		chatBtns:     s.chatBtns,
		isPro:        s.isPro,
		email:        s.email,
		showModal:    s.showModal,
		creating:     s.creating,
		formErr:      s.formErr,
		showUpgrade:  s.showUpgrade,
		upgradeMsg:   s.upgradeMsg,
		checkingOut:  s.checkingOut,
		checkoutErr:  s.checkoutErr,
		showIndexing: s.showIndexing,
		indexMsg:     s.indexMsg,
		indexErr:     s.indexErr,
		indexDone:    s.indexDone,
		indexTotal:   s.indexTotal,
	}
}

// Layout draws the Dashboard screen and processes its own clicks.
func (s *DashboardScreen) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	authS = authScale(gtx)

	s.mu.Lock()
	doReset := s.uiReset
	s.uiReset = false
	s.mu.Unlock()
	if doReset {
		s.showMenu, s.menuProg = false, 0
		s.overlay = overlayAnim{}
		s.overlayKind = 0
		s.owner.SetText("")
		s.repo.SetText("")
		s.branch.SetText("main")
		s.list = widget.List{List: layout.List{Axis: layout.Vertical}}
	}

	for s.menuBtn.Clicked(gtx) {
		s.showMenu = true
	}
	for s.menuScrim.Clicked(gtx) {
		s.showMenu = false
	}
	for s.menuSheet.Clicked(gtx) {
	}
	for s.menuProfile.Clicked(gtx) {
		s.showMenu = false
		s.app.ShowProfile()
	}
	for s.menuManual.Clicked(gtx) {
		s.showMenu = false
		go func() { _ = openURL(manualURL) }()
	}
	for s.menuWeb.Clicked(gtx) {
		s.showMenu = false
		go func() { _ = openURL(webURL) }()
	}
	for s.menuGithub.Clicked(gtx) {
		s.showMenu = false
		go func() { _ = openURL(repoURL) }()
	}
	for s.logoutBtn.Clicked(gtx) {
		s.showMenu = false
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
			s.openChat(chats[i])
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
	for s.indexCloseBtn.Clicked(gtx) {
		s.closeIndexingWait()
	}
	for s.indexProfileBtn.Clicked(gtx) {
		s.closeIndexingWait()
		s.app.ShowProfile()
	}

	// Android Back closes the topmost overlay; with nothing open no filter
	// is registered, so Back keeps its default (leave the app).
	if snap.showUpgrade || showModal || snap.showIndexing || s.showMenu {
		for {
			ev, ok := gtx.Event(key.Filter{Name: key.NameBack})
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press {
				switch {
				case snap.showUpgrade:
					s.closeUpgradeModal()
				case showModal:
					s.closeModal()
				case snap.showIndexing:
					s.closeIndexingWait()
				default:
					s.showMenu = false
				}
			}
		}
	}

	// Advance the drawer animation only after this frame's clicks/Back have
	// updated showMenu; stepping earlier meant the frame that opened the
	// drawer never requested a follow-up frame, which only worked on
	// desktop because mouse movement keeps redrawing.
	s.stepMenuAnimation(gtx)

	authBackground(gtx)
	gtx.Constraints.Min = gtx.Constraints.Max
	content := withInsets(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(s.header(th)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(4)}.Layout(gtx,
					s.body(th, loading, errMsg, chats, chatBtns, snap.isPro))
			}),
		)
	})

	open := snap.showUpgrade || showModal || snap.showIndexing
	s.overlay.step(gtx, open)
	switch {
	case snap.showUpgrade:
		s.overlayKind = 2
	case snap.showIndexing:
		s.overlayKind = 3
	case showModal:
		s.overlayKind = 1
	}
	if s.overlay.visible() {
		dialog := s.modal(th, creating, formErr)
		switch s.overlayKind {
		case 2:
			dialog = UpgradeModal(th, snap.upgradeMsg, &s.upgradeBtn, &s.upgradeCancel, snap.checkingOut, snap.checkoutErr)
		case 3:
			dialog = IndexingModal(th, snap.indexMsg, snap.indexDone, snap.indexTotal, snap.indexErr, &s.indexCloseBtn, &s.indexProfileBtn)
		}
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return content }),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return s.overlay.draw(gtx, dialog) }),
		)
	}
	if s.showMenu || s.menuProg > 0 {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions { return content }),
			layout.Expanded(s.drawer(th)),
		)
	}
	return content
}

// header is the compact top row: logo on the left, hamburger on the right.
func (s *DashboardScreen) header(th *material.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(10), Bottom: unit.Dp(6)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if logoOp == nil {
							return layout.Dimensions{}
						}
						return widget.Image{Src: *logoOp, Fit: widget.Contain, Position: layout.W}.Layout(
							constrainHeight(gtx, unit.Dp(40)))
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
					}),
					layout.Rigid(s.menuButton),
				)
			})
	}
}

// menuButton is the round hamburger on the right of the header.
func (s *DashboardScreen) menuButton(gtx layout.Context) layout.Dimensions {
	d := gtx.Dp(unit.Dp(44))
	gtx.Constraints.Min, gtx.Constraints.Max = image.Pt(d, d), image.Pt(d, d)
	return s.menuBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		borderedRRect(gtx, image.Pt(d, d), unit.Dp(22), authIconBg, authFieldBorder)
		pressOverlay(gtx, &s.menuBtn, image.Pt(d, d), d/2, pressLight)
		w, h := d*40/100, gtx.Dp(unit.Dp(2))
		gap := gtx.Dp(unit.Dp(5))
		x := (d - w) / 2
		y := (d - (3*h + 2*gap)) / 2
		for i := 0; i < 3; i++ {
			r := image.Rect(x, y+i*(h+gap), x+w, y+i*(h+gap)+h)
			paint.FillShape(gtx.Ops, authIconFg, clip.UniformRRect(r, h/2).Op(gtx.Ops))
		}
		return layout.Dimensions{Size: image.Pt(d, d)}
	})
}

// stepMenuAnimation eases menuProg toward open/closed and keeps
// requesting frames until it settles.
func (s *DashboardScreen) stepMenuAnimation(gtx layout.Context) {
	target := float32(0)
	if s.showMenu {
		target = 1
	}
	dt := float32(gtx.Now.Sub(s.menuLast).Seconds())
	s.menuLast = gtx.Now
	if dt <= 0 || dt > 0.05 {
		dt = 1.0 / 60
	}
	if s.menuProg == target {
		return
	}
	step := dt / 0.22 // ~220ms
	if s.menuProg < target {
		s.menuProg += step
		if s.menuProg > target {
			s.menuProg = target
		}
	} else {
		s.menuProg -= step
		if s.menuProg < target {
			s.menuProg = target
		}
	}
	if s.menuProg != target {
		gtx.Execute(op.InvalidateCmd{})
	}
}

// centerY lays w out at its natural height and centres it vertically in a
// row of height h.
func centerY(gtx layout.Context, h int, w layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = 0, h
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer op.Offset(image.Pt(0, (h-dims.Size.Y)/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(dims.Size.X, h)}
}

// drawer is the dimmed scrim plus the menu panel sliding in from the right.
func (s *DashboardScreen) drawer(th *material.Theme) layout.Widget {
	// row draws a drawer button: icon (Profile/GitHub/Logout - see
	// mobile/assets/{profile,github,logout}.png) beside its label.
	row := func(btn *widget.Clickable, icon *paint.ImageOp, label string, col color.NRGBA) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				h := gtx.Dp(unit.Dp(52))
				return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					sz := image.Pt(gtx.Constraints.Max.X, h)
					borderedRRect(gtx, sz, unit.Dp(16), authField, authFieldBorder)
					pressOverlay(gtx, btn, sz, gtx.Dp(unit.Dp(16)), pressLight)
					return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						centerY(gtx, h, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									d := gtx.Dp(unit.Dp(20))
									if icon != nil {
										g := gtx
										g.Constraints.Min, g.Constraints.Max = image.Pt(d, d), image.Pt(d, d)
										widget.Image{Src: *icon, Fit: widget.Contain, Position: layout.Center}.Layout(g)
									}
									return layout.Dimensions{Size: image.Pt(d+gtx.Dp(unit.Dp(10)), d)}
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Label(th, unit.Sp(15), label)
									l.Color = col
									l.Font.Weight = font.Bold
									return l.Layout(gtx)
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
		e := s.menuProg * s.menuProg * (3 - 2*s.menuProg) // smoothstep

		dw := gtx.Dp(unit.Dp(220))
		if lim := size.X * 72 / 100; dw > lim {
			dw = lim
		}

		// Scrim.
		scrim := colorScrim()
		scrim.A = uint8(float32(scrim.A) * e)
		s.menuScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return fill(gtx, scrim)
		})

		// Panel.
		x := size.X - int(float32(dw)*e)
		defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
		pg := gtx
		pg.Constraints.Min, pg.Constraints.Max = image.Pt(dw, size.Y), image.Pt(dw, size.Y)
		return s.menuSheet.Layout(pg, func(gtx layout.Context) layout.Dimensions {
			r := gtx.Dp(unit.Dp(28))
			paint.FillShape(gtx.Ops, authCardBorder, clip.RRect{Rect: image.Rectangle{Max: gtx.Constraints.Max}, NW: r, SW: r}.Op(gtx.Ops))
			paint.FillShape(gtx.Ops, authCard, clip.RRect{Rect: image.Rect(gtx.Dp(unit.Dp(1)), 0, dw, size.Y), NW: r, SW: r}.Op(gtx.Ops))
			layout.Inset{Left: unit.Dp(20), Right: unit.Dp(20), Top: unit.Dp(72) + sysInsets.Top}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(greeting(th, s.snapshot().email)),
					layout.Rigid(row(&s.menuProfile, profileOp, "Profile", authTitle)),
					layout.Rigid(row(&s.menuManual, manualOp, "Manual", authTitle)),
					layout.Rigid(row(&s.menuWeb, webOp, "Web", authTitle)),
					layout.Rigid(row(&s.menuGithub, githubOp, "GitHub", authTitle)),
					layout.Rigid(row(&s.logoutBtn, logoutOp, "Logout", colorError)),
				)
			})
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	}
}

// body is the scrollable dashboard content: hero + Create Chat, then
// the "Previous chats" section.
func (s *DashboardScreen) body(th *material.Theme, loading bool, errMsg string, chats []api.Chat, chatBtns []widget.Clickable, isPro bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		n := 3
		if len(chats) > 0 {
			n = 2 + len(chats)
		}
		return s.list.Layout(gtx, n, func(gtx layout.Context, i int) layout.Dimensions {
			switch {
			case i == 0:
				return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(th, authSp(26), "Your codebase,")
							l.Color = authTitle
							l.Font.Weight = font.Bold
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(th, authSp(26), "in conversation.")
							l.Color = color.NRGBA{R: 0x3d, G: 0xdc, B: 0x97, A: 0xff}
							l.Font.Weight = font.Bold
							return l.Layout(gtx)
						}),
						layout.Rigid(authGap(8)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(th, authSp(15), "Each chat indexes one repository. Ask questions, get answers grounded in real code.")
							l.LineHeightScale = 1.25
							l.Color = authBody
							return l.Layout(gtx)
						}),
						layout.Rigid(authGap(18)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							w := gtx.Dp(unit.Dp(240))
							if w > gtx.Constraints.Max.X {
								w = gtx.Constraints.Max.X
							}
							gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
							return layout.Center.Layout(gtx, AuthButton(th, &s.newBtn, "Create Chat", false))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if errMsg == "" {
								return layout.Dimensions{}
							}
							return layout.Inset{Top: unit.Dp(10)}.Layout(gtx, ErrorText(th, errMsg))
						}),
					)
				})
			case i == 1:
				return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(th, authSp(20), "Previous chats")
							l.Color = authTitle
							l.Font.Weight = font.Bold
							return l.Layout(gtx)
						}),
						layout.Rigid(authGap(4)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := material.Label(th, authSp(14), "Jump back into a repository you've already indexed.")
							l.LineHeightScale = 1.25
							l.Color = authBody
							return l.Layout(gtx)
						}),
					)
				})
			case len(chats) == 0:
				msg := "No chats yet. Create one to index a repository."
				if loading {
					msg = "Loading chats…"
				}
				return layout.Inset{Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return authCardWidget(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, authSp(15), msg)
						l.Color = authBody
						return l.Layout(gtx)
					})(gtx)
				})
			default:
				ci := i - 2
				return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, s.chatCard(th, ci, chats[ci], &chatBtns[ci], !isPro && ci >= freeChatLimit))
			}
		})
	}
}

// chatCard is one previous chat as a tappable rounded card. Chats beyond
// the free limit were never indexed, so they show a Pro pill and open the
// upgrade prompt instead of an empty conversation.
func (s *DashboardScreen) chatCard(th *material.Theme, _ int, c api.Chat, btn *widget.Clickable, locked bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(20), authCard, authCardBorder)
					pressOverlay(gtx, btn, gtx.Constraints.Min, gtx.Dp(unit.Dp(20)), pressLight)
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Left: unit.Dp(18), Right: unit.Dp(14), Top: unit.Dp(16), Bottom: unit.Dp(16)}.Layout(gtx,
						func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(th, unit.Sp(17), c.Title)
											l.Color = authTitle
											if locked {
												l.Color = authBody
											}
											l.Font.Weight = font.Bold
											return l.Layout(gtx)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := material.Label(th, unit.Sp(13), "branch: "+c.Branch)
											l.Color = authHint
											return l.Layout(gtx)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									txt, col := "›", authLink
									if locked {
										txt, col = "Pro", authLink
									}
									l := material.Label(th, unit.Sp(22), txt)
									if locked {
										l.TextSize = unit.Sp(13)
										l.Font.Weight = font.Bold
									}
									l.Color = col
									return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, l.Layout)
								}),
							)
						})
				}),
			)
		})
	}
}

func (s *DashboardScreen) modal(th *material.Theme, creating bool, formErr string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			maxW := gtx.Constraints.Max.X - 2*gtx.Dp(unit.Dp(20))
			if lim := gtx.Dp(unit.Dp(440)); maxW > lim {
				maxW = lim
			}
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = maxW, maxW
			return authCardWidget(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, authSp(20), "Create a repository chat")
						l.Color = authTitle
						l.Font.Weight = font.Bold
						return l.Layout(gtx)
					}),
					layout.Rigid(authGap(14)),
					layout.Rigid(AuthField(th, &s.owner, "GitHub username", "username")),
					layout.Rigid(authGap(10)),
					layout.Rigid(AuthField(th, &s.repo, "Repository name", "repo")),
					layout.Rigid(authGap(10)),
					layout.Rigid(AuthField(th, &s.branch, "Branch", "main")),
					layout.Rigid(authGap(8)),
					layout.Rigid(ErrorText(th, formErr)),
					layout.Rigid(authGap(12)),
					layout.Rigid(AuthButton(th, &s.createBtn, "Create", creating)),
					layout.Rigid(authGap(10)),
					layout.Rigid(AuthLink(th, &s.cancelBtn, "Cancel")),
				)
			})(gtx)
		})
	}
}

var greetingYellow = color.NRGBA{R: 0xfa, G: 0xcc, B: 0x15, A: 0xff}

// nameFromEmail extracts the display name shown by greeting() and
// ProfileScreen's title (the part of the email before "@", first
// letter capitalised). Returns "" when the email is unknown.
func nameFromEmail(email string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(email), "@")
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	r := []rune(name)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// greeting renders "Hi <Name>" from the logged-in user's email. It
// draws nothing when the email is unknown.
func greeting(th *material.Theme, email string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		name := nameFromEmail(email)
		if name == "" {
			return layout.Dimensions{}
		}
		return layout.Inset{Left: unit.Dp(4), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			part := func(txt string, col color.NRGBA, truncate bool) layout.Widget {
				return func(gtx layout.Context) layout.Dimensions {
					l := material.Label(th, unit.Sp(20), txt)
					l.Color = col
					l.Font.Weight = font.Bold
					l.MaxLines = 1
					if truncate {
						l.Truncator = "…"
					}
					return l.Layout(gtx)
				}
			}
			gap := layout.Spacer{Width: unit.Dp(6)}.Layout
			gtx.Constraints.Min.X = 0
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(part("Hi", authTitle, false)),
				layout.Rigid(gap),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// Leave room for the emoji so a long name truncates
					// instead of pushing it out of the menu.
					gtx.Constraints.Max.X -= gtx.Dp(unit.Dp(44))
					return part(name, greetingYellow, true)(gtx)
				}),
				layout.Rigid(gap),
				layout.Rigid(part("👋", authTitle, false)),
			)
		})
	}
}

// reset wipes everything account-specific. The dashboard starts in the
// loading state so nothing (not even "No chats yet") flashes before the
// new account's data arrives. UI-goroutine-only widgets are reset by Layout.
func (s *DashboardScreen) reset() {
	s.mu.Lock()
	s.loading = true
	s.errMsg = ""
	s.chats = nil
	s.chatBtns = nil
	s.isPro = false
	s.email = ""
	s.showModal = false
	s.creating = false
	s.formErr = ""
	s.showUpgrade = false
	s.upgradeMsg = ""
	s.checkingOut = false
	s.checkoutErr = ""
	s.indexSeq++ // invalidates any poll loop still running for the old account
	s.showIndexing = false
	s.indexMsg = ""
	s.indexErr = ""
	s.indexDone, s.indexTotal = 0, 0
	s.uiReset = true
	s.mu.Unlock()
}
