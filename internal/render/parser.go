// Package render turns Confluence "storage format" XHTML into readable text or
// markdown, with support for partial reads (outline / section / keyword).
package render

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// BlockKind classifies a parsed content block.
type BlockKind string

const (
	KindHeading BlockKind = "heading"
	KindPara    BlockKind = "para"
	KindCode    BlockKind = "code"
	KindQuote   BlockKind = "quote"
	KindList    BlockKind = "list"
	KindTable   BlockKind = "table"
)

// Block is one rendered content unit. Inline formatting inside Text is already
// markdown (bold, italics, links, code spans).
type Block struct {
	Kind  BlockKind
	Level int    // heading level (1-6) for KindHeading
	Text  string // rendered text / code / list lines
	Lang  string // language for KindCode
	// SectionID is assigned to headings after parsing (see assignSections).
	SectionID string
}

// parse converts storage-format XHTML into a flat slice of Blocks. It also
// returns notes describing content that markdown/text rendering cannot
// faithfully represent (see lossNotes).
func parse(storage string) ([]Block, []string) {
	// Confluence wraps macro code in CDATA, which the HTML parser would drop;
	// unwrap it so the inner text survives as text nodes.
	src := strings.NewReplacer("<![CDATA[", "", "]]>", "").Replace(storage)

	root, err := html.Parse(strings.NewReader("<html><body>" + src + "</body></html>"))
	if err != nil {
		return []Block{{Kind: KindPara, Text: strings.TrimSpace(stripTags(storage))}}, nil
	}
	body := findBody(root)
	if body == nil {
		return nil, nil
	}
	var blocks []Block
	walkBlocks(body, &blocks)
	return blocks, lossNotes(body)
}

// lossNotes walks the parsed tree and reports content that markdown/text
// rendering drops or degrades: structured macros without a native rendering,
// and images (shown only as a placeholder). Each kind is reported once.
func lossNotes(root *html.Node) []string {
	seen := map[string]bool{}
	var notes []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "ac:structured-macro":
				name := attrNS(n, "name")
				// code/noformat macros render losslessly as code blocks.
				if name != "" && name != "code" && name != "noformat" && !seen["macro:"+name] {
					seen["macro:"+name] = true
					notes = append(notes, "unrendered macro: "+name+" (use --as raw to see the source)")
				}
			case "ac:image":
				if !seen["image"] {
					seen["image"] = true
					notes = append(notes, "an image is shown only as a placeholder (use --as raw to see the source)")
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return notes
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Body {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b := findBody(c); b != nil {
			return b
		}
	}
	return nil
}

// walkBlocks emits a Block for each block-level element encountered.
func walkBlocks(n *html.Node, out *[]Block) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch headingLevel(c.Data) {
		case 0:
			// not a heading
		default:
			*out = append(*out, Block{
				Kind:  KindHeading,
				Level: headingLevel(c.Data),
				Text:  strings.TrimSpace(inlineText(c)),
			})
			continue
		}
		switch strings.ToLower(c.Data) {
		case "p":
			if t := strings.TrimSpace(inlineText(c)); t != "" {
				*out = append(*out, Block{Kind: KindPara, Text: t})
			}
		case "pre":
			*out = append(*out, Block{Kind: KindCode, Text: textContent(c)})
		case "blockquote":
			*out = append(*out, Block{Kind: KindQuote, Text: strings.TrimSpace(inlineText(c))})
		case "ul", "ol":
			ordered := strings.EqualFold(c.Data, "ol")
			if lines := listLines(c, ordered); lines != "" {
				*out = append(*out, Block{Kind: KindList, Text: lines})
			}
		case "table":
			*out = append(*out, Block{Kind: KindTable, Text: tableText(c)})
		case "ac:structured-macro":
			if b, ok := macroBlock(c); ok {
				*out = append(*out, b)
			} else {
				walkBlocks(c, out)
			}
		default:
			// Containers (div, section, ac:layout, ac:rich-text-body, ...).
			walkBlocks(c, out)
		}
	}
}

func headingLevel(tag string) int {
	t := strings.ToLower(tag)
	if len(t) == 2 && t[0] == 'h' && t[1] >= '1' && t[1] <= '6' {
		return int(t[1] - '0')
	}
	return 0
}

// macroBlock renders a known Confluence macro. The boolean reports whether the
// macro produced a standalone block (false means the caller should recurse).
func macroBlock(n *html.Node) (Block, bool) {
	name := attrNS(n, "name")
	switch name {
	case "code", "noformat":
		lang := macroParam(n, "language")
		code := macroPlainBody(n)
		return Block{Kind: KindCode, Text: code, Lang: lang}, true
	default:
		return Block{}, false
	}
}

// macroParam returns an <ac:parameter ac:name="..."> value.
func macroParam(n *html.Node, name string) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, "ac:parameter") &&
			attrNS(c, "name") == name {
			return strings.TrimSpace(textContent(c))
		}
	}
	return ""
}

// macroPlainBody returns the text inside <ac:plain-text-body> / <ac:rich-text-body>.
func macroPlainBody(n *html.Node) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode &&
			(strings.EqualFold(c.Data, "ac:plain-text-body") ||
				strings.EqualFold(c.Data, "ac:rich-text-body")) {
			return strings.Trim(textContent(c), "\n")
		}
	}
	return ""
}

// attrNS returns the value of an attribute, ignoring any "ac:"/"ri:" namespace
// prefix on the attribute name.
func attrNS(n *html.Node, name string) string {
	for _, a := range n.Attr {
		key := a.Key
		if i := strings.IndexByte(key, ':'); i >= 0 {
			key = key[i+1:]
		}
		if key == name {
			return a.Val
		}
	}
	return ""
}
