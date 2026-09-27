package handler

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/render"
	"github.com/konorlevich/fitto-club/internal/site"
	"github.com/konorlevich/fitto-club/internal/store"
	"github.com/sirupsen/logrus"
)

var tz = time.FixedZone("Asia/Tbilisi", 4*3600)

// newServer builds the real handler over the repo's files and the seed.
// The clock is pinned to Sunday 2026-09-27 19:00 unless a test moves it.
func newServer(t *testing.T, now time.Time) http.Handler {
	t.Helper()
	root := os.DirFS("../..")
	cfg := site.Config{BaseURL: "https://fitto.club", Locales: []string{"en", "ru", "ka"}, Env: "test", TZ: tz}
	copies := map[string]*content.SiteCopy{}
	for _, l := range cfg.Locales {
		c, err := content.LoadCopy(root, l)
		if err != nil {
			t.Fatal(err)
		}
		copies[l] = c
	}
	tmpl, err := render.Parse(root, Pages)
	if err != nil {
		t.Fatal(err)
	}
	static, _ := fs.Sub(root, "web/static")
	assets, err := Fingerprints(static)
	if err != nil {
		t.Fatal(err)
	}
	inline, err := BuildInline(static, assets)
	if err != nil {
		t.Fatal(err)
	}
	log := logrus.New()
	log.SetOutput(io.Discard)
	s := &Server{Cfg: cfg, Copy: copies, Tmpl: tmpl, Static: static, Assets: assets, Log: log,
		Store: store.NewMemory(store.SeedSnapshot()), Inline: inline, Clock: func() time.Time { return now }}
	return s.Handler()
}

var sunday19 = time.Date(2026, 9, 27, 19, 0, 0, 0, tz)

func get(t *testing.T, h http.Handler, url string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRedirectsAndStatuses(t *testing.T) {
	h := newServer(t, sunday19)
	cases := []struct {
		url, al  string
		code     int
		location string
	}{
		{"/", "ru-RU,ru;q=0.9", 302, "/ru/"},
		{"/", "de-DE", 302, "/en/"},
		{"/en", "", 301, "/en/"},
		{"/en/coaches/", "", 301, "/en/coaches"},
		{"/en/coaches?tag=bjj&speaks=en", "", 301, "/en/coaches?speaks=en&tag=bjj"},
		{"/en/coaches?tag=nope", "", 301, "/en/coaches"},
		{"/en/coaches?speaks=zz&utm_source=ig", "", 301, "/en/coaches?utm_source=ig"},
		{"/en/coaches?utm_source=ig", "", 200, ""},
		{"/en/coaches/lizi-gagnidze", "", 301, "/en/coaches"},
		{"/en/coaches/nobody", "", 404, ""},
		{"/de/", "", 404, ""},
		{"/en/nope", "", 404, ""},
		{"/ka/memberships", "", 200, ""},
	}
	for _, c := range cases {
		rec := get(t, h, c.url, "Accept-Language", c.al)
		if rec.Code != c.code || (c.location != "" && rec.Header().Get("Location") != c.location) {
			t.Errorf("%s: got %d %q, want %d %q", c.url, rec.Code, rec.Header().Get("Location"), c.code, c.location)
		}
	}
	// An unpublished coach's 301 must not be cached forever (BRIEF.md §4).
	if cc := get(t, h, "/en/coaches/lizi-gagnidze").Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("unpublished coach redirect Cache-Control = %q", cc)
	}
}

var h1Re = regexp.MustCompile(`<h1[\s>]`)

// Every page in every locale renders, has one h1, a lang attribute, no
// template formatting accidents, and no external link without rel.
func TestAllPagesRender(t *testing.T) {
	h := newServer(t, sunday19)
	paths := []string{"/", "/memberships", "/coaches", "/coaches?speaks=ru", "/coaches/otar-chkadua",
		"/coaches/evan-kostylev", "/coaches/gocha-butbaia", "/classes", "/massage", "/about", "/privacy"}
	for _, lang := range []string{"en", "ru", "ka"} {
		for _, p := range paths {
			url := "/" + lang + p
			rec := get(t, h, url)
			body := rec.Body.String()
			if rec.Code != 200 {
				t.Errorf("%s: %d", url, rec.Code)
				continue
			}
			if n := len(h1Re.FindAllString(body, -1)); n != 1 {
				t.Errorf("%s: %d <h1>", url, n)
			}
			if !strings.Contains(body, `<html lang="`+lang+`">`) {
				t.Errorf("%s: missing html lang", url)
			}
			for _, bad := range []string{"%!", "<no value>", "MISSING"} {
				if strings.Contains(body, bad) {
					t.Errorf("%s: contains %q", url, bad)
				}
			}
			for _, tag := range regexp.MustCompile(`<a [^>]*target="_blank"[^>]*>`).FindAllString(body, -1) {
				if !strings.Contains(tag, `rel="noopener noreferrer"`) {
					t.Errorf("%s: external link without rel: %s", url, tag)
				}
			}
		}
	}
}

// TASKS #8: the reviews block exists only when the coach has reviews.
func TestCoachReviewsBlock(t *testing.T) {
	h := newServer(t, sunday19)
	otar := get(t, h, "/en/coaches/otar-chkadua").Body.String()
	if !strings.Contains(otar, `id="reviews"`) || strings.Count(otar, `class="review"`) != 4 {
		t.Errorf("otar: want a reviews block with 4 reviews, got %d", strings.Count(otar, `class="review"`))
	}
	gocha := get(t, h, "/en/coaches/gocha-butbaia").Body.String()
	if strings.Contains(gocha, `id="reviews"`) || strings.Contains(gocha, "REVIEWS") {
		t.Error("gocha has no reviews: the block must not render at all")
	}
}

func TestFilteredListIsNoindex(t *testing.T) {
	h := newServer(t, sunday19)
	body := get(t, h, "/en/coaches?speaks=es").Body.String()
	if !strings.Contains(body, `content="noindex,follow"`) || !strings.Contains(body, `rel="canonical" href="https://fitto.club/en/coaches"`) {
		t.Error("filtered list must be noindex,follow with canonical on the clean list")
	}
	if strings.Count(body, `class="coach-card"`) != 1 {
		t.Errorf("only Evan speaks Spanish; got %d cards", strings.Count(body, `class="coach-card"`))
	}
	clean := get(t, h, "/en/coaches").Body.String()
	if !strings.Contains(clean, `content="index,follow"`) {
		t.Error("the clean list must be indexable")
	}
	empty := get(t, h, "/en/coaches?speaks=es&tag=bodybuilding").Body.String()
	if !strings.Contains(empty, "SHOW ALL COACHES") {
		t.Error("an empty filter must offer a way out")
	}
}

// IA flow 5: on Sunday evening the schedule list starts with Sunday.
func TestScheduleStartsToday(t *testing.T) {
	h := newServer(t, sunday19)
	body := get(t, h, "/en/classes").Body.String()
	i := strings.Index(body, `class="day day--today`)
	j := strings.Index(body, "Sunday</h3>")
	if i < 0 || j < 0 || j < i || strings.Index(body, ">Monday</h3>") < j {
		t.Error("the first day in the list must be today (Sunday)")
	}
}

func TestStatusInFooter(t *testing.T) {
	late := time.Date(2026, 9, 27, 22, 30, 0, 0, tz) // Sunday closes at 22:00
	body := get(t, newServer(t, late), "/en/").Body.String()
	if !strings.Contains(body, "Closed. Opens tomorrow at 08:00") {
		t.Error("Sunday 22:30 must read as closed until tomorrow 08:00")
	}
	body = get(t, newServer(t, sunday19), "/ru/").Body.String()
	if !strings.Contains(body, "Открыто до 22:00") {
		t.Error("Sunday 19:00 must read as open until 22:00")
	}
}

func TestConditionalAndCompressed(t *testing.T) {
	h := newServer(t, sunday19)
	first := get(t, h, "/en/", "Accept-Encoding", "br, gzip")
	if first.Header().Get("Content-Encoding") != "br" || first.Header().Get("ETag") == "" {
		t.Fatalf("want brotli with an ETag, got %q %q", first.Header().Get("Content-Encoding"), first.Header().Get("ETag"))
	}
	again := get(t, h, "/en/", "If-None-Match", first.Header().Get("ETag"))
	if again.Code != http.StatusNotModified {
		t.Errorf("revalidation: got %d, want 304", again.Code)
	}
}

func TestSitemap(t *testing.T) {
	body := get(t, newServer(t, sunday19), "/sitemap.xml").Body.String()
	for _, want := range []string{"<loc>https://fitto.club/ka/coaches/otar-chkadua</loc>", `hreflang="x-default" href="https://fitto.club/en/massage"`,
		"<lastmod>" + content.BaselineContentDate + "</lastmod>"} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %s", want)
		}
	}
	for _, not := range []string{"lizi-gagnidze", "/privacy", "speaks="} {
		if strings.Contains(body, not) {
			t.Errorf("sitemap must not list %s", not)
		}
	}
}
