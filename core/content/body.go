// Package content parses forum markup once for all presentation adapters.
package content

import (
	"html"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

type Kind int

const (
	Text Kind = iota
	Image
	File
	Emoji
	Link
)

type Style struct{ Bold, Italic, Underline, Quote bool }
type Part struct {
	Kind               Kind
	Text, URL, FullURL string
	Style              Style
}

var imageHTML = regexp.MustCompile(`(?is)<img\b[^>]*src\s*=\s*["']([^"']+)["'][^>]*>`)
var breaks = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</div>`)
var tags = regexp.MustCompile(`<[^>]*>`)
var token = regexp.MustCompile(`(?is)\[upload=(\d+)\](?:\s*\[/upload\])?|\[img(?:=[^\]]*)?\](.*?)\[/img\]|\[em([a-zA-Z0-9]+)\]|\[url(?:=([^\]]+))?\](.*?)\[/url\]|\[(/?)(b|i|u|quote)(?:=[^\]]*)?\]|\[/?(?:color|size|font|upload)(?:=[^\]]*)?\]`)

func Parse(body string, attachments []forum.Attachment) []Part {
	body = imageHTML.ReplaceAllString(body, "[img]$1[/img]")
	body = breaks.ReplaceAllString(body, "\n")
	var files []forum.File
	for _, a := range attachments {
		files = append(files, a.Files...)
	}
	used := map[int]bool{}
	var out []Part
	style := Style{}
	addText := func(s string) {
		s = html.UnescapeString(tags.ReplaceAllString(s, ""))
		if s != "" {
			out = append(out, Part{Kind: Text, Text: s, Style: style})
		}
	}
	cursor := 0
	for _, m := range token.FindAllStringSubmatchIndex(body, -1) {
		addText(body[cursor:m[0]])
		cursor = m[1]
		group := func(n int) string {
			if m[2*n] < 0 {
				return ""
			}
			return body[m[2*n]:m[2*n+1]]
		}
		switch {
		case group(1) != "":
			i, _ := strconv.Atoi(group(1))
			i--
			if i < 0 || i >= len(files) {
				addText("[图片缺失]")
			} else {
				used[i] = true
				out = append(out, FilePart(files[i]))
			}
		case group(2) != "":
			out = append(out, Part{Kind: Image, URL: html.UnescapeString(strings.TrimSpace(group(2))), FullURL: html.UnescapeString(strings.TrimSpace(group(2)))})
		case group(3) != "":
			out = append(out, Part{Kind: Emoji, Text: "[em" + group(3) + "]", URL: "em" + group(3) + ".gif"})
		case group(5) != "":
			u := group(4)
			if u == "" {
				u = group(5)
			}
			out = append(out, Part{Kind: Link, Text: html.UnescapeString(group(5)), URL: html.UnescapeString(u), Style: style})
		case group(7) != "":
			on := group(6) == ""
			switch strings.ToLower(group(7)) {
			case "b":
				style.Bold = on
			case "i":
				style.Italic = on
			case "u":
				style.Underline = on
			case "quote":
				style.Quote = on
			}
		}
	}
	addText(body[cursor:])
	for i, f := range files {
		if !used[i] {
			out = append(out, FilePart(f))
		}
	}
	return out
}
func FilePart(f forum.File) Part {
	filePath := f.URL
	if u, err := url.Parse(f.URL); err == nil {
		filePath = u.Path
	}
	ext := strings.ToLower(path.Ext(filePath))
	image := f.ThumbnailMiddle != "" || f.ThumbnailSmall != ""
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		image = true
	}
	kind := File
	if image {
		kind = Image
	}
	thumb := f.ThumbnailMiddle
	if thumb == "" {
		thumb = f.ThumbnailSmall
	}
	if thumb == "" {
		thumb = f.URL
	}
	return Part{Kind: kind, Text: f.Name, URL: thumb, FullURL: f.URL}
}
func Plain(body string) string {
	var b strings.Builder
	for _, p := range Parse(body, nil) {
		switch p.Kind {
		case Text, Link, Emoji:
			b.WriteString(p.Text)
		case Image:
			b.WriteString("[图片]")
		case File:
			b.WriteString(p.Text)
		}
	}
	return b.String()
}
