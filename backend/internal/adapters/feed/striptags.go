package feed

import (
	"html"
	"strings"
	"unicode"
)

// stripTags reduces a feed entry's HTML body to plain text for preview and status parsing.
//
// Hand-rolled rather than pulled from a dependency: this is a cosmetic transform on text we
// never render as markup (the UI prints it as text), so a tag-skipping scan plus entity
// unescaping is enough, and it keeps the dependency list honest. It is emphatically NOT a
// sanitiser — nothing here makes untrusted HTML safe to inject.
func stripTags(markup string) string {
	if markup == "" {
		return ""
	}

	var out strings.Builder
	out.Grow(len(markup))

	depth := 0
	for _, r := range markup {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
				// A tag boundary separates words: "<p>a</p><p>b</p>" must not become "ab".
				out.WriteRune(' ')
			}
		case depth == 0:
			out.WriteRune(r)
		}
	}

	return collapseSpaces(html.UnescapeString(out.String()))
}

// collapseSpaces squeezes every run of whitespace — including the newlines feeds love — into
// a single space, and trims the ends.
func collapseSpaces(s string) string {
	var out strings.Builder
	out.Grow(len(s))

	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && out.Len() > 0 {
			out.WriteRune(' ')
		}
		space = false
		out.WriteRune(r)
	}
	return out.String()
}
