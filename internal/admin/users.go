package admin

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/konorlevich/fitto-club/internal/store"
)

// ------------------------------------------------------------------ users

type usersData struct {
	Users []store.User
	Me    string
}

func (s *Server) usersList(c *ctx) {
	p := s.newPage(c, "users", c.copy(s).T("nav.users"))
	p.Action = &Link{Href: "/admin/users/new", Label: p.T("action.add")}
	users, err := s.Store.Users()
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	p.Data = usersData{Users: users, Me: c.user.ID}
	s.render(c, "users", p, http.StatusOK)
}

type userFormData struct {
	New          bool
	U            store.User
	Me           bool
	TempPassword string // shown once after create/reset
}

// tempPassword is 12 characters from an unambiguous alphabet.
func tempPassword() string {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	var b [12]byte
	_, _ = rand.Read(b[:])
	out := make([]byte, len(b))
	for i, x := range b {
		out[i] = alphabet[int(x)%len(alphabet)]
	}
	return string(out)
}

func (s *Server) userForm(c *ctx) {
	p := s.newPage(c, "users", "")
	p.Back = "/admin/users"
	d := userFormData{}
	if id := c.r.PathValue("id"); id != "" {
		u, err := s.Store.UserByID(id)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.U, d.Me = u, u.ID == c.user.ID
		p.Title = u.Name
		if p.Title == "" {
			p.Title = u.Login
		}
		if tp := s.takeTemp(c); tp != "" {
			d.TempPassword = tp
		}
	} else {
		d.New = true
		d.U = store.User{Role: store.RoleEditor}
		p.Title = p.T("user.title_new")
	}
	p.Data = d
	s.render(c, "user", p, http.StatusOK)
}

// The temporary password travels to the next page in a one-shot cookie,
// like a flash, and is never logged or stored in clear.
const tempCookie = "admin_temp"

func (s *Server) takeTemp(c *ctx) string {
	ck, err := c.r.Cookie(tempCookie)
	if err != nil || ck.Value == "" {
		return ""
	}
	s.setCookie(c, tempCookie, "", -1)
	return ck.Value
}

func (s *Server) userCreate(c *ctx) {
	p := s.newPage(c, "users", c.copy(s).T("user.title_new"))
	p.Back = "/admin/users"
	login := strings.ToLower(strings.TrimSpace(c.r.FormValue("login")))
	name := strings.TrimSpace(c.r.FormValue("name"))
	role := c.r.FormValue("role")
	if role != store.RoleOwner {
		role = store.RoleEditor
	}
	d := userFormData{New: true, U: store.User{Login: login, Name: name, Role: role}}
	if login == "" || strings.ContainsAny(login, " \t@/") || utf8.RuneCountInString(login) < 3 {
		p.Errors = append(p.Errors, FieldError{"login", p.T("user.error_login")})
	}
	if name == "" {
		p.Errors = append(p.Errors, FieldError{"name", p.T("field.empty")})
	}
	if _, _, err := s.Store.UserByLogin(login); err == nil {
		p.Errors = append(p.Errors, FieldError{"login", p.T("user.error_taken")})
	}
	if len(p.Errors) > 0 {
		p.Data = d
		s.render(c, "user", p, http.StatusUnprocessableEntity)
		return
	}
	temp := tempPassword()
	hash, err := hashPassword(temp)
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	u, err := s.Store.CreateUser(login, name, role, hash, c.user.Name)
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	s.setCookie(c, tempCookie, temp, time.Minute)
	s.redirect(c, "/admin/users/"+u.ID)
}

func (s *Server) userReset(c *ctx) {
	id := c.r.PathValue("id")
	if id == c.user.ID {
		s.redirect(c, "/admin/profile")
		return
	}
	temp := tempPassword()
	hash, err := hashPassword(temp)
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	if err := s.Store.ResetUserPassword(id, hash, c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.setCookie(c, tempCookie, temp, time.Minute)
	s.redirect(c, "/admin/users/"+id)
}

func (s *Server) userBlock(c *ctx, blocked bool) {
	id := c.r.PathValue("id")
	if id == c.user.ID {
		s.flash(c, "err", c.copy(s).T("user.error_self"), "", "")
		s.redirect(c, "/admin/users/"+id)
		return
	}
	if err := s.Store.SetBlocked(id, blocked, c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	key := "user.unblocked"
	if blocked {
		key = "user.blocked"
	}
	s.flash(c, "ok", c.copy(s).T(key), "", "")
	s.redirect(c, "/admin/users/"+id)
}

// ---------------------------------------------------------------- profile

type profileData struct {
	U store.User
}

func (s *Server) profileForm(c *ctx) {
	p := s.newPage(c, "profile", c.copy(s).T("nav.profile"))
	p.Data = profileData{U: *c.user}
	s.render(c, "profile", p, http.StatusOK)
}

func (s *Server) profilePost(c *ctx) {
	p := s.newPage(c, "profile", c.copy(s).T("nav.profile"))
	name := strings.TrimSpace(c.r.FormValue("name"))
	lang := s.uiLang(c.r.FormValue("lang"))
	theme := c.r.FormValue("theme")
	if theme != "light" && theme != "dark" {
		theme = "auto"
	}
	if name == "" {
		p.Errors = append(p.Errors, FieldError{"name", p.T("field.empty")})
	}
	// Optional password change: all three fields or none.
	old, pw, pw2 := c.r.FormValue("old_password"), c.r.FormValue("password"), c.r.FormValue("password2")
	changePw := old != "" || pw != "" || pw2 != ""
	if changePw {
		_, hash, err := s.Store.UserByLogin(c.user.Login)
		switch {
		case err != nil || !checkPassword(hash, old):
			p.Errors = append(p.Errors, FieldError{"old_password", p.T("profile.error_old")})
		case utf8.RuneCountInString(pw) < minPassword:
			p.Errors = append(p.Errors, FieldError{"password", p.T("setup.short")})
		case pw != pw2:
			p.Errors = append(p.Errors, FieldError{"password2", p.T("setup.mismatch")})
		}
	}
	if len(p.Errors) > 0 {
		u := *c.user
		u.Name, u.Lang, u.Theme = name, lang, theme
		p.Data = profileData{U: u}
		s.render(c, "profile", p, http.StatusUnprocessableEntity)
		return
	}
	if err := s.Store.UpdateProfile(c.user.ID, name, lang, theme); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	s.setCookie(c, langCookie, lang, 365*24*time.Hour)
	if theme == "auto" {
		s.setCookie(c, themeCookie, "", -1)
	} else {
		s.setCookie(c, themeCookie, theme, 365*24*time.Hour)
	}
	msg := p.T("flash.saved")
	if changePw {
		hash, err := hashPassword(pw)
		if err != nil || s.Store.SetPassword(c.user.ID, hash, false) != nil {
			s.errorPage(c, http.StatusInternalServerError)
			return
		}
		msg = p.T("flash.password_changed")
	}
	s.flash(c, "ok", msg, "", "")
	s.redirect(c, "/admin/profile")
}

// ----------------------------------------------------------------- export

// export streams every editable table as one JSON document, so the club's
// data is never hostage to us (checklist §13).
func (s *Server) export(c *ctx) {
	out := map[string]any{"exported_at": c.now.UTC().Format(time.RFC3339), "site": s.Cfg.BaseURL}
	if v, err := s.Store.MembershipRows(); err == nil {
		out["memberships"] = v
	}
	out["single_visit"] = s.Store.Current().SingleVisit
	if v, err := s.Store.CoachRows(); err == nil {
		out["coaches"] = v
	}
	out["tags"] = s.Store.Current().Tags
	if v, err := s.Store.ReviewRows(false); err == nil {
		out["reviews"] = v
	}
	if v, err := s.Store.ReviewRows(true); err == nil {
		out["reviews_deleted"] = v
	}
	if v, err := s.Store.ClassRows(); err == nil {
		out["classes"] = v
	}
	if v, err := s.Store.MassageRows(); err == nil {
		out["massage"] = v
	}
	out["hours"] = s.Store.Current().Hours
	if v, err := s.Store.Changes("", 10000, 0); err == nil {
		out["changelog"] = v
	}
	if v, err := s.Store.Users(); err == nil {
		out["users"] = v // no hashes: User carries none
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	h := c.w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="fitto-content-%s.json"`, c.now.Format("2006-01-02")))
	h.Set("Content-Length", fmt.Sprint(len(b)))
	_, _ = c.w.Write(b)
}
