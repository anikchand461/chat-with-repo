package ui

import (
	"image"
	"image/color"
	"regexp"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// A small, dependency-free Markdown renderer for chat answers. It handles
// the subset the web chat shows: paragraphs (line breaks preserved),
// headings, bullet and numbered lists, fenced code blocks, and inline
// **bold** and `code`. The response string itself is never modified.

type mdKind int

const (
	mdParagraph mdKind = iota
	mdHeading
	mdBullet
	mdNumbered
	mdCode
)

type mdBlock struct {
	kind   mdKind
	level  int    // heading level, or list nesting depth
	marker string // "•" or "1."
	text   string // raw text (code blocks keep it verbatim)
}

type mdWord struct {
	text string
	bold bool
	code bool
	nl   bool // forced line break
	glue bool // no space before this word (e.g. punctuation after **bold**)
}

var (
	reHeading  = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.*)$`)
	reBullet   = regexp.MustCompile(`^(\s*)[-*+•]\s+(.*)$`)
	reNumbered = regexp.MustCompile(`^(\s*)(\d+)[.)]\s+(.*)$`)
)

// parseMarkdown splits raw into blocks. With markdown == false (user
// messages) only paragraph/line-break structure is kept.
func parseMarkdown(raw string, markdown bool) []mdBlock {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")

	var blocks []mdBlock
	var para []string
	flush := func() {
		if len(para) > 0 {
			blocks = append(blocks, mdBlock{kind: mdParagraph, text: strings.Join(para, "\n")})
			para = nil
		}
	}

	if !markdown {
		for _, ln := range lines {
			para = append(para, ln)
		}
		text := strings.Trim(strings.Join(para, "\n"), "\n")
		return []mdBlock{{kind: mdParagraph, text: text}}
	}

	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		trim := strings.TrimSpace(ln)

		if strings.HasPrefix(trim, "```") {
			flush()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			blocks = append(blocks, mdBlock{kind: mdCode, text: strings.Join(code, "\n")})
			continue
		}
		if trim == "" {
			flush()
			continue
		}
		if m := reHeading.FindStringSubmatch(ln); m != nil {
			flush()
			blocks = append(blocks, mdBlock{kind: mdHeading, level: len(m[1]), text: m[2]})
			continue
		}
		if m := reBullet.FindStringSubmatch(ln); m != nil {
			flush()
			blocks = append(blocks, mdBlock{kind: mdBullet, level: len(m[1]) / 2, marker: "•", text: m[2]})
			continue
		}
		if m := reNumbered.FindStringSubmatch(ln); m != nil {
			flush()
			blocks = append(blocks, mdBlock{kind: mdNumbered, level: len(m[1]) / 2, marker: m[2] + ".", text: m[3]})
			continue
		}
		para = append(para, ln)
	}
	flush()
	if len(blocks) == 0 {
		blocks = []mdBlock{{kind: mdParagraph, text: raw}}
	}
	return blocks
}

// inlineWords tokenises one block of text into styled words, honouring
// **bold** and `code` spans and preserving newlines as forced breaks.
func inlineWords(text string, markdown, baseBold bool) []mdWord {
	var out []mdWord
	var cur strings.Builder
	bold, code := baseBold, false
	prev := ' ' // last source rune before the current segment started
	last := ' ' // last source text rune consumed

	emit := func() {
		seg := cur.String()
		cur.Reset()
		glue := prev != ' ' && prev != '\n' && len(out) > 0 && !out[len(out)-1].nl
		for i, w := range strings.Fields(seg) {
			out = append(out, mdWord{text: w, bold: bold, code: code, glue: glue && i == 0 && !startsSpace(seg)})
		}
	}
	write := func(r rune) {
		if cur.Len() == 0 {
			prev = last
		}
		cur.WriteRune(r)
		last = r
	}

	rs := []rune(text)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\n':
			emit()
			out = append(out, mdWord{nl: true})
			last = '\n'
		case markdown && r == '`' && !code:
			if j := indexRune(rs, '`', i+1); j > 0 {
				emit()
				code = true
				for _, cr := range rs[i+1 : j] {
					write(cr)
				}
				emit()
				code = false
				i = j
			} else {
				write(r)
			}
		case markdown && r == '*' && i+1 < len(rs) && rs[i+1] == '*':
			emit()
			bold = !bold
			i++
		default:
			write(r)
		}
	}
	emit()
	return out
}

func startsSpace(s string) bool {
	return s != "" && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n')
}

func indexRune(rs []rune, target rune, from int) int {
	for i := from; i < len(rs); i++ {
		if rs[i] == target {
			return i
		}
		if rs[i] == '\n' {
			return -1
		}
	}
	return -1
}

var (
	mdCodeFg = color.NRGBA{R: 0x7e, G: 0xe8, B: 0xc0, A: 0xff}
	mdCodeBg = color.NRGBA{R: 0x0b, G: 0x2a, B: 0x21, A: 0xff}
	monoFont = font.Font{Typeface: "Go Mono"}
)

type placed struct {
	call op.CallOp
	w, h int
	code bool
}

// flowText lays words out left-to-right, wrapping at the available width.
// Own flow layout is needed because material.Label can't mix bold, code
// and regular runs inside one wrapped paragraph.
func flowText(gtx layout.Context, th *material.Theme, words []mdWord, size unit.Sp, fg color.NRGBA) layout.Dimensions {
	maxW := gtx.Constraints.Max.X
	space := gtx.Sp(size) * 30 / 100
	padX := gtx.Dp(unit.Dp(3))

	var (
		x, y, lineH, widest int
		line                []placed
		xs                  []int
	)
	flush := func() {
		if len(line) == 0 {
			return
		}
		for i, p := range line {
			ox, oy := xs[i], y+(lineH-p.h)/2
			st := op.Offset(image.Pt(ox, oy)).Push(gtx.Ops)
			if p.code {
				r := gtx.Dp(unit.Dp(5))
				paint.FillShape(gtx.Ops, mdCodeBg, clip.RRect{Rect: image.Rect(0, 0, p.w, p.h), NE: r, NW: r, SE: r, SW: r}.Op(gtx.Ops))
			}
			p.call.Add(gtx.Ops)
			st.Pop()
		}
		y += lineH
		line, xs, x, lineH = nil, nil, 0, 0
	}

	for idx, w := range words {
		if w.nl {
			if len(line) == 0 {
				// Blank line inside a paragraph: keep a small gap.
				y += gtx.Sp(size) * 6 / 10
			}
			flush()
			continue
		}
		lbl := material.Label(th, size, w.text)
		lbl.Color = fg
		lbl.LineHeightScale = 1.3
		if w.bold {
			lbl.Font.Weight = font.Bold
		}
		pad := 0
		if w.code {
			lbl.Font = monoFont
			lbl.Color = mdCodeFg
			lbl.TextSize = size * 92 / 100
			pad = padX
		}
		c := gtx
		c.Constraints.Min = image.Point{}
		c.Constraints.Max.X = maxW - 2*pad
		rec := op.Record(gtx.Ops)
		off := op.Offset(image.Pt(pad, 0)).Push(gtx.Ops)
		d := lbl.Layout(c)
		off.Pop()
		call := rec.Stop()
		pw, ph := d.Size.X+2*pad, d.Size.Y

		gap := 0
		if x > 0 && !w.glue {
			gap = space
		}
		if x > 0 && x+gap+pw > maxW {
			flush()
			gap = 0
		}
		x += gap
		line = append(line, placed{call: call, w: pw, h: ph, code: w.code})
		xs = append(xs, x)
		x += pw
		if x > widest {
			widest = x
		}
		if ph > lineH {
			lineH = ph
		}
		_ = idx
	}
	flush()
	return layout.Dimensions{Size: image.Pt(widest, y)}
}

// renderMarkdown draws raw as a vertical stack of blocks.
func renderMarkdown(gtx layout.Context, th *material.Theme, raw string, fg color.NRGBA, markdown bool) layout.Dimensions {
	blocks := parseMarkdown(raw, markdown)
	const body = unit.Sp(15)

	children := make([]layout.FlexChild, 0, len(blocks))
	for i, b := range blocks {
		b := b
		top := unit.Dp(0)
		if i > 0 {
			top = 8
			if (b.kind == mdBullet || b.kind == mdNumbered) && (blocks[i-1].kind == b.kind) {
				top = 4
			}
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: top}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				switch b.kind {
				case mdCode:
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx layout.Context) layout.Dimensions {
							borderedRRect(gtx, gtx.Constraints.Min, unit.Dp(12), authField, authFieldBorder)
							return layout.Dimensions{Size: gtx.Constraints.Min}
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Label(th, unit.Sp(13), strings.ReplaceAll(b.text, "\t", "    "))
								l.Font = monoFont
								l.Color = authTitle
								l.LineHeightScale = 1.3
								return l.Layout(gtx)
							})
						}),
					)
				case mdHeading:
					sz := body + unit.Sp(9-2*minInt(b.level, 3))
					return flowText(gtx, th, inlineWords(b.text, markdown, true), sz, fg)
				case mdBullet, mdNumbered:
					indent := gtx.Dp(unit.Dp(float32(b.level) * 16))
					markerW := gtx.Dp(unit.Dp(22))
					if b.kind == mdNumbered {
						markerW = gtx.Dp(unit.Dp(26))
					}
					return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						defer op.Offset(image.Pt(indent, 0)).Push(gtx.Ops).Pop()
						gtx.Constraints.Max.X -= indent
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Label(th, body, b.marker)
								l.Color = fg
								l.LineHeightScale = 1.3
								gtx.Constraints.Min.X = markerW
								gtx.Constraints.Max.X = markerW
								return l.Layout(gtx)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return flowText(gtx, th, inlineWords(b.text, markdown, false), body, fg)
							}),
						)
					})
				default:
					return flowText(gtx, th, inlineWords(b.text, markdown, false), body, fg)
				}
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
