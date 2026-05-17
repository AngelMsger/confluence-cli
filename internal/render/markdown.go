package render

import "strings"

// renderBlocks renders a block slice to a string in the requested syntax.
func renderBlocks(blocks []Block, opt Options) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if s := renderBlock(b, opt); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

// renderBlock renders one block. Markdown and plain-text differ only in the
// decoration applied to headings, code and quotes.
func renderBlock(b Block, opt Options) string {
	markdown := opt.As != AsText
	switch b.Kind {
	case KindHeading:
		return renderHeading(b, opt, markdown)
	case KindPara:
		return b.Text
	case KindCode:
		if markdown {
			return "```" + b.Lang + "\n" + b.Text + "\n```"
		}
		return indent(b.Text, "    ")
	case KindQuote:
		if markdown {
			return prefixLines(b.Text, "> ")
		}
		return b.Text
	case KindList, KindTable:
		return b.Text
	default:
		return b.Text
	}
}

func renderHeading(b Block, opt Options, markdown bool) string {
	text := b.Text
	if opt.Detail == DetailWithIDs || opt.Detail == DetailFull {
		text += " [" + b.SectionID + "]"
	}
	if markdown {
		level := b.Level
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		return strings.Repeat("#", level) + " " + text
	}
	return strings.ToUpper(text)
}

// renderOutline renders an outline as an indented markdown list.
func renderOutline(outline []OutlineEntry) string {
	if len(outline) == 0 {
		return "(no headings)"
	}
	var lines []string
	for _, e := range outline {
		indent := strings.Repeat("  ", max(0, e.Level-1))
		lines = append(lines, indent+"- "+e.Title+" ["+e.SectionID+"]")
	}
	return strings.Join(lines, "\n")
}

func prefixLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func indent(s, pad string) string { return prefixLines(s, pad) }
