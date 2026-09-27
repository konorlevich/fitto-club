package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"regexp"
	"strings"

	"github.com/konorlevich/fitto-club/internal/render"
)

// Fingerprints hashes every embedded static file at boot so assets can be
// served under a content-addressed path with an immutable cache header
// (checklist §2). A changed file gets a new URL, never a stale cache hit.
func Fingerprints(fsys fs.FS) (map[string]string, error) {
	out := map[string]string{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[p] = hex.EncodeToString(sum[:5])
		return nil
	})
	return out, err
}

// staticHandler serves versioned paths as immutable, bare paths revalidated.
func (s *Server) staticHandler() http.Handler {
	fsrv := http.FileServer(http.FS(s.Static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versioned := false
		if rest, ok := strings.CutPrefix(r.URL.Path, "v/"); ok {
			if i := strings.IndexByte(rest, '/'); i > 0 {
				r.URL.Path = rest[i+1:]
				versioned = true
			}
		}
		switch {
		case strings.HasSuffix(r.URL.Path, ".woff2"):
			w.Header().Set("Content-Type", "font/woff2")
		case strings.HasSuffix(r.URL.Path, ".webp"):
			w.Header().Set("Content-Type", "image/webp")
		case strings.HasSuffix(r.URL.Path, ".webmanifest"):
			w.Header().Set("Content-Type", "application/manifest+json")
		}
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r) // no directory listings
			return
		}
		if versioned {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		}
		fsrv.ServeHTTP(w, r)
	})
}

func (s *Server) favicon(w http.ResponseWriter, r *http.Request) {
	b, err := fs.ReadFile(s.Static, "img/brand/favicon.ico")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(b)
}

// Inline is the CSS, JS and @font-face block inlined into every page. Inlined
// rather than linked: the whole stylesheet is small, and inlining removes the
// render-blocking request that costs more than the bytes (checklist §1).
type Inline struct {
	CSS      template.CSS
	JS       template.JS
	Fonts    template.CSS
	Preloads map[string][]string
}

// Unicode ranges: must match tools/assets/build_fonts.py.
const (
	rLatin    = "U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+20AC,U+20BE,U+2122,U+2190-2193,U+2212,U+2215,U+2248,U+2605,U+2713,U+FEFF,U+FFFD"
	rCyrillic = "U+0400-045F,U+0490-0491,U+04B0-04B1,U+2116"
	rGeorgian = "U+10A0-10FF,U+1C90-1CBF,U+2D00-2D2F"
	rNumeric  = "U+0020-007E,U+00A0,U+2009,U+2013-2014,U+2248"
)

var urlRe = regexp.MustCompile(`url\("?/static/([^")]+)"?\)`)

// BuildInline prepares the inline assets once at boot. Any missing file is a
// boot failure.
func BuildInline(fsys fs.FS, assets map[string]string) (Inline, error) {
	var css strings.Builder
	for _, f := range []string{"css/tokens.css", "css/site.css"} {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return Inline{}, fmt.Errorf("inline css: %w", err)
		}
		css.Write(b)
		css.WriteByte('\n')
	}
	// Point url(/static/...) at the fingerprinted path.
	out := urlRe.ReplaceAllStringFunc(css.String(), func(m string) string {
		sub := urlRe.FindStringSubmatch(m)
		return `url("` + render.AssetURL(assets, sub[1]) + `")`
	})
	js, err := fs.ReadFile(fsys, "js/app.js")
	if err != nil {
		return Inline{}, fmt.Errorf("inline js: %w", err)
	}

	font := func(file string) string { return render.AssetURL(assets, "fonts/"+file) }
	faceD := func(family, weight, file, rng, display string) string {
		return fmt.Sprintf(`@font-face{font-family:%q;font-style:normal;font-weight:%s;font-display:%s;src:url(%q) format("woff2");unicode-range:%s}`,
			family, weight, display, font(file), rng)
	}
	face := func(family, weight, file, rng string) string { return faceD(family, weight, file, rng, "swap") }
	for _, f := range []string{"jost-latin.woff2", "jost-cyrillic.woff2", "archivo-black-numeric.woff2",
		"firago-400-latin.woff2", "firago-400-cyrillic.woff2", "firago-400-georgian.woff2",
		"firago-600-latin.woff2", "firago-600-cyrillic.woff2", "firago-600-georgian.woff2",
		"noto-sans-georgian.woff2"} {
		if _, ok := assets["fonts/"+f]; !ok {
			return Inline{}, fmt.Errorf("font %s is missing; run tools/assets/build_fonts.py", f)
		}
	}
	fonts := strings.Join([]string{
		face("Jost", "400 600", "jost-latin.woff2", rLatin),
		face("Jost", "400 600", "jost-cyrillic.woff2", rCyrillic),
		face("Archivo Black", "400", "archivo-black-numeric.woff2", rNumeric),
		face("FiraGO", "400", "firago-400-latin.woff2", rLatin),
		face("FiraGO", "400", "firago-400-cyrillic.woff2", rCyrillic),
		faceD("FiraGO", "400", "firago-400-georgian.woff2", rGeorgian, "optional"),
		face("FiraGO", "600", "firago-600-latin.woff2", rLatin),
		face("FiraGO", "600", "firago-600-cyrillic.woff2", rCyrillic),
		faceD("FiraGO", "600", "firago-600-georgian.woff2", rGeorgian, "optional"),
		// Every Georgian face is "optional", not "swap". They are preloaded
		// on /ka/, so they almost always arrive in the optional window; when
		// they do not, the text stays in the system Georgian face for that
		// view instead of re-wrapping and pushing the hero photo down (CLS
		// 0.019 measured on /ka/ with swap). Every phone ships a Georgian
		// system face, so the fallback is readable, just not FiraGO.
		faceD("Noto Sans Georgian", "500 700", "noto-sans-georgian.woff2", rGeorgian, "optional"),
	}, "")

	return Inline{
		CSS:   template.CSS(minifyCSS(out)),
		JS:    template.JS(minifyJS(string(js))),
		Fonts: template.CSS(fonts),
		// Two above-the-fold faces per locale: the display face of the
		// hero and the body face, in the page's own script.
		Preloads: map[string][]string{
			"en": {font("jost-latin.woff2"), font("firago-400-latin.woff2")},
			"ru": {font("jost-cyrillic.woff2"), font("firago-400-cyrillic.woff2")},
			// Georgian also preloads the 600 face: the hero buttons use it,
			// and a late swap there re-wrapped them and nudged the photo
			// (Lighthouse CLS 0.019 on /ka/).
			"ka": {font("noto-sans-georgian.woff2"), font("firago-400-georgian.woff2"), font("firago-600-georgian.woff2")},
		},
	}, nil
}

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssSpace   = regexp.MustCompile(`\s+`)
	cssPunct   = regexp.MustCompile(`\s*([{};,>])\s*`)
	cssColon   = regexp.MustCompile(`([{;])\s*([a-z-]+)\s*:\s*`)
)

// minifyCSS is deliberately conservative: comments and whitespace only.
func minifyCSS(s string) string {
	s = cssComment.ReplaceAllString(s, "")
	s = cssSpace.ReplaceAllString(s, " ")
	s = cssPunct.ReplaceAllString(s, "$1")
	s = cssColon.ReplaceAllString(s, "$1$2:")
	return strings.TrimSpace(s)
}

var jsBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// minifyJS strips block comments and indentation. It does not touch line
// comments or string contents, so it cannot change behaviour; app.js uses
// only block comments.
func minifyJS(s string) string {
	s = jsBlockComment.ReplaceAllString(s, "")
	var b strings.Builder
	for line := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			b.WriteString(t)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
