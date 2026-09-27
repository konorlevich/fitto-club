// Package admin is the club's own CMS under /admin: server-rendered forms
// over the same store the public pages read from (BRIEF.md §4, checklist
// §13). Every write is an ordinary POST answered with a 303; JavaScript only
// adds conveniences. Design decisions: .design/fitto-admin/.
package admin

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/render"
	"github.com/konorlevich/fitto-club/internal/site"
	"github.com/konorlevich/fitto-club/internal/store"
	"github.com/sirupsen/logrus"
)

const (
	sessionCookie = "admin_session"
	langCookie    = "admin_lang"
	themeCookie   = "admin_theme"
	flashCookie   = "admin_flash"
	defaultLang   = "ru"
)

// UILangs are the languages the admin interface itself speaks.
var UILangs = []string{"ru", "ka"}

// Server holds everything an admin request needs, built once at boot.
type Server struct {
	Cfg    site.Config
	Store  *store.SQLite
	Copy   map[string]content.AdminCopy
	Tmpl   map[string]*template.Template
	Assets map[string]string
	CSS    template.CSS
	JS     template.JS
	Fonts  template.CSS
	Log    *logrus.Logger
	Clock  func() time.Time
}

// Pages is every admin template.
var Pages = []string{"login", "setup", "today", "error", "hours", "special", "reviews", "review", "coaches", "coach", "tags", "tag", "classes", "class", "slot", "schedule", "memberships", "membership", "single", "massage", "massage_form", "history", "users", "user", "profile"}

// New parses the admin templates and inlines its CSS and JS.
func New(cfg site.Config, st *store.SQLite, copies map[string]content.AdminCopy, templates, static fs.FS,
	assets map[string]string, fonts template.CSS, log *logrus.Logger) (*Server, error) {
	tmpl := map[string]*template.Template{}
	for _, name := range Pages {
		t, err := template.New(name).Funcs(funcs()).ParseFS(templates,
			"web/templates/admin/layout.html",
			"web/templates/admin/partials/*.html",
			"web/templates/admin/pages/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("admin template %q: %w", name, err)
		}
		tmpl[name] = t
	}
	var css strings.Builder
	for _, f := range []string{"css/tokens.css", "css/admin-tokens.css", "css/admin.css"} {
		b, err := fs.ReadFile(static, f)
		if err != nil {
			return nil, fmt.Errorf("admin css: %w", err)
		}
		css.Write(b)
		css.WriteByte('\n')
	}
	js, err := fs.ReadFile(static, "js/admin.js")
	if err != nil {
		return nil, fmt.Errorf("admin js: %w", err)
	}
	return &Server{Cfg: cfg, Store: st, Copy: copies, Tmpl: tmpl, Assets: assets,
		CSS: template.CSS(minifyCSS(css.String(), assets)), JS: template.JS(string(js)), Fonts: fonts, Log: log}, nil
}

func funcs() template.FuncMap {
	return template.FuncMap{
		"add":       func(a, b int) int { return a + b },
		"join":      strings.Join,
		"hasPrefix": strings.HasPrefix,
		"seq": func(n int) []int {
			out := make([]int, n)
			for i := range out {
				out[i] = i
			}
			return out
		},
		"dict": func(kv ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(kv); i += 2 {
				m[fmt.Sprint(kv[i])] = kv[i+1]
			}
			return m
		},
	}
}

func (s *Server) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now().In(s.Cfg.TZ)
}

// ------------------------------------------------------------------ routes

// Mount registers the admin routes on the site's mux.
func (s *Server) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/login", s.public(s.loginForm))
	mux.HandleFunc("POST /admin/login", s.public(s.loginPost))
	mux.HandleFunc("POST /admin/logout", s.protected(s.logout))
	mux.HandleFunc("GET /admin/setup", s.protected(s.setupForm))
	mux.HandleFunc("POST /admin/setup", s.protected(s.setupPost))
	mux.HandleFunc("POST /admin/theme", s.protected(s.setTheme))
	mux.HandleFunc("POST /admin/lang", s.protected(s.setLang))
	mux.HandleFunc("GET /admin/hours", s.protected(s.hoursForm))
	mux.HandleFunc("POST /admin/hours", s.protected(s.hoursPost))
	mux.HandleFunc("GET /admin/hours/special/new", s.protected(s.specialForm))
	mux.HandleFunc("POST /admin/hours/special/new", s.protected(s.specialPost))
	mux.HandleFunc("GET /admin/hours/special/{date}", s.protected(s.specialForm))
	mux.HandleFunc("POST /admin/hours/special/{date}", s.protected(s.specialPost))
	mux.HandleFunc("POST /admin/hours/special/{date}/delete", s.protected(s.specialDelete))
	mux.HandleFunc("GET /admin/reviews", s.protected(s.reviewsList))
	mux.HandleFunc("GET /admin/reviews/new", s.protected(s.reviewForm))
	mux.HandleFunc("POST /admin/reviews/new", s.protected(s.reviewPost))
	mux.HandleFunc("GET /admin/reviews/{id}", s.protected(s.reviewForm))
	mux.HandleFunc("POST /admin/reviews/{id}", s.protected(s.reviewPost))
	mux.HandleFunc("POST /admin/reviews/{id}/delete", s.protected(s.reviewDelete))
	mux.HandleFunc("POST /admin/reviews/{id}/restore", s.protected(s.reviewRestore))
	mux.HandleFunc("GET /admin/coaches", s.protected(s.coachesList))
	mux.HandleFunc("GET /admin/coaches/new", s.protected(s.coachForm))
	mux.HandleFunc("POST /admin/coaches/new", s.protected(s.coachPost))
	mux.HandleFunc("GET /admin/tags", s.protected(s.tagsList))
	mux.HandleFunc("GET /admin/tags/new", s.protected(s.tagForm))
	mux.HandleFunc("POST /admin/tags/new", s.protected(s.tagPost))
	mux.HandleFunc("GET /admin/tags/{slug}", s.protected(s.tagForm))
	mux.HandleFunc("POST /admin/tags/{slug}", s.protected(s.tagPost))
	mux.HandleFunc("POST /admin/tags/{slug}/delete", s.protected(s.tagDelete))
	mux.HandleFunc("POST /admin/tags/{slug}/move", s.protected(s.tagMove))
	mux.HandleFunc("GET /admin/coaches/{id}", s.protected(s.coachForm))
	mux.HandleFunc("POST /admin/coaches/{id}", s.protected(s.coachPost))
	mux.HandleFunc("POST /admin/coaches/{id}/delete", s.protected(s.coachDelete))
	mux.HandleFunc("POST /admin/coaches/{id}/restore", s.protected(s.coachRestore))
	mux.HandleFunc("POST /admin/coaches/{id}/move", s.protected(s.coachMove))
	mux.HandleFunc("POST /admin/coaches/{id}/photo", s.protectedUpload(s.photoUpload))
	mux.HandleFunc("POST /admin/coaches/{id}/photo/focus", s.protected(s.photoFocus))
	mux.HandleFunc("POST /admin/coaches/{id}/photo/delete", s.protected(s.photoDelete))
	mux.HandleFunc("GET /admin/classes", s.protected(s.classesList))
	mux.HandleFunc("GET /admin/classes/new", s.protected(s.classForm))
	mux.HandleFunc("POST /admin/classes/new", s.protected(s.classPost))
	mux.HandleFunc("GET /admin/classes/{id}", s.protected(s.classForm))
	mux.HandleFunc("POST /admin/classes/{id}", s.protected(s.classPost))
	mux.HandleFunc("POST /admin/classes/{id}/delete", s.protected(s.classDelete))
	mux.HandleFunc("POST /admin/classes/{id}/restore", s.protected(s.classRestore))
	mux.HandleFunc("POST /admin/classes/{id}/move", s.protected(s.classMove))
	mux.HandleFunc("GET /admin/classes/{id}/slots/{n}", s.protected(s.slotForm))
	mux.HandleFunc("POST /admin/classes/{id}/slots/{n}", s.protected(s.slotPost))
	mux.HandleFunc("POST /admin/classes/{id}/slots/{n}/delete", s.protected(s.slotDelete))
	mux.HandleFunc("GET /admin/schedule", s.protected(s.schedule))
	mux.HandleFunc("GET /admin/schedule/new", s.protected(s.slotForm))
	mux.HandleFunc("POST /admin/schedule/new", s.protected(s.slotPost))
	mux.HandleFunc("GET /admin/memberships", s.protected(s.membershipsList))
	mux.HandleFunc("GET /admin/memberships/new", s.protected(s.membershipForm))
	mux.HandleFunc("POST /admin/memberships/new", s.protected(s.membershipPost))
	mux.HandleFunc("GET /admin/memberships/single", s.protected(s.singleForm))
	mux.HandleFunc("POST /admin/memberships/single", s.protected(s.singlePost))
	mux.HandleFunc("GET /admin/memberships/{id}", s.protected(s.membershipForm))
	mux.HandleFunc("POST /admin/memberships/{id}", s.protected(s.membershipPost))
	mux.HandleFunc("POST /admin/memberships/{id}/delete", s.protected(s.membershipDelete))
	mux.HandleFunc("POST /admin/memberships/{id}/restore", s.protected(s.membershipRestore))
	mux.HandleFunc("POST /admin/memberships/{id}/move", s.protected(s.membershipMove))
	mux.HandleFunc("GET /admin/massage", s.protected(s.massageList))
	mux.HandleFunc("GET /admin/massage/new", s.protected(s.massageForm))
	mux.HandleFunc("POST /admin/massage/new", s.protected(s.massagePost))
	mux.HandleFunc("GET /admin/massage/{id}", s.protected(s.massageForm))
	mux.HandleFunc("POST /admin/massage/{id}", s.protected(s.massagePost))
	mux.HandleFunc("POST /admin/massage/{id}/delete", s.protected(s.massageDelete))
	mux.HandleFunc("POST /admin/massage/{id}/restore", s.protected(s.massageRestore))
	mux.HandleFunc("POST /admin/massage/{id}/move", s.protected(s.massageMove))
	mux.HandleFunc("GET /admin/history", s.protected(s.history))
	mux.HandleFunc("GET /admin/profile", s.protected(s.profileForm))
	mux.HandleFunc("POST /admin/profile", s.protected(s.profilePost))
	mux.HandleFunc("GET /admin/users", s.owner(s.usersList))
	mux.HandleFunc("GET /admin/users/new", s.owner(s.userForm))
	mux.HandleFunc("POST /admin/users/new", s.owner(s.userCreate))
	mux.HandleFunc("GET /admin/users/{id}", s.owner(s.userForm))
	mux.HandleFunc("POST /admin/users/{id}/reset", s.owner(s.userReset))
	mux.HandleFunc("POST /admin/users/{id}/block", s.owner(func(c *ctx) { s.userBlock(c, true) }))
	mux.HandleFunc("POST /admin/users/{id}/unblock", s.owner(func(c *ctx) { s.userBlock(c, false) }))
	mux.HandleFunc("POST /admin/export", s.owner(s.export))
	mux.HandleFunc("GET /admin/{$}", s.protected(s.today))
	mux.HandleFunc("GET /admin", s.protected(s.today))
	// Anything else under /admin: a 404 inside the shell (or the login page).
	mux.HandleFunc("/admin/", s.protected(func(c *ctx) { s.errorPage(c, http.StatusNotFound) }))
}

// ctx is one admin request.
type ctx struct {
	w     http.ResponseWriter
	r     *http.Request
	user  *store.User
	csrf  string
	lang  string
	theme string
	nonce string
	now   time.Time
}

func (c *ctx) copy(s *Server) content.AdminCopy { return s.Copy[c.lang] }

type handlerFunc func(*ctx)

// public wraps the pages that exist before a session (login).
func (s *Server) public(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := s.begin(w, r)
		if u, csrf, ok := s.session(r); ok {
			c.user, c.csrf = &u, csrf
		}
		h(c)
	}
}

// protected requires a session; an account that must still set its
// password is sent to /admin/setup from everywhere else.
func (s *Server) protected(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := s.begin(w, r)
		u, csrf, ok := s.session(r)
		if !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		c.user, c.csrf = &u, csrf
		if u.Lang != "" && r.URL.Path != "/admin/lang" {
			if _, err := r.Cookie(langCookie); err != nil {
				c.lang = s.uiLang(u.Lang)
			}
		}
		if u.Theme != "" && u.Theme != "auto" {
			c.theme = u.Theme
		}
		if u.MustChange && r.URL.Path != "/admin/setup" && r.URL.Path != "/admin/logout" {
			http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
			return
		}
		if r.Method == http.MethodPost && r.FormValue("_csrf") != csrf {
			s.errorPage(c, http.StatusForbidden)
			return
		}
		h(c)
	}
}

// protectedUpload is protected without the eager CSRF check: a multipart
// body is parsed with a size cap inside the handler, which checks the token
// itself right after.
func (s *Server) protectedUpload(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := s.begin(w, r)
		u, csrf, ok := s.session(r)
		if !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		c.user, c.csrf = &u, csrf
		if u.MustChange {
			http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
			return
		}
		h(c)
	}
}

// owner is protected plus the owner role.
func (s *Server) owner(h handlerFunc) http.HandlerFunc {
	return s.protected(func(c *ctx) {
		if !c.user.IsOwner() {
			s.errorPage(c, http.StatusForbidden)
			return
		}
		h(c)
	})
}

// begin sets the headers every admin response carries and reads the
// pre-session preferences (language and theme cookies).
func (s *Server) begin(w http.ResponseWriter, r *http.Request) *ctx {
	var b [16]byte
	_, _ = rand.Read(b[:])
	nonce := hex.EncodeToString(b[:])
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'nonce-"+nonce+"'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	c := &ctx{w: w, r: r, lang: defaultLang, nonce: nonce, now: s.now()}
	if ck, err := r.Cookie(langCookie); err == nil {
		c.lang = s.uiLang(ck.Value)
	}
	if ck, err := r.Cookie(themeCookie); err == nil && (ck.Value == "light" || ck.Value == "dark") {
		c.theme = ck.Value
	}
	return c
}

func (s *Server) uiLang(l string) string {
	for _, u := range UILangs {
		if u == l {
			return l
		}
	}
	return defaultLang
}

func (s *Server) session(r *http.Request) (store.User, string, bool) {
	ck, err := r.Cookie(sessionCookie)
	if err != nil || ck.Value == "" {
		return store.User{}, "", false
	}
	u, csrf, err := s.Store.SessionUser(ck.Value)
	if err != nil {
		return store.User{}, "", false
	}
	return u, csrf, true
}

func (s *Server) setCookie(c *ctx, name, value string, maxAge time.Duration) {
	http.SetCookie(c.w, &http.Cookie{
		Name: name, Value: value, Path: "/admin", HttpOnly: true,
		Secure: s.Cfg.IsProd(), SameSite: http.SameSiteLaxMode, MaxAge: int(maxAge.Seconds()),
	})
}

// ------------------------------------------------------------------- flash

type Flash struct {
	Kind, Text, Link, LinkText string
}

// flash queues a one-shot message for the next page (cookie, read once).
func (s *Server) flash(c *ctx, kind, text, link, linkText string) {
	v := url.Values{"k": {kind}, "t": {text}, "l": {link}, "lt": {linkText}}
	s.setCookie(c, flashCookie, url.QueryEscape(v.Encode()), time.Minute)
}

func (s *Server) takeFlash(c *ctx) *Flash {
	ck, err := c.r.Cookie(flashCookie)
	if err != nil || ck.Value == "" {
		return nil
	}
	s.setCookie(c, flashCookie, "", -1)
	raw, err := url.QueryUnescape(ck.Value)
	if err != nil {
		return nil
	}
	v, err := url.ParseQuery(raw)
	if err != nil {
		return nil
	}
	return &Flash{Kind: v.Get("k"), Text: v.Get("t"), Link: v.Get("l"), LinkText: v.Get("lt")}
}

// ------------------------------------------------------------------ render

type Link struct {
	Href, Label string
	Post        bool
}

type FieldError struct {
	Field, Text string
}

// Page is what every admin template receives.
type Page struct {
	Lang, Theme string
	C           content.AdminCopy
	User        *store.User
	CSRF        string
	Nonce       string
	Now         time.Time
	Path        string
	Section     string
	Title       string
	Back        string
	Action      *Link
	Flash       *Flash
	Errors      []FieldError
	Bare        bool // login/setup: no navigation
	Wide        bool // schedule: use the wide content width
	SiteURL     string
	Cfg         site.Config
	CSS         template.CSS
	JS          template.JS
	Fonts       template.CSS
	Assets      map[string]string
	Data        any
}

func (p *Page) T(key string) string { return p.C.T(key) }
func (p *Page) F(key string, args ...any) string {
	return fmt.Sprintf(p.C.T(key), args...)
}

// L shows a localized value in the UI language, falling back to English.
func (p *Page) L(l content.L) string { return l.Get(p.Lang) }

func (p *Page) Asset(path string) string { return render.AssetURL(p.Assets, path) }
func (p *Page) Is(section string) bool   { return p.Section == section }
func (p *Page) HasError(field string) bool {
	for _, e := range p.Errors {
		if e.Field == field {
			return true
		}
	}
	return false
}
func (p *Page) ErrorFor(field string) string {
	for _, e := range p.Errors {
		if e.Field == field {
			return e.Text
		}
	}
	return ""
}
func (p *Page) Day(iso int) string      { return p.C.T(fmt.Sprintf("day.%d", iso)) }
func (p *Page) DayShort(iso int) string { return p.C.T(fmt.Sprintf("day_short.%d", iso)) }
func (p *Page) Today() int              { return content.ISODay(p.Now) }

// When renders a changelog timestamp as club-local "Sat 27.09, 18:42".
func (p *Page) When(rfc string) string {
	t, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		return rfc
	}
	t = t.In(p.Cfg.TZ)
	return fmt.Sprintf("%s %s, %s", p.DayShort(content.ISODay(t)), t.Format("02.01"), t.Format("15:04"))
}

func (s *Server) newPage(c *ctx, section, title string) *Page {
	p := &Page{Lang: c.lang, Theme: c.theme, C: c.copy(s), User: c.user, CSRF: c.csrf, Nonce: c.nonce,
		Now: c.now, Path: c.r.URL.Path, Section: section, Title: title,
		SiteURL: "/" + s.siteLang(c) + "/", Cfg: s.Cfg, CSS: s.CSS, JS: s.JS, Fonts: s.Fonts, Assets: s.Assets}
	if c.r.Method == http.MethodGet {
		p.Flash = s.takeFlash(c)
	}
	return p
}

// siteLang maps the UI language to the public locale to link to.
func (s *Server) siteLang(c *ctx) string {
	if s.Cfg.HasLocale(c.lang) {
		return c.lang
	}
	return site.DefaultLocale
}

func (s *Server) render(c *ctx, name string, p *Page, status int) {
	t, ok := s.Tmpl[name]
	if !ok {
		s.Log.WithField("template", name).Error("unknown admin template")
		http.Error(c.w, "internal error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		s.Log.WithError(err).WithField("template", name).Error("admin template failed")
		http.Error(c.w, "internal error", http.StatusInternalServerError)
		return
	}
	h := c.w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Length", fmt.Sprint(buf.Len()))
	c.w.WriteHeader(status)
	_, _ = c.w.Write(buf.Bytes())
}

func (s *Server) errorPage(c *ctx, status int) {
	p := s.newPage(c, "", "")
	p.Bare = c.user == nil
	p.Data = status
	p.Title = p.T(fmt.Sprintf("error.%d.title", status))
	s.render(c, "error", p, status)
}

// redirect answers a POST with 303 to a safe admin path.
func (s *Server) redirect(c *ctx, to string) {
	http.Redirect(c.w, c.r, safeBack(to, "/admin"), http.StatusSeeOther)
}

// safeBack accepts only same-site admin paths for ?back= and redirects.
func safeBack(p, def string) string {
	if p == "" || strings.HasPrefix(p, "//") || strings.ContainsAny(p, "\r\n\\") {
		return def
	}
	if p != "/admin" && !strings.HasPrefix(p, "/admin/") && !strings.HasPrefix(p, "/admin?") {
		return def
	}
	return p
}

// ---------------------------------------------------------------- settings

func (s *Server) setTheme(c *ctx) {
	th := c.r.FormValue("theme")
	if th != "light" && th != "dark" {
		th = "auto"
	}
	if th == "auto" {
		s.setCookie(c, themeCookie, "", -1)
	} else {
		s.setCookie(c, themeCookie, th, 365*24*time.Hour)
	}
	_ = s.Store.UpdateProfile(c.user.ID, c.user.Name, c.user.Lang, th)
	s.redirect(c, c.r.FormValue("back"))
}

func (s *Server) setLang(c *ctx) {
	l := s.uiLang(c.r.FormValue("lang"))
	s.setCookie(c, langCookie, l, 365*24*time.Hour)
	_ = s.Store.UpdateProfile(c.user.ID, c.user.Name, l, c.user.Theme)
	s.redirect(c, c.r.FormValue("back"))
}

// clientIP is the rate-limit key behind Railway's proxy.
func clientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if i := strings.IndexByte(ip, ','); i > 0 {
		ip = ip[:i]
	}
	if ip == "" {
		ip = r.RemoteAddr
		if i := strings.LastIndexByte(ip, ':'); i > 0 {
			ip = ip[:i]
		}
	}
	return strings.TrimSpace(ip)
}
