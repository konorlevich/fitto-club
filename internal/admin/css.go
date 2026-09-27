package admin

import (
	"regexp"
	"strings"

	"github.com/konorlevich/fitto-club/internal/render"
)

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssSpace   = regexp.MustCompile(`\s+`)
	cssPunct   = regexp.MustCompile(`\s*([{};,>])\s*`)
	urlRe      = regexp.MustCompile(`url\("?/static/([^")]+)"?\)`)
)

// minifyCSS strips comments and whitespace and points url(/static/...) at
// the fingerprinted asset path, like the public site's inliner.
func minifyCSS(s string, assets map[string]string) string {
	s = cssComment.ReplaceAllString(s, "")
	s = urlRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := urlRe.FindStringSubmatch(m)
		return `url("` + render.AssetURL(assets, sub[1]) + `")`
	})
	s = cssSpace.ReplaceAllString(s, " ")
	s = cssPunct.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}
