package ui

import (
	"image"
	"strings"
	"unicode"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/richtext"
	"gioui.org/x/styledtext"

	"github.com/VeinsureLee/BYR-Client/core/application"
	"github.com/VeinsureLee/BYR-Client/core/content"
	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type threadReadRequest struct {
	Serial, Generation uint64
	Route              Route
	HistoryToken       application.HistoryReadToken
}

type threadViewportKey struct {
	Width int
	Scale float32
}

type threadAnchor struct {
	Row       int
	Offset    int
	ArticleID string
	Signature string
}

type threadBlockKind uint8

const (
	threadParagraphBlock threadBlockKind = iota
	threadBlankBlock
	threadImageBlock
	threadFileBlock
)

type threadBlock struct {
	kind  threadBlockKind
	runs  []contentRun
	part  content.Part
	atoms []textAtom
}

type contentRun struct {
	kind  content.Kind
	text  string
	url   string
	full  string
	style content.Style
}

type textAtom struct {
	run   contentRun
	text  string
	space bool
}

type atomWidthKey struct {
	text   string
	weight font.Weight
	scale  float32
}

// postThreadContent is prepared when a thread page arrives. It holds parsed
// blocks, copyable text and reply metadata so scrolling only lays out styles.
type postThreadContent struct {
	parts  []content.Part
	plain  string
	blocks []threadBlock

	replyPosition int
	replyAuthor   string

	atomWidths map[atomWidthKey]int
}

func buildThreadContent(body string, attachments []forum.Attachment) *postThreadContent {
	parts := content.Parse(body, attachments)
	pc := &postThreadContent{
		parts:      parts,
		plain:      content.PlainParts(parts),
		atomWidths: map[atomWidthKey]int{},
	}
	var paragraph []contentRun
	flush := func() {
		if len(paragraph) == 0 {
			return
		}
		block := threadBlock{kind: threadParagraphBlock, runs: paragraph}
		block.atoms = atomizeRuns(paragraph)
		pc.blocks = append(pc.blocks, block)
		paragraph = nil
	}
	for _, part := range parts {
		switch part.Kind {
		case content.Text:
			lines := strings.Split(part.Text, "\n")
			for index, line := range lines {
				if index > 0 {
					flush()
				}
				if line == "" {
					pc.blocks = append(pc.blocks, threadBlock{kind: threadBlankBlock})
					continue
				}
				paragraph = append(paragraph, contentRun{kind: content.Text, text: line, style: part.Style})
			}
		case content.Link:
			text := part.Text
			if text == "" {
				text = part.URL
			}
			paragraph = append(paragraph, contentRun{kind: content.Link, text: text, url: part.URL, style: part.Style})
		case content.Emoji:
			paragraph = append(paragraph, contentRun{kind: content.Emoji, text: part.Text, url: part.URL})
		case content.Image:
			flush()
			pc.blocks = append(pc.blocks, threadBlock{kind: threadImageBlock, part: part})
		case content.File:
			flush()
			pc.blocks = append(pc.blocks, threadBlock{kind: threadFileBlock, part: part})
		}
	}
	flush()
	return pc
}

// atomizeRuns keeps natural paragraph flow: CJK runes, Latin words, spaces and
// inline emoji are separate measurement units but are laid out on one line when
// they fit, so an inline [b] run no longer starts a new row.
func atomizeRuns(runs []contentRun) []textAtom {
	var atoms []textAtom
	for _, run := range runs {
		switch run.kind {
		case content.Emoji:
			atoms = append(atoms, textAtom{run: run})
		case content.Link:
			atoms = append(atoms, textAtom{run: run, text: run.text})
		default:
			var word strings.Builder
			flushWord := func() {
				if word.Len() > 0 {
					atoms = append(atoms, textAtom{run: run, text: word.String()})
					word.Reset()
				}
			}
			for _, r := range run.text {
				switch {
				case unicode.IsSpace(r):
					flushWord()
					atoms = append(atoms, textAtom{run: run, text: " ", space: true})
				case isWideRune(r):
					flushWord()
					atoms = append(atoms, textAtom{run: run, text: string(r)})
				default:
					word.WriteRune(r)
				}
			}
			flushWord()
		}
	}
	return atoms
}

func isWideRune(r rune) bool {
	return r >= 0x2E80 && r <= 0x9FFF || r >= 0xF900 && r <= 0xFAFF || r >= 0xFF00 && r <= 0xFFEF ||
		r >= 0x3000 && r <= 0x303F
}

func (pc *postThreadContent) paragraphText(block threadBlock) string {
	var b strings.Builder
	for _, run := range block.runs {
		b.WriteString(run.text)
	}
	return b.String()
}

func simpleRunStyle(block threadBlock) (content.Style, bool) {
	if len(block.runs) == 0 {
		return content.Style{}, false
	}
	style := block.runs[0].style
	for _, run := range block.runs {
		if run.kind != content.Text || run.style != style {
			return content.Style{}, false
		}
	}
	return style, true
}

func (a *App) prepareThreadContent(thread forum.Thread) {
	models := make(map[string]*postThreadContent, len(thread.Articles))
	positions := make(map[string]forum.Article, len(thread.Articles))
	for _, article := range thread.Articles {
		models[string(article.ID)] = buildThreadContent(article.Content, article.Attachments)
		positions[string(article.ID)] = article
	}
	for _, article := range thread.Articles {
		model := models[string(article.ID)]
		if model == nil {
			continue
		}
		id := string(article.ReplyID)
		if id == "" || id == "0" {
			continue
		}
		if target, ok := positions[id]; ok {
			model.replyPosition = int(target.Position)
			model.replyAuthor = target.Author.DisplayName()
		}
	}
	a.threadModels = models
}

func (a *App) threadContent(article forum.Article) *postThreadContent {
	if a.threadModels != nil {
		if model := a.threadModels[string(article.ID)]; model != nil {
			return model
		}
	}
	model := buildThreadContent(article.Content, article.Attachments)
	id := string(article.ReplyID)
	if id != "" && id != "0" {
		for _, target := range a.thread.Articles {
			if string(target.ID) == id {
				model.replyPosition = int(target.Position)
				model.replyAuthor = target.Author.DisplayName()
				break
			}
		}
	}
	if a.threadModels == nil {
		a.threadModels = make(map[string]*postThreadContent)
	}
	a.threadModels[string(article.ID)] = model
	return model
}

func (a *App) postBodyContent(gtx layout.Context, pc *postThreadContent) layout.Dimensions {
	if pc == nil || len(pc.blocks) == 0 {
		return material.Body1(a.theme, "(无内容)").Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, a.contentBlocks(gtx, pc)...)
}

func (a *App) contentBlocks(gtx layout.Context, pc *postThreadContent) []layout.FlexChild {
	children := make([]layout.FlexChild, 0, len(pc.blocks))
	for index := range pc.blocks {
		index := index
		block := pc.blocks[index]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			switch block.kind {
			case threadBlankBlock:
				return layout.Spacer{Height: unit.Dp(16)}.Layout(gtx)
			case threadImageBlock:
				return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.inlineImage(gtx, block.part.URL, block.part.FullURL)
				})
			case threadFileBlock:
				name := block.part.Text
				if name == "" {
					name = "附件"
				}
				return material.Caption(a.theme, "[附件] "+name).Layout(gtx)
			default:
				return a.threadParagraph(gtx, pc, index)
			}
		}))
	}
	return children
}

func (a *App) threadParagraph(gtx layout.Context, pc *postThreadContent, index int) layout.Dimensions {
	block := pc.blocks[index]
	if style, ok := simpleRunStyle(block); ok {
		l := material.Label(a.theme, unit.Sp(16), pc.paragraphText(block))
		l.LineHeight = unit.Sp(25)
		if style.Bold {
			l.Font.Weight = font.SemiBold
		}
		if style.Italic {
			l.Font.Style = font.Italic
		}
		if style.Quote {
			l.Color = colorHint
		}
		return l.Layout(gtx)
	}
	if !hasEmojiRuns(block.runs) {
		return a.richParagraph(gtx, block.runs)
	}
	return a.inlineParagraph(gtx, pc, block.atoms)
}

func hasEmojiRuns(runs []contentRun) bool {
	for _, run := range runs {
		if run.kind == content.Emoji {
			return true
		}
	}
	return false
}

// richParagraph uses the Gio styledtext engine so inline weights and colors are
// shaped together with one wrapping pass. Emoji paragraphs keep the bitmap
// flow below because inline images cannot be represented as text spans.
func (a *App) richParagraph(gtx layout.Context, runs []contentRun) layout.Dimensions {
	styles := make([]richtext.SpanStyle, 0, len(runs))
	for _, run := range runs {
		f := font.Font{Typeface: a.theme.Face, Weight: font.Normal}
		fg := a.theme.Fg
		if run.style.Bold {
			f.Weight = font.SemiBold
		}
		if run.style.Italic {
			f.Style = font.Italic
		}
		if run.kind == content.Link {
			fg = colorAccent
		} else if run.style.Quote {
			fg = colorHint
		}
		styles = append(styles, richtext.SpanStyle{
			Font: f, Size: unit.Sp(16), Color: fg, Content: run.text,
		})
	}
	paragraph := richtext.Text(nil, a.theme.Shaper, styles...)
	paragraph.LineHeight = unit.Sp(20)
	paragraph.WrapPolicy = styledtext.WrapGraphemes
	return paragraph.Layout(gtx)
}

// inlineParagraph lays out mixed-style runs on shared lines. Atom widths are
// cached per font scale; a line is then drawn as one label per style run, so a
// scroll frame does not measure or shape every character again.
type inlineLineRun struct {
	run  contentRun
	text string
}

func (a *App) inlineParagraph(gtx layout.Context, pc *postThreadContent, atoms []textAtom) layout.Dimensions {
	maxWidth := gtx.Constraints.Max.X
	if maxWidth <= 0 {
		maxWidth = gtx.Dp(unit.Dp(320))
	}
	lineHeight := gtx.Sp(unit.Sp(25))
	x, y := 0, 0
	lineHasText := false
	var line []inlineLineRun
	appendAtom := func(atom textAtom) {
		if atom.run.kind == content.Emoji {
			line = append(line, inlineLineRun{run: atom.run})
			return
		}
		text := atom.text
		if atom.space && len(line) > 0 {
			last := &line[len(line)-1]
			if last.run.kind != content.Emoji && last.run.style == atom.run.style && last.run.kind == atom.run.kind {
				last.text += text
				return
			}
		}
		if len(line) > 0 {
			last := &line[len(line)-1]
			if last.run.kind != content.Emoji && last.run.style == atom.run.style && last.run.kind == atom.run.kind && last.run.url == atom.run.url {
				last.text += text
				return
			}
		}
		line = append(line, inlineLineRun{run: atom.run, text: text})
	}
	flush := func(y int) int {
		if len(line) == 0 {
			return 0
		}
		width := a.drawInlineLine(gtx, line, y, lineHeight, maxWidth)
		line = line[:0]
		return width
	}
	for _, atom := range atoms {
		width := a.atomWidth(gtx, pc, atom)
		if atom.space && !lineHasText {
			continue
		}
		if lineHasText && x+width > maxWidth {
			flush(y)
			y += lineHeight
			x = 0
			lineHasText = false
			if atom.space {
				continue
			}
		}
		if width > maxWidth {
			for _, r := range atom.text {
				sub := textAtom{run: atom.run, text: string(r)}
				runeWidth := a.runewidth(gtx, pc, sub)
				if lineHasText && x+runeWidth > maxWidth {
					flush(y)
					y += lineHeight
					x = 0
					lineHasText = false
				}
				appendAtom(sub)
				x += runeWidth
				lineHasText = true
			}
			continue
		}
		appendAtom(atom)
		x += width
		if !atom.space {
			lineHasText = true
		}
	}
	flush(y)
	height := y
	if lineHasText {
		height = y + lineHeight
	}
	return layout.Dimensions{Size: image.Pt(maxWidth, height)}
}

// drawInlineLine records one label per style run and centers each run in the
// line box. It returns the drawn width for callers that track the last line.
func (a *App) drawInlineLine(gtx layout.Context, runs []inlineLineRun, y, lineHeight, maxWidth int) int {
	x := 0
	for _, item := range runs {
		if item.run.kind == content.Emoji {
			size := gtx.Dp(unit.Dp(24))
			off := op.Offset(image.Pt(x, y+(lineHeight-size)/2)).Push(gtx.Ops)
			a.inlineEmoji(gtx, "emoji:"+item.run.url, item.run.text)
			off.Pop()
			x += size
			continue
		}
		l := material.Label(a.theme, unit.Sp(16), item.text)
		l.MaxLines = 1
		if item.run.kind == content.Link {
			l.Color = colorAccent
		} else {
			if item.run.style.Bold {
				l.Font.Weight = font.SemiBold
			}
			if item.run.style.Italic {
				l.Font.Style = font.Italic
			}
			if item.run.style.Quote {
				l.Color = colorHint
			}
		}
		macro := op.Record(gtx.Ops)
		startGtx := gtx
		startGtx.Constraints.Min = image.Point{}
		startGtx.Constraints.Max.X = maxWidth
		dims := l.Layout(startGtx)
		call := macro.Stop()
		off := op.Offset(image.Pt(x, y+(lineHeight-dims.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		x += dims.Size.X
	}
	return x
}

func (a *App) atomWidth(gtx layout.Context, pc *postThreadContent, atom textAtom) int {
	if atom.run.kind == content.Emoji {
		return gtx.Dp(unit.Dp(24))
	}
	return a.runewidth(gtx, pc, atom)
}

func (a *App) runewidth(gtx layout.Context, pc *postThreadContent, atom textAtom) int {
	if atom.run.kind == content.Emoji {
		return gtx.Dp(unit.Dp(24))
	}
	weight := font.Normal
	if atom.run.style.Bold {
		weight = font.SemiBold
	}
	key := atomWidthKey{text: atom.text, weight: weight, scale: gtx.Metric.PxPerSp}
	if pc.atomWidths != nil {
		if width, ok := pc.atomWidths[key]; ok {
			return width
		}
	}
	l := material.Label(a.theme, unit.Sp(16), atom.text)
	l.MaxLines = 1
	l.Font.Weight = weight
	if atom.run.style.Italic {
		l.Font.Style = font.Italic
	}
	var scratch op.Ops
	measureCtx := gtx
	measureCtx.Ops = &scratch
	measureCtx.Constraints = layout.Constraints{Max: image.Pt(100000, gtx.Sp(unit.Sp(25)))}
	width := l.Layout(measureCtx).Size.X
	if atom.text == " " && width == 0 {
		width = gtx.Dp(unit.Dp(4))
	}
	if pc.atomWidths == nil {
		pc.atomWidths = map[atomWidthKey]int{}
	}
	pc.atomWidths[key] = width
	return width
}
