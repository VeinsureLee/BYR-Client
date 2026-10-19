package byr

import (
	"bytes"
	"html"
	"strings"

	xhtml "golang.org/x/net/html"
)

func parseHTML(data []byte) (*xhtml.Node, error) {
	return xhtml.Parse(bytes.NewReader(data))
}

func walkHTML(root *xhtml.Node, fn func(*xhtml.Node) bool) {
	if root == nil {
		return
	}
	if !fn(root) {
		return
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, fn)
	}
}

func attr(node *xhtml.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, item := range node.Attr {
		if item.Key == key {
			return item.Val
		}
	}
	return ""
}

func hasClass(node *xhtml.Node, class string) bool {
	if node == nil {
		return false
	}
	for _, field := range strings.Fields(attr(node, "class")) {
		if field == class {
			return true
		}
	}
	return false
}

func findHTML(root *xhtml.Node, match func(*xhtml.Node) bool) *xhtml.Node {
	var found *xhtml.Node
	walkHTML(root, func(node *xhtml.Node) bool {
		if found != nil {
			return false
		}
		if match(node) {
			found = node
			return false
		}
		return true
	})
	return found
}

func findAllHTML(root *xhtml.Node, match func(*xhtml.Node) bool) []*xhtml.Node {
	var found []*xhtml.Node
	walkHTML(root, func(node *xhtml.Node) bool {
		if match(node) {
			found = append(found, node)
		}
		return true
	})
	return found
}

func findClass(root *xhtml.Node, class string) *xhtml.Node {
	return findHTML(root, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && hasClass(node, class)
	})
}

func findAllClass(root *xhtml.Node, class string) []*xhtml.Node {
	return findAllHTML(root, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && hasClass(node, class)
	})
}

func findTag(root *xhtml.Node, tag string) *xhtml.Node {
	return findHTML(root, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && node.Data == tag
	})
}

func findAllTags(root *xhtml.Node, tag string) []*xhtml.Node {
	return findAllHTML(root, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && node.Data == tag
	})
}

func textContent(root *xhtml.Node) string {
	if root == nil {
		return ""
	}
	var b strings.Builder
	walkHTML(root, func(node *xhtml.Node) bool {
		switch node.Type {
		case xhtml.TextNode:
			b.WriteString(node.Data)
		case xhtml.ElementNode:
			switch node.Data {
			case "br":
				b.WriteByte('\n')
			case "p", "div", "li", "tr":
				if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
					b.WriteByte('\n')
				}
			}
		}
		return true
	})
	return strings.TrimSpace(b.String())
}

func cleanText(value string) string {
	return strings.TrimSpace(html.UnescapeString(value))
}
