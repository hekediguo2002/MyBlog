package markdownx

import (
	"regexp"
	"strings"
)

var (
	reFence   = regexp.MustCompile("(?s)```.*?```")
	reInline  = regexp.MustCompile("`[^`]*`")
	reImage   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reLink    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHeading = regexp.MustCompile(`(?m)^#{1,6}\s*`)
	reEmph    = regexp.MustCompile(`[*_]{1,3}`)
	reQuote   = regexp.MustCompile(`(?m)^>\s?`)
	reList    = regexp.MustCompile(`(?m)^([*+\-]|\d+\.)\s+`)
	reSpaces  = regexp.MustCompile(`\s+`)
)

func Summary(content string, maxRunes int) string {
	s := content
	s = reFence.ReplaceAllString(s, "")
	s = reImage.ReplaceAllString(s, "")
	s = reLink.ReplaceAllString(s, "$1")
	s = reInline.ReplaceAllString(s, "")
	s = reHeading.ReplaceAllString(s, "")
	s = reEmph.ReplaceAllString(s, "")
	s = reQuote.ReplaceAllString(s, "")
	s = reList.ReplaceAllString(s, "")
	s = reSpaces.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > maxRunes {
		r = r[:maxRunes]
	}
	return string(r)
}
