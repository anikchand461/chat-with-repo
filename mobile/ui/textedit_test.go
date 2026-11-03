package ui

import (
	"image"
	"io"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// field runs a text field through Gio's real input router, one frame at a
// time.
type field struct {
	t   *testing.T
	r   input.Router
	ed  widget.Editor
	th  *material.Theme
	now time.Time
}

func newField(t *testing.T, text string) *field {
	f := &field{t: t, th: material.NewTheme(), now: time.Unix(1000, 0)}
	f.th.Shaper = newShaper()
	f.ed.SetText(text)
	t.Cleanup(func() { delete(editMenus, &f.ed) })
	return f
}

func (f *field) frame(extra ...func(gtx layout.Context)) {
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Now:         f.now,
		Source:      f.r.Source(),
		Constraints: layout.Exact(image.Pt(400, 60)),
	}
	for _, fn := range extra {
		fn(gtx)
	}
	layoutEditor(gtx, f.th, material.Editor(f.th, &f.ed, "hint"))
	f.r.Frame(&ops)
}

func (f *field) focus() {
	f.frame(func(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: &f.ed}) })
	f.frame()
}

func (f *field) touch(kind pointer.Kind, after time.Duration) {
	f.touchAt(kind, after, f32.Pt(20, 20))
}

func (f *field) touchAt(kind pointer.Kind, after time.Duration, p f32.Point) {
	f.now = f.now.Add(after)
	f.r.Queue(pointer.Event{Kind: kind, Source: pointer.Touch, Position: p, Time: time.Duration(f.now.UnixNano())})
	f.frame()
}

// longPress holds a finger on p until the menu opens, then lifts it.
func (f *field) longPress(p f32.Point) {
	f.touchAt(pointer.Press, 0, p)
	f.now = f.now.Add(longPressDelay)
	f.frame()
	f.touchAt(pointer.Release, 50*time.Millisecond, p)
}

func (f *field) menuOpen() bool { return editMenus[&f.ed] != nil && editMenus[&f.ed].menu }

func TestLongPressOpensTheMenuAndATapDoesNot(t *testing.T) {
	f := newField(t, "hello")
	f.frame()

	f.touch(pointer.Press, 0)
	f.touch(pointer.Release, 100*time.Millisecond)
	f.now = f.now.Add(time.Second)
	f.frame()
	if f.menuOpen() {
		t.Fatal("a quick tap must not open the menu")
	}

	f.touch(pointer.Press, 0)
	f.now = f.now.Add(longPressDelay)
	f.frame()
	if !f.menuOpen() {
		t.Fatal("holding the field should open the menu")
	}
}

func TestPasteInsertsTheClipboard(t *testing.T) {
	f := newField(t, "owner/")
	f.focus()
	f.longPress(f32.Pt(300, 20)) // the empty part of the field: no word, just the menu

	editMenus[&f.ed].paste.Click()
	f.frame()
	if !f.r.ClipboardRequested() {
		t.Fatal("Paste should read the clipboard")
	}
	f.r.Queue(transfer.DataEvent{Type: "application/text", Open: func() io.ReadCloser {
		return io.NopCloser(strings.NewReader("repo"))
	}})
	f.frame()

	if got := f.ed.Text(); got != "owner/repo" {
		t.Fatalf("text = %q", got)
	}
	if f.menuOpen() {
		t.Fatal("the menu should close after pasting")
	}
}

func TestSelectAllThenCut(t *testing.T) {
	f := newField(t, "secret text")
	f.focus()
	f.touch(pointer.Press, 0)
	f.now = f.now.Add(longPressDelay)
	f.frame()

	editMenus[&f.ed].selectAll.Click()
	f.frame()
	if f.ed.SelectedText() != "secret text" || !f.menuOpen() {
		t.Fatalf("selected %q, menu open %v", f.ed.SelectedText(), f.menuOpen())
	}

	editMenus[&f.ed].cut.Click()
	f.frame()
	if f.ed.Text() != "" || f.menuOpen() {
		t.Fatalf("text %q after cut, menu open %v", f.ed.Text(), f.menuOpen())
	}
}

func TestKeyboardSeesTheWholeTextAndDeletesWithoutARestart(t *testing.T) {
	f := newField(t, "hello world")
	f.focus()
	f.ed.SetCaret(f.ed.Len(), f.ed.Len())
	f.frame()

	got := f.r.EditorState().Snippet
	if got.Text != "hello world" || got.Start != 0 || got.End != 11 {
		t.Fatalf("keyboard sees %+v, want the whole text", got)
	}

	// The keyboard deletes the last character, the way Gboard does when it
	// can see the text. Gio's window updates its copy of the snippet
	// itself (app/ime.go Replace); the Android backend restarts the
	// keyboard whenever the editor's snippet then differs from that copy.
	f.r.Queue(key.EditEvent{Range: key.Range{Start: 10, End: 11}})
	f.frame()

	want := key.Snippet{Range: key.Range{Start: 0, End: 10}, Text: "hello worl"}
	if got := f.r.EditorState().Snippet; got != want {
		t.Fatalf("snippet after delete = %+v, want %+v (a mismatch restarts the keyboard)", got, want)
	}
}

func TestLongPressSelectsTheWordAndLiftingKeepsIt(t *testing.T) {
	f := newField(t, "hello world")
	f.frame()

	// A point inside "world".
	p, _ := boundaryPoint(&f.ed, 8, false)
	f.longPress(f32.Pt(float32(p.X), float32(p.Y-5)))

	if got := f.ed.SelectedText(); got != "world" {
		t.Fatalf("selected %q, want the word under the finger", got)
	}
	if !f.menuOpen() {
		t.Fatal("menu should be open")
	}
}

func TestDraggingTheEndHandleExtendsTheSelection(t *testing.T) {
	f := newField(t, "hello big world")
	f.frame()
	p, _ := boundaryPoint(&f.ed, 2, false)
	f.longPress(f32.Pt(float32(p.X), float32(p.Y-5)))
	if f.ed.SelectedText() != "hello" {
		t.Fatalf("selected %q", f.ed.SelectedText())
	}

	// Grab the end handle (it hangs below the end of the selection) and drag
	// it to the end of the text.
	var gtx layout.Context
	gtx.Metric.PxPerDp = 1
	r := float32(gtx.Dp(handleRadius))
	end, _ := boundaryPoint(&f.ed, 5, true)
	last, _ := boundaryPoint(&f.ed, f.ed.Len(), true)
	grab := f32.Pt(float32(end.X)+r, float32(end.Y)+r)
	f.touchAt(pointer.Press, 0, grab)
	f.touchAt(pointer.Move, 20*time.Millisecond, grab.Add(f32.Pt(float32(last.X-end.X), 0)))
	f.touchAt(pointer.Release, 20*time.Millisecond, grab.Add(f32.Pt(float32(last.X-end.X), 0)))

	if got := f.ed.SelectedText(); got != "hello big world" {
		t.Fatalf("selected %q after dragging the handle", got)
	}
}

func TestCopyIsOfferedForAnySelection(t *testing.T) {
	f := newField(t, "copy me")
	f.focus()
	f.ed.SetCaret(4, 0) // e.g. selected by a double tap
	f.frame()

	m := editMenus[&f.ed]
	m.copy.Click()
	f.frame()
	if f.ed.SelectionLen() != 0 {
		t.Fatal("copy should end the selection, like Android")
	}
}

func TestPasswordsCannotBeSelected(t *testing.T) {
	f := newField(t, "hunter2")
	f.ed.Mask = '•'
	f.frame()
	f.longPress(f32.Pt(10, 10))
	if f.ed.SelectionLen() != 0 {
		t.Fatal("a masked field must not select text")
	}
	if !f.menuOpen() {
		t.Fatal("the menu (Paste) should still open")
	}
}
