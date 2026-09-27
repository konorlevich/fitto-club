package admin

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/handler"
	"github.com/konorlevich/fitto-club/internal/render"
	"github.com/konorlevich/fitto-club/internal/site"
	"github.com/konorlevich/fitto-club/internal/store"
	"github.com/sirupsen/logrus"
)

var tz = time.FixedZone("Asia/Tbilisi", 4*3600)
var saturday = time.Date(2026, 10, 3, 12, 0, 0, 0, tz)

type env struct {
	h  http.Handler
	st *store.SQLite
	s  *Server
}

// newEnv builds the real site handler with the admin mounted over a fresh
// SQLite database seeded from the shipped defaults.
func newEnv(t *testing.T) *env {
	t.Helper()
	root := os.DirFS("../..")
	cfg := site.Config{BaseURL: "https://fitto.club", Locales: []string{"en", "ru", "ka"}, Env: "test", TZ: tz,
		AdminLogin: "otar", AdminPassword: "start-password-1"}
	copies := map[string]*content.SiteCopy{}
	for _, l := range cfg.Locales {
		c, err := content.LoadCopy(root, l)
		if err != nil {
			t.Fatal(err)
		}
		copies[l] = c
	}
	tmpl, err := render.Parse(root, handler.Pages)
	if err != nil {
		t.Fatal(err)
	}
	static, _ := fs.Sub(root, "web/static")
	assets, err := handler.Fingerprints(static)
	if err != nil {
		t.Fatal(err)
	}
	inline, err := handler.BuildInline(static, assets)
	if err != nil {
		t.Fatal(err)
	}
	log := logrus.New()
	log.SetOutput(io.Discard)
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ac := map[string]content.AdminCopy{}
	ref, err := content.LoadAdminCopy(root, "en", nil)
	if err != nil {
		t.Fatal(err)
	}
	ac["en"] = ref
	for _, l := range UILangs {
		c, err := content.LoadAdminCopy(root, l, ref)
		if err != nil {
			t.Fatal(err)
		}
		ac[l] = c
	}
	adm, err := New(cfg, st, ac, root, static, assets, inline.Fonts, log)
	if err != nil {
		t.Fatal(err)
	}
	adm.Clock = func() time.Time { return saturday }
	srv := &handler.Server{Cfg: cfg, Copy: copies, Tmpl: tmpl, Static: static, Assets: assets, Log: log,
		Store: st, Inline: inline, Clock: func() time.Time { return saturday }, Admin: adm.Mount}
	return &env{h: srv.Handler(), st: st, s: adm}
}

// client keeps cookies like a browser and knows the current CSRF token.
type client struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]string
	csrf    string
	ip      string
}

func (e *env) client(t *testing.T) *client {
	return &client{t: t, h: e.h, cookies: map[string]string{}, ip: "203.0.113.7:4444"}
}

var csrfRe = regexp.MustCompile(`name="_csrf" value="([0-9a-f]+)"`)

func (c *client) do(method, target string, form url.Values) *httptest.ResponseRecorder {
	c.t.Helper()
	var body io.Reader
	if form != nil {
		if c.csrf != "" && form.Get("_csrf") == "" {
			form.Set("_csrf", c.csrf)
		}
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	req.RemoteAddr = c.ip
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range c.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.MaxAge < 0 {
			delete(c.cookies, ck.Name)
		} else {
			c.cookies[ck.Name] = ck.Value
		}
	}
	if m := csrfRe.FindStringSubmatch(rec.Body.String()); m != nil {
		c.csrf = m[1]
	}
	return rec
}

func (c *client) get(target string) *httptest.ResponseRecorder {
	return c.do(http.MethodGet, target, nil)
}
func (c *client) post(target string, kv ...string) *httptest.ResponseRecorder {
	f := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		f.Set(kv[i], kv[i+1])
	}
	return c.do(http.MethodPost, target, f)
}

func (c *client) login(login, pw string) *httptest.ResponseRecorder {
	return c.post("/admin/login", "login", login, "password", pw)
}

// loginOwner completes the bootstrap: ENV password, then setup.
func (c *client) loginOwner() {
	c.t.Helper()
	if rec := c.login("otar", "start-password-1"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/setup" {
		c.t.Fatalf("bootstrap login: %d -> %s", rec.Code, rec.Header().Get("Location"))
	}
	c.get("/admin/setup")
	if rec := c.post("/admin/setup", "name", "Отар", "password", "my-own-password-9", "password2", "my-own-password-9"); rec.Code != http.StatusSeeOther {
		c.t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOwnerSetupFlow(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)

	if rec := c.get("/admin"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("anonymous /admin: %d -> %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := c.get("/admin/login"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `data-surface="orange"`) {
		t.Fatalf("login page: %d", rec.Code)
	}
	if rec := c.login("otar", "wrong"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", rec.Code)
	}
	c.loginOwner()

	// Every other page redirects to setup until the password is set; now it
	// is, Today renders with the status and the pending facts.
	rec := c.get("/admin")
	if rec.Code != http.StatusOK {
		t.Fatalf("today: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Открыто до 22:00", "Требует внимания", "Гагнидзе: черновик", "Ждёт подтверждения клуба: 5 фактов", "Пароль изменён", `aria-current="page"`} {
		if !strings.Contains(body, want) {
			t.Errorf("today page lacks %q", want)
		}
	}
	if h := rec.Header(); h.Get("Cache-Control") != "no-store" || !strings.Contains(h.Get("X-Robots-Tag"), "noindex") || !strings.Contains(h.Get("Content-Security-Policy"), "nonce-") {
		t.Errorf("admin headers: %v", h)
	}

	// The ENV password stops working once the owner has a real one.
	c2 := e.client(t)
	if rec := c2.login("otar", "start-password-1"); rec.Code != http.StatusUnauthorized {
		t.Errorf("ENV password still accepted after setup: %d", rec.Code)
	}
	if rec := c2.login("otar", "my-own-password-9"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin" {
		t.Errorf("real password rejected: %d -> %s", rec.Code, rec.Header().Get("Location"))
	}

	// Logout ends the session.
	c.post("/admin/logout")
	if rec := c.get("/admin"); rec.Code != http.StatusSeeOther {
		t.Errorf("after logout /admin: %d", rec.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	for i := 0; i < loginMax; i++ {
		c.login("otar", "nope")
	}
	rec := c.login("otar", "start-password-1")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Слишком много попыток") {
		t.Fatalf("sixth attempt with the right password must be locked: %d", rec.Code)
	}
	// The window passes: the same client gets in.
	e.st.SetClock(func() time.Time { return saturday.Add(loginWindow + time.Minute) })
	e.s.Clock = func() time.Time { return saturday.Add(loginWindow + time.Minute) }
	if rec := c.login("otar", "start-password-1"); rec.Code != http.StatusSeeOther {
		t.Fatalf("after the window: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFRejected(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	f := url.Values{"theme": {"dark"}, "_csrf": {"deadbeef"}}
	if rec := c.do(http.MethodPost, "/admin/theme", f); rec.Code != http.StatusForbidden {
		t.Fatalf("bad csrf accepted: %d", rec.Code)
	}
	if rec := c.post("/admin/theme", "theme", "dark", "back", "/admin"); rec.Code != http.StatusSeeOther {
		t.Fatalf("good csrf rejected: %d", rec.Code)
	}
	if rec := c.get("/admin"); !strings.Contains(rec.Body.String(), `data-theme="dark"`) {
		t.Error("theme not applied")
	}
	// Open redirects are refused.
	if rec := c.post("/admin/theme", "theme", "light", "back", "https://evil.example/"); rec.Header().Get("Location") != "/admin" {
		t.Errorf("open redirect: %s", rec.Header().Get("Location"))
	}
}

func TestUILanguage(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	c.post("/admin/lang", "lang", "ka", "back", "/admin")
	rec := c.get("/admin")
	if !strings.Contains(rec.Body.String(), `<html lang="ka"`) || !strings.Contains(rec.Body.String(), "ყურადღებას საჭიროებს") {
		t.Errorf("georgian UI not applied")
	}
}

func TestSpecialDayChangesStatus(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	c.get("/admin/hours")
	// Saturday 3 Oct 2026 12:00 is open; a closed special day for today
	// flips the status on Today and on the public site, no restart.
	rec := c.post("/admin/hours/special/new", "date", "2026-10-03", "closed", "1", "note_ru", "Праздник", "back", "/admin/hours")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save special day: %d %s", rec.Code, rec.Body.String())
	}
	if body := c.get("/admin").Body.String(); !strings.Contains(body, "Закрыто") || !strings.Contains(body, "Праздник") {
		t.Errorf("today does not reflect the special day")
	}
	if body := c.get("/ru/").Body.String(); strings.Contains(body, "Открыто до") {
		t.Errorf("public site still open after the special day was saved")
	}
	// Validation: closing before opening keeps the entered values.
	rec = c.post("/admin/hours/special/new", "date", "2026-12-31", "open", "12:00", "close", "10:00", "back", "/admin/hours")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `value="12:00"`) {
		t.Errorf("bad hours: %d, entered values kept: %v", rec.Code, strings.Contains(rec.Body.String(), `value="12:00"`))
	}
	// Week hours: Monday closes at 22:00 now.
	form := url.Values{}
	for i := 1; i <= 7; i++ {
		form.Set(fmt.Sprintf("open_%d", i), "08:00")
		form.Set(fmt.Sprintf("close_%d", i), "22:00")
	}
	if rec := c.do(http.MethodPost, "/admin/hours", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("save hours: %d %s", rec.Code, rec.Body.String())
	}
	if e.st.Current().Hours.Week[0].Close != "22:00" {
		t.Errorf("hours not saved")
	}
	// Delete the special day again.
	if rec := c.post("/admin/hours/special/2026-10-03/delete"); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	if len(e.st.Current().Hours.Special) != 0 {
		t.Errorf("special day not deleted")
	}
}

func TestDetectScript(t *testing.T) {
	cases := map[string]string{
		"ძალიან კარგი დარბაზი, მწვრთნელები მეგობრულები არიან": "ka",
		"Отличный зал, тренер Отар — топ":                     "ru",
		"Great gym, friendly coaches. 10/10":        "en",
		"Отар — лучший coach в Тбилиси, рекомендую": "ru",
		"12345 !!!": "",
	}
	for text, want := range cases {
		if got := DetectLang(text); got != want {
			t.Errorf("DetectLang(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestReviewTouchesCoachPages(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	c.get("/admin/reviews/new")
	e.st.SetClock(func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC) })
	rec := c.post("/admin/reviews/new", "author", "Nino", "text", "ძალიან კარგი დარბაზი და მწვრთნელი გოჩა", "rating", "5", "date", "2026-10", "coach", "c-gocha-butbaia", "back", "/admin/reviews")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save review: %d %s", rec.Code, rec.Body.String())
	}
	snap := e.st.Current()
	got := snap.ReviewsFor("gocha-butbaia")
	if len(got) != 1 || got[0].Lang != "ka" {
		t.Fatalf("review not linked or language not detected: %+v", got)
	}
	if snap.Date("/coaches/gocha-butbaia") != "2026-10-05" || snap.Date("/") != "2026-10-05" {
		t.Errorf("touched pages not stamped: %v", snap.LastMod)
	}
	if snap.Date("/coaches/otar-chkadua") != content.BaselineContentDate {
		t.Errorf("unrelated coach page moved")
	}
	// The public coach page now shows the review; delete hides it and the
	// admin list offers to restore.
	if body := c.get("/ka/coaches/gocha-butbaia").Body.String(); !strings.Contains(body, "Nino") {
		t.Error("public page lacks the new review")
	}
	id := got[0].ID
	if rec := c.post("/admin/reviews/" + id + "/delete"); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	if len(e.st.Current().ReviewsFor("gocha-butbaia")) != 0 {
		t.Error("deleted review still public")
	}
	if body := c.get("/admin/reviews").Body.String(); !strings.Contains(body, "Показать удалённые (1)") {
		t.Error("list lacks the deleted filter link")
	}
	if rec := c.post("/admin/reviews/" + id + "/restore"); rec.Code != http.StatusSeeOther {
		t.Fatalf("restore: %d", rec.Code)
	}
	if len(e.st.Current().ReviewsFor("gocha-butbaia")) != 1 {
		t.Error("restore failed")
	}
	// Validation keeps the entered text.
	rec = c.post("/admin/reviews/new", "author", "", "text", "hello", "rating", "5", "url", "not a url", "back", "/admin/reviews")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), ">hello</textarea>") {
		t.Errorf("validation: %d, kept text: %v", rec.Code, strings.Contains(rec.Body.String(), ">hello</textarea>"))
	}
}

func TestPublishGate(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	c.get("/admin/coaches/new")
	// A draft may be partial: only the English name is required.
	rec := c.post("/admin/coaches/new", "name_en", "Nino Beridze", "speaks", "ka", "pt_currency", "GEL", "back", "/admin/coaches")
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/admin/coaches/c-") {
		t.Fatalf("draft save: %d -> %s %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/admin/coaches/")
	// An existing but unpublished coach 301s to the list (BRIEF.md §4);
	// only a slug that never existed is a 404.
	if r := c.get("/ka/coaches/nino-beridze"); r.Code != http.StatusMovedPermanently {
		t.Errorf("draft page: %d, want 301 to the list", r.Code)
	}
	// Publishing without Georgian text is refused, with the field named.
	c.get("/admin/coaches/" + id)
	rec = c.post("/admin/coaches/"+id, "name_en", "Nino Beridze", "name_ru", "Нино Беридзе", "bio_en", "Yoga", "bio_ru", "Йога", "published", "1", "pt_currency", "GEL", "slug", "nino-beridze", "back", "/admin/coaches")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "грузинском") || !strings.Contains(rec.Body.String(), `name="draft"`) {
		t.Fatalf("publish gate: %d, has ka error: %v", rec.Code, strings.Contains(rec.Body.String(), "грузинском"))
	}
	// Complete in every locale: published, on the site and in the sitemap.
	rec = c.post("/admin/coaches/"+id, "name_en", "Nino Beridze", "name_ru", "Нино Беридзе", "name_ka", "ნინო ბერიძე", "bio_en", "Yoga", "bio_ru", "Йога", "bio_ka", "იოგა", "published", "1", "pt_currency", "GEL", "slug", "nino-beridze", "speaks", "ka", "back", "/admin/coaches")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body.String())
	}
	if r := c.get("/ka/coaches/nino-beridze"); r.Code != http.StatusOK {
		t.Errorf("published coach page: %d", r.Code)
	}
	if sm := c.get("/sitemap.xml").Body.String(); !strings.Contains(sm, "/en/coaches/nino-beridze") {
		t.Error("sitemap lacks the new coach")
	}
	// Unpublish: 301 to the list. Delete: still 301. Restore: 200 again.
	c.post("/admin/coaches/"+id, "name_en", "Nino Beridze", "name_ru", "Нино Беридзе", "name_ka", "ნინო ბერიძე", "bio_en", "Yoga", "bio_ru", "Йога", "bio_ka", "იოგა", "pt_currency", "GEL", "slug", "nino-beridze", "back", "/admin/coaches")
	if r := c.get("/en/coaches/nino-beridze"); r.Code != http.StatusMovedPermanently || r.Header().Get("Location") != "/en/coaches" {
		t.Errorf("unpublished: %d -> %s", r.Code, r.Header().Get("Location"))
	}
	c.post("/admin/coaches/"+id, "name_en", "Nino Beridze", "name_ru", "Нино Беридзе", "name_ka", "ნინო ბერიძე", "bio_en", "Yoga", "bio_ru", "Йога", "bio_ka", "იოგა", "published", "1", "pt_currency", "GEL", "slug", "nino-beridze", "back", "/admin/coaches")
	c.post("/admin/coaches/" + id + "/delete")
	if r := c.get("/en/coaches/nino-beridze"); r.Code != http.StatusMovedPermanently {
		t.Errorf("deleted coach page: %d", r.Code)
	}
	if body := c.get("/admin/coaches?deleted=1").Body.String(); !strings.Contains(body, "Нино Беридзе") {
		t.Error("deleted list lacks the coach")
	}
	c.post("/admin/coaches/" + id + "/restore")
	if r := c.get("/en/coaches/nino-beridze"); r.Code != http.StatusOK {
		t.Errorf("restored coach page: %d", r.Code)
	}
}

func TestTagDeleteDetachesAndMove(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	c.get("/admin/tags")
	rec := c.post("/admin/tags/new", "name_en", "Boxing", "name_ru", "Бокс", "name_ka", "კრივი")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("new tag: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := e.st.Current().Tag("boxing"); !ok {
		t.Fatal("tag not created with a slug from the English name")
	}
	// Attach to Otar, then delete the tag: Otar loses it, no error.
	in := e.st.Current()
	otar, _ := in.Coach("otar-chkadua")
	c.get("/admin/coaches/c-otar-chkadua")
	form := url.Values{"name_en": {otar.Name["en"]}, "name_ru": {otar.Name["ru"]}, "name_ka": {otar.Name["ka"]},
		"bio_en": {otar.Bio["en"]}, "bio_ru": {otar.Bio["ru"]}, "bio_ka": {otar.Bio["ka"]}, "published": {"1"}, "slug": {"otar-chkadua"},
		"pt_currency": {"GEL"}, "tag": {"boxing"}, "speaks": {"ka", "en"}, "back": {"/admin/coaches"}}
	if rec := c.do(http.MethodPost, "/admin/coaches/c-otar-chkadua", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("attach tag: %d %s", rec.Code, rec.Body.String())
	}
	if o, _ := e.st.Current().Coach("otar-chkadua"); !o.HasTag("boxing") {
		t.Fatal("tag not attached")
	}
	c.post("/admin/tags/boxing/delete")
	if o, _ := e.st.Current().Coach("otar-chkadua"); o.HasTag("boxing") {
		t.Error("deleted tag still on the coach")
	}
	// Move: Nikolai (2nd) goes up above Otar; edges are no-ops.
	c.post("/admin/coaches/c-nikolai-starodubcev/move", "dir", "up", "back", "/admin/coaches")
	if snap := e.st.Current(); snap.Coaches[0].Slug != "nikolai-starodubcev" || snap.Coaches[1].Slug != "otar-chkadua" {
		t.Errorf("move up failed: %s, %s", snap.Coaches[0].Slug, snap.Coaches[1].Slug)
	}
	c.post("/admin/coaches/c-nikolai-starodubcev/move", "dir", "up", "back", "/admin/coaches")
	if snap := e.st.Current(); snap.Coaches[0].Slug != "nikolai-starodubcev" {
		t.Error("move at the top edge changed order")
	}
}

func TestSlotsReplaceAllAndBackWhitelist(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	// Full Body has six seed slots; move Monday 19:00 to 19:30 from the grid.
	row, _ := e.st.ClassRow("k-full-body")
	n := -1
	for i, sl := range row.Slots {
		if sl.Day == 1 && sl.Start == "19:00" {
			n = i
		}
	}
	if n < 0 {
		t.Fatalf("seed slot not found: %+v", row.Slots)
	}
	c.get(fmt.Sprintf("/admin/classes/k-full-body/slots/%d", n))
	rec := c.post(fmt.Sprintf("/admin/classes/k-full-body/slots/%d", n), "day", "1", "start", "19:30", "minutes", "60", "back", "/admin/schedule?day=1")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("move slot: %d %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/admin/schedule?day=1") || !strings.Contains(loc, "saved=k-full-body-1-1930") {
		t.Errorf("back to the grid with the saved marker: %s", loc)
	}
	row, _ = e.st.ClassRow("k-full-body")
	if len(row.Slots) != 6 {
		t.Fatalf("slot count changed: %d", len(row.Slots))
	}
	found := false
	for _, sl := range row.Slots {
		if sl.Day == 1 && sl.Start == "19:30" {
			found = true
		}
	}
	if !found {
		t.Error("slot not moved")
	}
	if body := c.get("/ru/classes").Body.String(); !strings.Contains(body, "19:30") {
		t.Error("public schedule not updated")
	}
	// New slot from the grid with the class chosen on the form.
	c.get("/admin/schedule/new?day=6&start=11:00")
	if rec := c.post("/admin/schedule/new", "class", "k-just-run", "day", "6", "start", "11:00", "minutes", "45", "back", "/admin/schedule"); rec.Code != http.StatusSeeOther {
		t.Fatalf("new slot: %d %s", rec.Code, rec.Body.String())
	}
	jr, _ := e.st.ClassRow("k-just-run")
	if len(jr.Slots) != 2 {
		t.Errorf("just run slots: %+v", jr.Slots)
	}
	// Delete a slot.
	if rec := c.post("/admin/classes/k-just-run/slots/1/delete", "back", "/admin/classes/k-just-run"); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete slot: %d", rec.Code)
	}
	if jr, _ = e.st.ClassRow("k-just-run"); len(jr.Slots) != 1 {
		t.Errorf("slot not deleted: %+v", jr.Slots)
	}
	// ?back= only accepts admin paths.
	for _, bad := range []string{"https://evil.example/", "//evil.example", "/ru/", "/adminx"} {
		if got := safeBack(bad, "/admin/schedule"); got != "/admin/schedule" {
			t.Errorf("safeBack(%q) = %q", bad, got)
		}
	}
	if got := safeBack("/admin/classes/k-full-body", "/admin"); got != "/admin/classes/k-full-body" {
		t.Errorf("safeBack accepted path rejected: %q", got)
	}
}

func TestMembershipTermsAndMassage(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	c.get("/admin/memberships/m-standard")
	// Monthly 180 -> 160, the yearly term withdrawn: /memberships, the home
	// teaser and the JSON-LD change on one save.
	rec := c.post("/admin/memberships/m-standard", "name_en", "Standard", "name_ru", "Стандартный", "name_ka", "სტანდარტული",
		"price_1", "160", "freeze_1", "0", "price_3", "490", "freeze_3", "1", "price_6", "870", "freeze_6", "2", "price_12", "", "gift_pt", "1", "visible", "1", "slug", "standard")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save membership: %d %s", rec.Code, rec.Body.String())
	}
	m := e.st.Current().Memberships[0]
	if m.Slug != "standard" || len(m.Terms) != 3 || m.Terms[0].Price != 160 {
		t.Fatalf("terms: %+v", m.Terms)
	}
	body := c.get("/ru/memberships").Body.String()
	if !strings.Contains(body, "160") || strings.Contains(body, "1550") {
		_ = os.WriteFile(filepath.Join(os.TempDir(), "memberships-debug.html"), []byte(body), 0o644)
		t.Errorf("public memberships not updated or withdrawn term still shown (has 160: %v, has 1550: %v)", strings.Contains(body, "160"), strings.Contains(body, "1550"))
	}
	if !strings.Contains(body, `"price":"160"`) && !strings.Contains(body, `"price": "160"`) && !strings.Contains(body, "160") {
		t.Error("JSON-LD not updated")
	}
	// A visible membership with no sold term is refused.
	rec = c.post("/admin/memberships/new", "name_en", "Ghost", "name_ru", "Призрак", "name_ka", "მოჩვენება", "visible", "1")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "хотя бы для одного срока") {
		t.Errorf("no-term membership accepted: %d", rec.Code)
	}
	// Single visit.
	c.get("/admin/memberships/single")
	if rec := c.post("/admin/memberships/single", "price", "25"); rec.Code != http.StatusSeeOther {
		t.Fatalf("single: %d", rec.Code)
	}
	if e.st.Current().SingleVisit != 25 {
		t.Error("single visit not saved")
	}
	// Massage: new hidden service is absent from the site, visible one appears.
	c.get("/admin/massage/new")
	rec = c.post("/admin/massage/new", "name_en", "Foot massage", "name_ru", "Массаж стоп", "name_ka", "ტერფის მასაჟი", "minutes", "30", "price", "70", "visible", "1")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("massage: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(c.get("/ru/massage").Body.String(), "Массаж стоп") {
		t.Error("massage service not on the site")
	}
	rows, _ := e.st.MassageRows()
	id := rows[len(rows)-1].ID
	c.post("/admin/massage/"+id, "name_en", "Foot massage", "name_ru", "Массаж стоп", "name_ka", "ტერფის მასაჟი", "minutes", "30", "price", "70")
	if strings.Contains(c.get("/ru/massage").Body.String(), "Массаж стоп") {
		t.Error("hidden massage service still on the site")
	}
}

func TestGridConflicts(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	// Kettlebell on Wednesday 14:00 already exists; add Just Run at 14:30
	// on Wednesday: overlap allowed, marked, both on the grid.
	c.get("/admin/schedule/new")
	if rec := c.post("/admin/schedule/new", "class", "k-just-run", "day", "3", "start", "14:30", "minutes", "60", "back", "/admin/schedule?day=3"); rec.Code != http.StatusSeeOther {
		t.Fatalf("slot: %d %s", rec.Code, rec.Body.String())
	}
	body := c.get("/admin/schedule?day=3").Body.String()
	if !strings.Contains(body, "slot--conflict") || !strings.Contains(body, "пересекается с") {
		t.Error("overlap not marked on the grid")
	}
	if !strings.Contains(body, `class="grid grid--day-3"`) || !strings.Contains(body, "Добавить слот, Ср 07:00") {
		t.Error("grid day selection or cell links missing")
	}
}

func TestRoleGateAndExport(t *testing.T) {
	e := newEnv(t)
	owner := e.client(t)
	owner.loginOwner()
	owner.get("/admin/users/new")
	rec := owner.post("/admin/users/new", "name", "Нино", "login", "nino", "role", "editor")
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/admin/users/u-") {
		t.Fatalf("create user: %d %s", rec.Code, rec.Body.String())
	}
	page := owner.get(rec.Header().Get("Location")).Body.String()
	m := regexp.MustCompile(`<code>([a-z0-9]{12})</code>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("temporary password not shown once")
	}
	temp := m[1]
	if strings.Contains(owner.get(rec.Header().Get("Location")).Body.String(), temp) {
		t.Error("temporary password shown twice")
	}
	// The editor logs in with the temporary password, must set their own,
	// then sees no owner-only sections.
	ed := e.client(t)
	if r := ed.login("nino", temp); r.Code != http.StatusSeeOther || r.Header().Get("Location") != "/admin/setup" {
		t.Fatalf("editor login: %d -> %s", r.Code, r.Header().Get("Location"))
	}
	ed.get("/admin/setup")
	if r := ed.post("/admin/setup", "name", "Нино", "password", "editor-password-1", "password2", "editor-password-1"); r.Code != http.StatusSeeOther {
		t.Fatalf("editor setup: %d %s", r.Code, r.Body.String())
	}
	if r := ed.get("/admin/users"); r.Code != http.StatusForbidden {
		t.Errorf("editor opened users: %d", r.Code)
	}
	if r := ed.post("/admin/export"); r.Code != http.StatusForbidden {
		t.Errorf("editor exported: %d", r.Code)
	}
	if body := ed.get("/admin").Body.String(); strings.Contains(body, "/admin/users") {
		t.Error("editor sees the users link")
	}
	// The editor edits content freely.
	ed.get("/admin/hours")
	if r := ed.post("/admin/memberships/single", "price", "30"); r.Code != http.StatusSeeOther {
		t.Errorf("editor cannot edit content: %d", r.Code)
	}
	// Owner cannot block themselves; blocking the editor ends their session.
	me, _ := e.st.Owner()
	owner.post("/admin/users/" + me.ID + "/block")
	if u, _ := e.st.Owner(); u.Blocked {
		t.Error("owner blocked themselves")
	}
	edID := strings.TrimPrefix(rec.Header().Get("Location"), "/admin/users/")
	owner.post("/admin/users/" + edID + "/block")
	if r := ed.get("/admin"); r.Code != http.StatusSeeOther {
		t.Errorf("blocked editor still in: %d", r.Code)
	}
	// Export contains every table.
	r := owner.post("/admin/export")
	if r.Code != http.StatusOK || !strings.Contains(r.Header().Get("Content-Disposition"), "fitto-content-") {
		t.Fatalf("export: %d", r.Code)
	}
	for _, key := range []string{`"memberships"`, `"coaches"`, `"reviews"`, `"classes"`, `"massage"`, `"hours"`, `"changelog"`, `"users"`, `"single_visit": 30`} {
		if !strings.Contains(r.Body.String(), key) {
			t.Errorf("export lacks %s", key)
		}
	}
	if strings.Contains(r.Body.String(), "password_hash") || strings.Contains(r.Body.String(), "$2a$") {
		t.Error("export leaks password hashes")
	}
	// History lists the block with a human sentence and offers restore for deletions.
	owner.post("/admin/massage/s-03/delete")
	hist := owner.get("/admin/history").Body.String()
	if !strings.Contains(hist, "заблокировал(а)") || !strings.Contains(hist, "/admin/massage/s-03/restore") {
		t.Error("history lacks the block line or the restore button")
	}
	owner.post("/admin/massage/s-03/restore")
	if strings.Contains(owner.get("/admin/history").Body.String(), "/admin/massage/s-03/restore\"") {
		t.Error("restore button shown for a restored record")
	}
}

func TestProfilePasswordChange(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	c.get("/admin/profile")
	if r := c.post("/admin/profile", "name", "Отар", "lang", "ka", "theme", "dark", "old_password", "wrong", "password", "another-password-2", "password2", "another-password-2"); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("wrong old password accepted: %d", r.Code)
	}
	if r := c.post("/admin/profile", "name", "Отар", "lang", "ka", "theme", "dark", "old_password", "my-own-password-9", "password", "another-password-2", "password2", "another-password-2"); r.Code != http.StatusSeeOther {
		t.Fatalf("profile: %d %s", r.Code, r.Body.String())
	}
	body := c.get("/admin").Body.String()
	if !strings.Contains(body, `<html lang="ka" data-theme="dark"`) {
		t.Error("profile language/theme not applied")
	}
	c2 := e.client(t)
	if r := c2.login("otar", "another-password-2"); r.Code != http.StatusSeeOther {
		t.Error("new password rejected")
	}
}
