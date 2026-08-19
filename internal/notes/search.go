package notes

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// FoldSearchText normalizes case and removes canonical combining marks for
// case- and accent-insensitive matching. It does not alter persisted text.
func FoldSearchText(text string) string {
	decomposed := norm.NFD.String(text)
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return unicode.ToLower(r)
	}, decomposed)
}
