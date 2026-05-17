package render

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// inlineText renders the inline content of a node as markdown.
func inlineText(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderInline(c, &b)
	}
	return collapseSpaces(b.String())
}

func renderInline(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	if n.Type != html.ElementNode {
		return
	}
	switch strings.ToLower(n.Data) {
	case "strong", "b":
		b.WriteString("**")
		childrenInline(n, b)
		b.WriteString("**")
	case "em", "i":
		b.WriteString("*")
		childrenInline(n, b)
		b.WriteString("*")
	case "code":
		b.WriteString("`")
		b.WriteString(textContent(n))
		b.WriteString("`")
	case "br":
		b.WriteString("\n")
	case "a":
		text := childrenInlineString(n)
		href := attr(n, "href")
		if href != "" {
			b.WriteString("[" + text + "](" + href + ")")
		} else {
			b.WriteString(text)
		}
	case "ac:link":
		// Confluence cross-link: prefer the explicit link body, else the page title.
		if t := childrenInlineString(n); strings.TrimSpace(t) != "" {
			b.WriteString(t)
		} else if title := linkTarget(n); title != "" {
			b.WriteString(title)
		}
	case "ac:image":
		b.WriteString("[image]")
	default:
		// Unknown / namespaced inline element: render its children.
		childrenInline(n, b)
	}
}

func childrenInline(n *html.Node, b *strings.Builder) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderInline(c, b)
	}
}

func childrenInlineString(n *html.Node) string {
	var b strings.Builder
	childrenInline(n, &b)
	return b.String()
}

// linkTarget extracts a referenced title from <ri:page>/<ri:attachment> children.
func linkTarget(n *html.Node) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if v := attrNS(c, "content-title"); v != "" {
			return v
		}
		if v := attrNS(c, "filename"); v != "" {
			return v
		}
	}
	return ""
}

// textContent returns the concatenated plain text of a subtree.
func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(nd *html.Node) {
		if nd.Type == html.TextNode {
			b.WriteString(nd.Data)
			return
		}
		for c := nd.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// listLines renders <ul>/<ol> children as markdown list lines.
func listLines(n *html.Node, ordered bool) string {
	var lines []string
	idx := 1
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || !strings.EqualFold(c.Data, "li") {
			continue
		}
		marker := "- "
		if ordered {
			marker = strconv.Itoa(idx) + ". "
			idx++
		}
		lines = append(lines, marker+strings.TrimSpace(inlineText(c)))
	}
	return strings.Join(lines, "\n")
}

// tableText renders a <table> as pipe-delimited rows.
func tableText(n *html.Node) string {
	var rows []string
	var walk func(*html.Node)
	walk = func(nd *html.Node) {
		for c := nd.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && strings.EqualFold(c.Data, "tr") {
				var cells []string
				for cell := c.FirstChild; cell != nil; cell = cell.NextSibling {
					if cell.Type == html.ElementNode &&
						(strings.EqualFold(cell.Data, "td") || strings.EqualFold(cell.Data, "th")) {
						cells = append(cells, strings.TrimSpace(inlineText(cell)))
					}
				}
				if len(cells) > 0 {
					rows = append(rows, "| "+strings.Join(cells, " | ")+" |")
				}
			} else {
				walk(c)
			}
		}
	}
	walk(n)
	return strings.Join(rows, "\n")
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

var (
	tagRe   = regexp.MustCompile(`<[^>]+>`)
	spaceRe = regexp.MustCompile(`[ \t]+`)
)

func stripTags(s string) string { return tagRe.ReplaceAllString(s, "") }

func collapseSpaces(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(strings.ReplaceAll(s, " ", " "), " "))
}
