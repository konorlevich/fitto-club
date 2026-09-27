package admin

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/konorlevich/fitto-club/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const (
	loginWindow   = 15 * time.Minute
	loginMax      = 5
	minPassword   = 10
	bcryptCost    = 12
	sessionMaxAge = 30 * 24 * time.Hour
)

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	return string(b), err
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

type loginData struct {
	Login, Error string
	Locked       bool
}

func (s *Server) loginForm(c *ctx) {
	if c.user != nil {
		http.Redirect(c.w, c.r, "/admin", http.StatusSeeOther)
		return
	}
	if l := c.r.URL.Query().Get("lang"); l != "" {
		c.lang = s.uiLang(l)
		s.setCookie(c, langCookie, c.lang, 365*24*time.Hour)
	}
	p := s.newPage(c, "", c.copy(s).T("login.title"))
	p.Bare = true
	p.Data = loginData{}
	s.render(c, "login", p, http.StatusOK)
}

func (s *Server) loginPost(c *ctx) {
	login := strings.TrimSpace(c.r.FormValue("login"))
	pw := c.r.FormValue("password")
	ip := clientIP(c.r)
	if l := c.r.FormValue("lang"); l != "" {
		c.lang = s.uiLang(l)
		s.setCookie(c, langCookie, c.lang, 365*24*time.Hour)
	}
	fail := func(locked bool) {
		p := s.newPage(c, "", c.copy(s).T("login.title"))
		p.Bare = true
		d := loginData{Login: login, Error: p.T("login.failed")}
		if locked {
			mins := int(loginWindow.Minutes())
			if t, ok := s.Store.OldestFailure(login, ip, loginWindow); ok {
				if left := int(t.Add(loginWindow).Sub(c.now.UTC()).Minutes()) + 1; left > 0 && left <= mins {
					mins = left
				}
			}
			d.Error, d.Locked = p.F("login.locked", mins), true
		}
		p.Data = d
		p.Errors = []FieldError{{Field: "login", Text: d.Error}}
		s.render(c, "login", p, http.StatusUnauthorized)
	}
	if n, _ := s.Store.Failures(login, ip, loginWindow); n >= loginMax {
		fail(true)
		return
	}
	u, ok := s.authenticate(login, pw)
	if !ok {
		_ = s.Store.RecordFailure(login, ip)
		if n, _ := s.Store.Failures(login, ip, loginWindow); n >= loginMax {
			fail(true)
			return
		}
		fail(false)
		return
	}
	token, _, err := s.Store.CreateSession(u.ID)
	if err != nil {
		s.Log.WithError(err).Error("creating admin session")
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	_ = s.Store.ClearFailures(login)
	s.setCookie(c, sessionCookie, token, sessionMaxAge)
	if c.r.FormValue("lang") == "" && u.Lang != "" {
		s.setCookie(c, langCookie, s.uiLang(u.Lang), 365*24*time.Hour)
	} else if c.r.FormValue("lang") != "" {
		_ = s.Store.UpdateProfile(u.ID, u.Name, c.lang, u.Theme)
	}
	if u.MustChange {
		http.Redirect(c.w, c.r, "/admin/setup", http.StatusSeeOther)
		return
	}
	http.Redirect(c.w, c.r, "/admin", http.StatusSeeOther)
}

// authenticate checks a stored bcrypt hash, or, only while the owner has no
// password in the database yet, the ENV bootstrap pair. Unknown logins,
// wrong passwords and blocked accounts all fail the same way.
func (s *Server) authenticate(login, pw string) (store.User, bool) {
	if login == "" || pw == "" {
		return store.User{}, false
	}
	u, hash, err := s.Store.UserByLogin(login)
	if errors.Is(err, store.ErrNotFound) && s.Cfg.AdminEnabled() && login == s.Cfg.AdminLogin {
		if _, oerr := s.Store.Owner(); errors.Is(oerr, store.ErrNotFound) {
			if u, err = s.Store.EnsureOwner(login); err != nil {
				return store.User{}, false
			}
		}
	} else if err != nil {
		return store.User{}, false
	}
	if u.Blocked {
		return store.User{}, false
	}
	if hash != "" {
		return u, checkPassword(hash, pw)
	}
	// No password yet: owner bootstrap from ENV, or a user whose temporary
	// password was never issued (cannot log in at all).
	if u.IsOwner() && s.Cfg.AdminEnabled() && login == s.Cfg.AdminLogin &&
		subtle.ConstantTimeCompare([]byte(pw), []byte(s.Cfg.AdminPassword)) == 1 {
		return u, true
	}
	return store.User{}, false
}

func (s *Server) logout(c *ctx) {
	if ck, err := c.r.Cookie(sessionCookie); err == nil {
		_ = s.Store.DeleteSession(ck.Value)
	}
	s.setCookie(c, sessionCookie, "", -1)
	http.Redirect(c.w, c.r, "/admin/login", http.StatusSeeOther)
}

type setupData struct {
	Name     string
	Change   bool // temporary password → own password (not the very first owner login)
	Password string
}

func (s *Server) setupForm(c *ctx) {
	if !c.user.MustChange {
		http.Redirect(c.w, c.r, "/admin/profile", http.StatusSeeOther)
		return
	}
	p := s.newPage(c, "", c.copy(s).T("setup.title"))
	p.Bare = true
	p.Data = setupData{Name: c.user.Name, Change: c.user.HasPassword}
	if p.Data.(setupData).Change {
		p.Title = p.T("setup.change_title")
	}
	s.render(c, "setup", p, http.StatusOK)
}

func (s *Server) setupPost(c *ctx) {
	name := strings.TrimSpace(c.r.FormValue("name"))
	pw, pw2 := c.r.FormValue("password"), c.r.FormValue("password2")
	p := s.newPage(c, "", c.copy(s).T("setup.title"))
	p.Bare = true
	d := setupData{Name: name, Change: c.user.HasPassword}
	if d.Change {
		p.Title = p.T("setup.change_title")
	}
	if utf8.RuneCountInString(pw) < minPassword {
		p.Errors = append(p.Errors, FieldError{"password", p.T("setup.short")})
	} else if pw != pw2 {
		p.Errors = append(p.Errors, FieldError{"password2", p.T("setup.mismatch")})
	}
	if name == "" {
		p.Errors = append(p.Errors, FieldError{"name", p.T("field.empty")})
	}
	if len(p.Errors) > 0 {
		p.Data = d
		s.render(c, "setup", p, http.StatusUnprocessableEntity)
		return
	}
	hash, err := hashPassword(pw)
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	if err := s.Store.SetPassword(c.user.ID, hash, false); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	_ = s.Store.UpdateProfile(c.user.ID, name, c.lang, c.user.Theme)
	s.flash(c, "ok", p.T("flash.password_changed"), "", "")
	http.Redirect(c.w, c.r, "/admin", http.StatusSeeOther)
}
