package ui

import (
	"image"
	"io"
	"math"
	"strings"
	"time"
	"unicode"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Gio draws its own text fields, so on Android they miss what native fields
// have: long-press to select a word, selection handles, the Cut/Copy/Paste
// menu, and a backspace that keeps deleting while held. layoutEditor adds
// them; every text field in the app is drawn through it.

// longPressDelay is how long a finger must rest to count as a long press
// (Android's own timeout is 400-500 ms).
const longPressDelay = 450 * time.Millisecond

// maxSharedRunes caps the text handed to the keyboard (see shareText).
const maxSharedRunes = 4000

// editMenus holds each field's selection state, keyed by its editor. Only
// the UI goroutine touches it.
var editMenus = map[*widget.Editor]*editMenu{}

type editMenu struct {
	press   longPress
	menu    bool         // opened by a long press, even with nothing selected
	handles [2]selHandle // start, end
	dismiss int          // tag for the tap-elsewhere area

	paste, copy, cut, selectAll widget.Clickable
}

// selHandle is one draggable end of the selection.
type selHandle struct {
	dragging bool
	off      f32.Point // finger position minus the point in the text it moves
}

// layoutEditor lays out ed like ed.Layout, plus text selection, the
// Cut/Copy/Paste menu and the held-backspace fix.
func layoutEditor(gtx layout.Context, th *material.Theme, ed material.EditorStyle) layout.Dimensions {
	e := ed.Editor
	m := editMenus[e]
	if m == nil {
		m = &editMenu{}
		editMenus[e] = m
	}

	dims := ed.Layout(gtx)
	if !e.ReadOnly {
		shareText(gtx, e)
	}
	m.layout(gtx, th, e, dims.Size)
	return dims
}

// shareText gives the keyboard the field's whole text. Gio otherwise shares
// it only when the keyboard asks, a frame late, so Gboard sees an empty
// field and deletes with plain key events - and Gio restarts the keyboard
// after every such edit, which stops a held backspace after one character.
// With the text visible, Gboard deletes through the input connection, which
// Gio applies without a restart.
func shareText(gtx layout.Context, e *widget.Editor) {
	if !gtx.Focused(e) {
		return
	}
	n := e.Len()
	if n > maxSharedRunes {
		return
	}
	gtx.Execute(key.SnippetCmd{Tag: e, Snippet: key.Snippet{
		Range: key.Range{Start: 0, End: n},
		Text:  e.Text(),
	}})
}

func (m *editMenu) layout(gtx layout.Context, th *material.Theme, e *widget.Editor, size image.Point) {
	masked := e.Mask != 0 // passwords and tokens: no selecting or copying, like Android

	// Long press: select the word under the finger and show the menu.
	pos, pressed := m.press.update(gtx, size)
	if pressed {
		gtx.Execute(key.FocusCmd{Tag: e})
		m.menu = true
		if !masked {
			at := runeAt(e, pos)
			start, end := wordAt([]rune(e.Text()), at)
			e.SetCaret(end, start)
		}
	}

	// Focus requested above takes effect after this frame.
	if !gtx.Focused(e) && !pressed {
		m.menu = false
		m.handles = [2]selHandle{}
		return
	}

	start, end := e.Selection()
	lo, hi := min(start, end), max(start, end)
	selected := lo != hi && !masked

	// Menu actions.
	switch {
	case m.paste.Clicked(gtx):
		gtx.Execute(clipboard.ReadCmd{Tag: e}) // the editor inserts it, replacing the selection
		m.menu = false
	case selected && m.copy.Clicked(gtx):
		writeClipboard(gtx, e.SelectedText())
		e.SetCaret(hi, hi)
		m.menu = false
	case selected && m.cut.Clicked(gtx):
		writeClipboard(gtx, e.SelectedText())
		e.Insert("")
		m.menu = false
	case m.selectAll.Clicked(gtx):
		e.SetCaret(e.Len(), 0)
		m.menu = true
	}

	// Tapping anywhere else ends the selection. The tap still reaches
	// whatever is there (a button, the field itself).
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &m.dismiss, Kinds: pointer.Press})
		if !ok {
			break
		}
		if _, ok := ev.(pointer.Event); ok {
			m.menu = false
			_, end := e.Selection()
			e.SetCaret(end, end)
		}
	}

	start, end = e.Selection()
	lo, hi = min(start, end), max(start, end)
	selected = lo != hi && !masked

	// Handles: dragging one moves that end of the selection.
	var anchors [2]image.Point
	var visible [2]bool
	if selected {
		anchors[0], visible[0] = boundaryPoint(e, lo, false)
		anchors[1], visible[1] = boundaryPoint(e, hi, true)
		for i := range m.handles {
			fixed := hi
			if i == 1 {
				fixed = lo
			}
			if at, ok := m.handles[i].update(gtx, e, anchors[i]); ok {
				e.SetCaret(at, fixed)
			}
		}
	} else {
		m.handles = [2]selHandle{}
	}
	dragging := m.handles[0].dragging || m.handles[1].dragging

	if !selected && !m.menu {
		return
	}

	var items []menuItem
	if selected && !e.ReadOnly {
		items = append(items, menuItem{&m.cut, "Cut"})
	}
	if selected {
		items = append(items, menuItem{&m.copy, "Copy"})
	}
	if !e.ReadOnly {
		items = append(items, menuItem{&m.paste, "Paste"})
	}
	if e.Len() > 0 && !(lo == 0 && hi == e.Len()) {
		items = append(items, menuItem{&m.selectAll, "Select all"})
	}

	rec := op.Record(gtx.Ops)

	pass := pointer.PassOp{}.Push(gtx.Ops)
	outside := clip.Rect{Min: image.Pt(-1<<15, -1<<15), Max: image.Pt(1<<15, 1<<15)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &m.dismiss)
	outside.Pop()
	pass.Pop()

	if !dragging && len(items) > 0 {
		// Above the selection (or the field), like Android.
		mgtx := gtx
		mgtx.Constraints.Min = image.Point{}
		mrec := op.Record(gtx.Ops)
		menuDims := menuBar(mgtx, th, items...)
		menu := mrec.Stop()

		x, top := 0, 0
		if selected {
			if r := e.Regions(lo, hi, nil); len(r) > 0 {
				x, top = r[0].Bounds.Min.X, r[0].Bounds.Min.Y
			}
		}
		x = max(0, min(x, size.X-menuDims.Size.X))
		off := op.Offset(image.Pt(x, top-menuDims.Size.Y-gtx.Dp(unit.Dp(8)))).Push(gtx.Ops)
		menu.Add(gtx.Ops)
		off.Pop()
	}

	for i := range m.handles {
		if selected && visible[i] {
			m.handles[i].draw(gtx, anchors[i], i == 0)
		}
	}

	op.Defer(gtx.Ops, rec.Stop())
}

// handleRadius is the size of a selection handle's round part.
const handleRadius = unit.Dp(10)

// update handles drags of a handle anchored at p (editor coordinates) and
// returns the rune offset it was dragged to.
func (h *selHandle) update(gtx layout.Context, e *widget.Editor, p image.Point) (int, bool) {
	moved, at := false, 0
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: h, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			h.dragging = true
			// Aim at the middle of the line above the handle, not the
			// finger, so the text being selected stays visible.
			lineMid := float32(gtx.Dp(unit.Dp(10)))
			h.off = pe.Position.Sub(f32.Pt(float32(p.X), float32(p.Y)-lineMid))
			gtx.Execute(pointer.GrabCmd{Tag: h, ID: pe.PointerID})
		case pointer.Drag:
			if h.dragging {
				target := pe.Position.Sub(h.off)
				at, moved = runeAt(e, image.Pt(int(target.X), int(target.Y))), true
			}
		case pointer.Release, pointer.Cancel:
			h.dragging = false
		}
	}
	return at, moved
}

// draw paints the handle hanging below p and registers its touch area. The
// start handle points up-right, the end handle up-left, like Android's.
func (h *selHandle) draw(gtx layout.Context, p image.Point, start bool) {
	r := gtx.Dp(handleRadius)
	cx := p.X + r
	square := image.Rect(p.X, p.Y, p.X+r, p.Y+r)
	if start {
		cx = p.X - r
		square = image.Rect(p.X-r, p.Y, p.X, p.Y+r)
	}
	center := image.Pt(cx, p.Y+r)

	paint.FillShape(gtx.Ops, authAccent, clip.Ellipse{Min: center.Sub(image.Pt(r, r)), Max: center.Add(image.Pt(r, r))}.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, authAccent, clip.Rect(square).Op())

	area := clip.Rect{Min: center.Sub(image.Pt(2*r, 2*r)), Max: center.Add(image.Pt(2*r, 2*r))}.Push(gtx.Ops)
	event.Op(gtx.Ops, h)
	area.Pop()
}

// boundaryPoint returns the bottom of the boundary before rune i, in editor
// coordinates; end picks the right edge of rune i-1 (where a selection ends).
func boundaryPoint(e *widget.Editor, i int, end bool) (image.Point, bool) {
	if end && i > 0 || i >= e.Len() && i > 0 {
		r := e.Regions(i-1, i, nil)
		if len(r) == 0 {
			return image.Point{}, false
		}
		b := r[len(r)-1].Bounds
		return image.Pt(b.Max.X, b.Max.Y), true
	}
	r := e.Regions(i, i+1, nil)
	if len(r) == 0 {
		return image.Point{}, false
	}
	b := r[0].Bounds
	return image.Pt(b.Min.X, b.Max.Y), true
}

// runeAt returns the boundary (rune offset) in e closest to p, in editor
// coordinates.
func runeAt(e *widget.Editor, p image.Point) int {
	n := e.Len()
	best, bestScore := -1, math.MaxInt
	var regs []widget.Region
	for i := 0; i < n; i++ {
		regs = e.Regions(i, i+1, regs[:0])
		for _, r := range regs {
			b := r.Bounds
			dy := 0
			if p.Y < b.Min.Y {
				dy = b.Min.Y - p.Y
			} else if p.Y >= b.Max.Y {
				dy = p.Y - b.Max.Y + 1
			}
			pos, dx := i, abs(p.X-b.Min.X)
			if p.X >= (b.Min.X+b.Max.X)/2 {
				pos, dx = i+1, abs(p.X-b.Max.X)
			}
			if score := dy*100000 + dx; score < bestScore {
				best, bestScore = pos, score
			}
		}
	}
	if best < 0 {
		return n
	}
	return best
}

// wordAt returns the word touching boundary i, or an empty range at i.
func wordAt(text []rune, i int) (int, int) {
	isWord := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
	}
	if !(i < len(text) && isWord(text[i]) || i > 0 && isWord(text[i-1])) {
		return i, i
	}
	start, end := i, i
	for start > 0 && isWord(text[start-1]) {
		start--
	}
	for end < len(text) && isWord(text[end]) {
		end++
	}
	return start, end
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type menuItem struct {
	btn   *widget.Clickable
	label string
}

// menuBar draws a menu: a dark rounded bar of text buttons.
func menuBar(gtx layout.Context, th *material.Theme, items ...menuItem) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(items))
	for _, it := range items {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return it.btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Top: unit.Dp(10), Bottom: unit.Dp(10)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, authSp(14), it.label)
						l.Color = authTitle
						return l.Layout(gtx)
					})
			})
		}))
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(12), authCard, authCardBorder)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
		}),
	)
}

func writeClipboard(gtx layout.Context, s string) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(s))})
}

// longPress detects a finger (or mouse button) held still on an area. It
// lets presses through, so the widget underneath still gets its taps and
// scrolling - until the long press fires: then it takes the pointer, so
// lifting the finger isn't also a tap (which would clear the selection).
type longPress struct {
	down bool
	id   pointer.ID
	at   time.Time
	pos  f32.Point
}

// update registers the area (size, at the current offset) and reports
// where a long press completed this frame.
func (lp *longPress) update(gtx layout.Context, size image.Point) (image.Point, bool) {
	slop := float32(gtx.Dp(unit.Dp(10)))
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: lp, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			lp.down, lp.id, lp.at, lp.pos = true, e.PointerID, gtx.Now, e.Position
		case pointer.Drag:
			if d := e.Position.Sub(lp.pos); d.X*d.X+d.Y*d.Y > slop*slop {
				lp.down = false // scrolling, not a long press
			}
		case pointer.Release, pointer.Cancel:
			lp.down = false
		}
	}

	var at image.Point
	fired := false
	if lp.down {
		if due := lp.at.Add(longPressDelay); !gtx.Now.Before(due) {
			lp.down = false
			fired = true
			at = image.Pt(int(lp.pos.X), int(lp.pos.Y))
			gtx.Execute(pointer.GrabCmd{Tag: lp, ID: lp.id})
		} else {
			gtx.Execute(op.InvalidateCmd{At: due})
		}
	}

	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, lp)
	area.Pop()
	pass.Pop()
	return at, fired
}
