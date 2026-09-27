// Package handler wires routes, builds page data and serves responses.
package handler

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/render"
	"github.com/konorlevich/fitto-club/internal/site"
	"github.com/konorlevich/fitto-club/internal/store"
	"github.com/sirupsen/logrus"
)

const localeCookie = "lang"

// Server holds everything a request needs. All of it is built once at boot.
type Server struct {
	Cfg    site.Config
	Copy   map[string]*content.SiteCopy
	Tmpl   render.Templates
	Static fs.FS
	Assets map[string]string
	Store  store.Store
	Log    *logrus.Logger
	// Inline assets, prepared at boot (assets.go).
	Inline Inline
	// Clock is the time source; tests pin it. Returns club-local time.
	Clock func() time.Time
	// Admin mounts the authoring routes when configured.
	Admin func(*http.ServeMux)

	cache *pageCache
}

// Pages is every template the server renders.
var Pages = []string{"home", "memberships", "coaches", "coach", "classes", "massage", "about", "privacy", "notfound"}

// sections are the locale-prefixed static routes: path -> template.
var sections = []struct{ Path, Tmpl string }{
	{"/memberships", "memberships"},
	{"/coaches", "coaches"},
	{"/classes", "classes"},
	{"/massage", "massage"},
	{"/about", "about"},
	{"/privacy", "privacy"},
}

var langNames = map[string]string{"en": "English", "ru": "Русский", "ka": "ქართული"}

func (s *Server) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now().In(s.Cfg.TZ)
}

// Handler builds the route table.
func (s *Server) Handler() http.Handler {
	if s.cache == nil {
		s.cache = newPageCache()
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /robots.txt", s.robots)
	mux.HandleFunc("GET /sitemap.xml", s.sitemap)
	mux.HandleFunc("GET /llms.txt", func(w http.ResponseWriter, r *http.Request) { s.llms(w, r, site.DefaultLocale) })
	mux.HandleFunc("GET /site.webmanifest", s.manifest)
	mux.HandleFunc("GET /favicon.ico", s.favicon)
	mux.Handle("GET /static/", http.StripPrefix("/static/", s.staticHandler()))

	// Root and any unmatched path: redirect "/" to a locale, 404 the rest.
	mux.HandleFunc("/", s.fallback)

	// Locales are a whitelist known at boot, so routes are registered
	// concretely: a {lang} wildcard first segment would collide with /static/.
	for _, lang := range s.Cfg.Locales {
		l := lang
		mux.HandleFunc("GET /"+l+"/{$}", s.page("home", "/"))
		// The home page's canonical form ends in a slash; /en 301s to /en/.
		mux.HandleFunc("GET /"+l, redirect("/"+l+"/"))
		for _, sec := range sections {
			mux.HandleFunc("GET /"+l+sec.Path, s.page(sec.Tmpl, sec.Path))
			mux.HandleFunc("GET /"+l+sec.Path+"/{$}", redirect("/"+l+sec.Path))
		}
		mux.HandleFunc("GET /"+l+"/coaches/{slug}", s.coachPage)
		mux.HandleFunc("GET /"+l+"/coaches/{slug}/{$}", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/"+l+"/coaches/"+r.PathValue("slug"), http.StatusMovedPermanently)
		})
		mux.HandleFunc("GET /"+l+"/llms.txt", func(w http.ResponseWriter, r *http.Request) { s.llms(w, r, l) })
	}

	if s.Admin != nil {
		s.Admin(mux)
	}
	return s.accessLog(s.security(mux))
}

func redirect(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := to
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}

// security sets the headers every response carries, and sends www to the
// apex in one hop (IA, URL strategy).
func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if host, ok := strings.CutPrefix(r.Host, "www."); ok && s.Cfg.IsProd() {
			http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusMovedPermanently)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		if s.Cfg.IsProd() {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// detectLocale: cookie -> Accept-Language -> default (checklist §5).
func (s *Server) detectLocale(r *http.Request) string {
	if c, err := r.Cookie(localeCookie); err == nil && s.Cfg.HasLocale(c.Value) {
		return c.Value
	}
	for part := range strings.SplitSeq(r.Header.Get("Accept-Language"), ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if i := strings.IndexByte(tag, '-'); i > 0 {
			tag = tag[:i]
		}
		if s.Cfg.HasLocale(tag) {
			return tag
		}
	}
	return site.DefaultLocale
}

func (s *Server) fallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Vary", "Cookie, Accept-Language")
		w.Header().Set("Cache-Control", "no-cache")
		target := "/" + s.detectLocale(r) + "/"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	seg, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	lang := seg
	if !s.Cfg.HasLocale(lang) {
		lang = s.detectLocale(r)
	}
	s.notFound(w, r, lang)
}

func (s *Server) setLocaleCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name: localeCookie, Value: lang, Path: "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		SameSite: http.SameSiteLaxMode, Secure: s.Cfg.IsProd(),
	})
}

func langFromPath(p string) string {
	return strings.SplitN(strings.TrimPrefix(p, "/"), "/", 2)[0]
}

// newPage fills everything every page shares.
func (s *Server) newPage(lang, route, path string, snap *store.Snapshot, now time.Time) *render.Page {
	c := s.Copy[lang]
	p := &render.Page{
		Lang: lang, Route: route, Path: path, Cfg: s.Cfg, Copy: c, Snap: snap, Now: now,
		Assets: s.Assets, Robots: "index,follow",
		Canonical:    s.Cfg.BaseURL + "/" + lang + path,
		OGImage:      s.Cfg.BaseURL + render.AssetURL(s.Assets, "img/brand/og-"+lang+".png"),
		Status:       snap.Hours.StatusAt(now),
		VisitSurface: "orange",
		CTABar:       true,
		CSS:          s.Inline.CSS,
		JS:           s.Inline.JS,
		FontFaces:    s.Inline.Fonts,
		Preloads:     s.Inline.Preloads[lang],
	}
	for _, l := range s.Cfg.Locales {
		p.Alternates = append(p.Alternates, render.Alt{
			Lang: l, Name: langNames[l],
			URL:  s.Cfg.BaseURL + "/" + l + path,
			Href: "/" + l + path,
		})
	}
	active := func(prefix string) bool { return path == prefix || strings.HasPrefix(path, prefix+"/") }
	p.Nav = []render.NavItem{
		{Label: c.Nav.Memberships, Href: "/" + lang + "/memberships", Active: active("/memberships")},
		{Label: c.Nav.Coaches, Href: "/" + lang + "/coaches", Active: active("/coaches")},
		{Label: c.Nav.Classes, Href: "/" + lang + "/classes", Active: active("/classes")},
		{Label: c.Nav.Massage, Href: "/" + lang + "/massage", Active: active("/massage")},
		{Label: c.Nav.About, Href: "/" + lang + "/about", Active: active("/about")},
	}
	return p
}

func (s *Server) page(name, path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := langFromPath(r.URL.Path)
		snap := s.Store.Current()
		now := s.now()

		// /coaches takes two optional filters; everything else takes none.
		// A query we do not understand is dropped with a 301 rather than
		// served as a second URL for the same page.
		var speaks, tag string
		if name == "coaches" {
			var redirectTo string
			speaks, tag, redirectTo = canonicalFilter(r.URL.RawQuery, snap, "/"+lang+"/coaches")
			if redirectTo != "" {
				http.Redirect(w, r, redirectTo, http.StatusMovedPermanently)
				return
			}
		}
		key := cacheKey(r.URL.Path, speaks, tag, snap, now)
		if s.serveCached(w, r, key, lang) {
			return
		}

		p := s.newPage(lang, name, path, snap, now)
		c := s.Copy[lang]
		switch name {
		case "home":
			p.Title, p.Desc = c.Home.Title, c.Home.Description
			p.Canonical = s.Cfg.BaseURL + "/" + lang + "/"
			pub := snap.Published()
			p.TotalCoaches = len(pub)
			if len(pub) > 4 {
				pub = pub[:4]
			}
			p.Coaches = pub
			p.Reviews = snap.HomeReviews(lang)
		case "memberships":
			p.Title, p.Desc = c.Memberships.Title, c.Memberships.Description
			p.VisitSurface = "orange"
		case "coaches":
			p.Title, p.Desc = c.Coaches.Title, c.Coaches.Description
			p.FilterSpeaks, p.FilterTag = speaks, tag
			p.Coaches = snap.Filter(speaks, tag)
			p.TotalCoaches = len(snap.Published())
			if speaks != "" || tag != "" {
				// Filtered views are for people, not the index: the
				// canonical stays on the clean list (IA, URL strategy).
				p.Robots = "noindex,follow"
			}
		case "classes":
			p.Title, p.Desc = c.Classes.Title, c.Classes.Description
		case "massage":
			p.Title, p.Desc = c.Massage.Title, c.Massage.Description
			p.VisitSurface = "black"
		case "about":
			p.Title, p.Desc = c.About.Title, c.About.Description
			p.VisitSurface = "black"
		case "privacy":
			p.Title, p.Desc = c.Privacy.Title, c.Privacy.Description
			p.Robots = "noindex,follow"
			p.NoVisit, p.CTABar = true, false
		}
		p.JSONLD = s.jsonLD(p)
		s.setLocaleCookie(w, lang)
		s.renderAndServe(w, r, key, name, p, http.StatusOK)
	}
}

// canonicalFilter validates the /coaches query. It returns the filters, or a
// redirect target when the query is not in its one canonical form: unknown
// parameters or values are dropped, and speaks always precedes tag.
// Campaign parameters (utm_*, gclid...) are left alone and carried over, so
// a redirect never eats attribution; the canonical link strips them anyway.
func canonicalFilter(raw string, snap *store.Snapshot, base string) (speaks, tag, redirectTo string) {
	clean := true
	var filterPairs, tracking []string
	for pair := range strings.SplitSeq(raw, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		switch {
		case isTracking(k):
			tracking = append(tracking, pair)
		case k == "speaks" && speaks == "":
			speaks, _ = url.QueryUnescape(v)
			filterPairs = append(filterPairs, pair)
		case k == "tag" && tag == "":
			tag, _ = url.QueryUnescape(v)
			filterPairs = append(filterPairs, pair)
		default:
			clean = false
		}
	}
	if speaks != "" && !slices.Contains(snap.LanguagesInUse(), speaks) {
		speaks, clean = "", false
	}
	if tag != "" {
		if _, ok := snap.Tag(tag); !ok || !tagInUse(snap, tag) {
			tag, clean = "", false
		}
	}
	var want []string
	if speaks != "" {
		want = append(want, "speaks="+url.QueryEscape(speaks))
	}
	if tag != "" {
		want = append(want, "tag="+url.QueryEscape(tag))
	}
	if clean && strings.Join(filterPairs, "&") == strings.Join(want, "&") {
		return speaks, tag, ""
	}
	q := strings.Join(append(want, tracking...), "&")
	if q == "" {
		return speaks, tag, base
	}
	return speaks, tag, base + "?" + q
}

func tagInUse(snap *store.Snapshot, slug string) bool {
	for _, t := range snap.TagsInUse() {
		if t.Slug == slug {
			return true
		}
	}
	return false
}

func isTracking(k string) bool {
	switch k {
	case "gclid", "fbclid", "gbraid", "wbraid", "msclkid", "_ga", "_gl", "ref":
		return true
	}
	return strings.HasPrefix(k, "utm_")
}

func (s *Server) coachPage(w http.ResponseWriter, r *http.Request) {
	lang, slug := langFromPath(r.URL.Path), r.PathValue("slug")
	snap := s.Store.Current()

	// A renamed slug answers with one hop to the current URL.
	if cur, ok := snap.SlugHistory[slug]; ok && cur != slug {
		if c, ok := snap.Coach(cur); ok && c.Published {
			http.Redirect(w, r, "/"+lang+"/coaches/"+cur, http.StatusMovedPermanently)
			return
		}
		slug = cur
	}
	coach, ok := snap.Coach(slug)
	if !ok {
		// Never existed: a real 404, not a redirect (soft-404 otherwise).
		s.notFound(w, r, lang)
		return
	}
	if !coach.Published {
		// Unpublished (usually: left the club). 301 to the list, marked
		// no-cache so a browser does not remember it forever if the coach
		// is published again (BRIEF.md §4).
		w.Header().Set("Cache-Control", "no-cache")
		http.Redirect(w, r, "/"+lang+"/coaches", http.StatusMovedPermanently)
		return
	}

	now := s.now()
	key := cacheKey(r.URL.Path, "", "", snap, now)
	if s.serveCached(w, r, key, lang) {
		return
	}
	p := s.newPage(lang, "coach", "/coaches/"+slug, snap, now)
	p.Coach = &coach
	p.Title = p.T(coach.Name) + " — " + s.Copy[lang].Coach.TitleSuffix
	p.Desc = truncate(p.T(coach.Bio), 155)
	p.Reviews = snap.ReviewsFor(slug)
	p.CoachClasses = snap.ClassesBy(slug)
	p.Similar = snap.SimilarCoaches(slug, 3)
	p.JSONLD = s.jsonLD(p)
	s.setLocaleCookie(w, lang)
	s.renderAndServe(w, r, key, "coach", p, http.StatusOK)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request, lang string) {
	if !s.Cfg.HasLocale(lang) {
		lang = site.DefaultLocale
	}
	snap := s.Store.Current()
	p := s.newPage(lang, "notfound", "/404", snap, s.now())
	c := s.Copy[lang]
	p.Title, p.Desc = c.NotFound.Title, c.NotFound.Description
	p.Robots, p.Canonical = "noindex,follow", ""
	p.Alternates = nil
	p.NoVisit, p.CTABar = true, false
	for _, l := range s.Cfg.Locales {
		p.Alternates = append(p.Alternates, render.Alt{Lang: l, Name: langNames[l], Href: "/" + l + "/"})
	}
	p.JSONLD = s.jsonLD(p)
	s.renderAndServe(w, r, "", "notfound", p, http.StatusNotFound)
}

// truncate cuts at a word boundary and never mid-rune.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	cut := string(r[:n-1])
	if i := strings.LastIndexByte(cut, ' '); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}
