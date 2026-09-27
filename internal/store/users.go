package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Admin accounts, sessions and the login rate limit. Password hashing lives
// in the admin package; the store only keeps opaque hashes.

const (
	RoleOwner  = "owner"
	RoleEditor = "editor"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ID, Login, Name, Role string
	Lang, Theme           string
	MustChange, Blocked   bool
	HasPassword           bool
	CreatedAt             string
}

func (u User) IsOwner() bool { return u.Role == RoleOwner }

const userCols = `id, login, name, role, password_hash, must_change, blocked, lang, theme, created_at`

func scanUser(r interface{ Scan(...any) error }) (User, string, error) {
	var u User
	var hash string
	var mc, bl int
	if err := r.Scan(&u.ID, &u.Login, &u.Name, &u.Role, &hash, &mc, &bl, &u.Lang, &u.Theme, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, "", ErrNotFound
		}
		return User{}, "", err
	}
	u.MustChange, u.Blocked, u.HasPassword = mc == 1, bl == 1, hash != ""
	return u, hash, nil
}

// UserByLogin returns the user and the stored password hash ("" when unset).
func (s *SQLite) UserByLogin(login string) (User, string, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE login=?`, login))
}

func (s *SQLite) UserByID(id string) (User, error) {
	u, _, err := scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id=?`, id))
	return u, err
}

// Users lists every account, owner first, then by name.
func (s *SQLite) Users() ([]User, error) {
	var out []User
	err := s.each(`SELECT `+userCols+` FROM users ORDER BY CASE role WHEN 'owner' THEN 0 ELSE 1 END, name, login`, func(r *sql.Rows) error {
		u, _, err := scanUser(r)
		if err != nil {
			return err
		}
		out = append(out, u)
		return nil
	})
	return out, err
}

// Owner returns the owner account if one exists.
func (s *SQLite) Owner() (User, error) {
	u, _, err := scanUser(s.db.QueryRow(`SELECT ` + userCols + ` FROM users WHERE role='owner' ORDER BY created_at LIMIT 1`))
	return u, err
}

// EnsureOwner creates the owner row for the bootstrap login if no owner
// exists yet. The row has no password: the first login goes through setup.
func (s *SQLite) EnsureOwner(login string) (User, error) {
	if u, err := s.Owner(); err == nil {
		return u, nil
	} else if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}
	stamp := s.stamp()
	id := NewID("u-")
	if _, err := s.db.Exec(`INSERT INTO users(id,login,name,role,password_hash,must_change,blocked,lang,theme,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, login, "", RoleOwner, "", 1, 0, "ru", "auto", stamp, stamp); err != nil {
		return User{}, err
	}
	return s.UserByID(id)
}

// ResetOwner clears the owner's password so the ENV bootstrap works again
// (DEPLOY.md: recovering a lost owner password).
func (s *SQLite) ResetOwner() error {
	res, err := s.db.Exec(`UPDATE users SET password_hash='', must_change=1, blocked=0, updated_at=? WHERE role='owner'`, s.stamp())
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_, err = s.db.Exec(`DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE role='owner')`)
	return err
}

// CreateUser adds an editor (or a second owner) with a temporary password
// hash that must be changed at first login.
func (s *SQLite) CreateUser(login, name, role, tempHash, actor string) (User, error) {
	if login == "" {
		return User{}, fmt.Errorf("%w: login", ErrValidation)
	}
	if role != RoleOwner && role != RoleEditor {
		return User{}, fmt.Errorf("%w: role", ErrValidation)
	}
	stamp := s.stamp()
	id := NewID("u-")
	if _, err := s.db.Exec(`INSERT INTO users(id,login,name,role,password_hash,must_change,blocked,lang,theme,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, login, name, role, tempHash, 1, 0, "ru", "auto", stamp, stamp); err != nil {
		return User{}, err
	}
	if err := s.logChange(s.db, stamp, actor, "user", id, "create", login); err != nil {
		return User{}, err
	}
	return s.UserByID(id)
}

// SetPassword stores a new hash; mustChange=false marks the account ready.
func (s *SQLite) SetPassword(id, hash string, mustChange bool) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash=?, must_change=?, updated_at=? WHERE id=?`, hash, b2i(mustChange), s.stamp(), id)
	return err
}

// UpdateProfile saves the fields a user edits about themselves.
func (s *SQLite) UpdateProfile(id, name, lang, theme string) error {
	_, err := s.db.Exec(`UPDATE users SET name=?, lang=?, theme=?, updated_at=? WHERE id=?`, name, lang, theme, s.stamp(), id)
	return err
}

// SetBlocked blocks or unblocks an account; blocking ends its sessions.
func (s *SQLite) SetBlocked(id string, blocked bool, actor string) error {
	stamp := s.stamp()
	if _, err := s.db.Exec(`UPDATE users SET blocked=?, updated_at=? WHERE id=?`, b2i(blocked), stamp, id); err != nil {
		return err
	}
	if blocked {
		if _, err := s.db.Exec(`DELETE FROM sessions WHERE user_id=?`, id); err != nil {
			return err
		}
	}
	action := "unblock"
	if blocked {
		action = "block"
	}
	return s.logChange(s.db, stamp, actor, "user", id, action, "")
}

// ResetUserPassword sets a temporary hash and forces a change; sessions end.
func (s *SQLite) ResetUserPassword(id, tempHash, actor string) error {
	stamp := s.stamp()
	if _, err := s.db.Exec(`UPDATE users SET password_hash=?, must_change=1, updated_at=? WHERE id=?`, tempHash, stamp, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE user_id=?`, id); err != nil {
		return err
	}
	return s.logChange(s.db, stamp, actor, "user", id, "reset", "")
}

// ------------------------------------------------------------ sessions

const sessionTTL = 30 * 24 * time.Hour

func randomToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// CreateSession returns the cookie value and the CSRF token.
func (s *SQLite) CreateSession(userID string) (token, csrf string, err error) {
	token, csrf = randomToken(), randomToken()
	now := s.now().UTC()
	_, err = s.db.Exec(`INSERT INTO sessions(id,user_id,csrf,created_at,expires_at) VALUES(?,?,?,?,?)`,
		hashToken(token), userID, csrf, now.Format(time.RFC3339), now.Add(sessionTTL).Format(time.RFC3339))
	return
}

// SessionUser resolves a cookie value to its user; expired or blocked
// sessions answer ErrNotFound. Sessions slide: each use extends them.
func (s *SQLite) SessionUser(token string) (User, string, error) {
	id := hashToken(token)
	var userID, csrf, exp string
	if err := s.db.QueryRow(`SELECT user_id, csrf, expires_at FROM sessions WHERE id=?`, id).Scan(&userID, &csrf, &exp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, "", ErrNotFound
		}
		return User{}, "", err
	}
	now := s.now().UTC()
	if t, err := time.Parse(time.RFC3339, exp); err != nil || !t.After(now) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE id=?`, id)
		return User{}, "", ErrNotFound
	}
	u, err := s.UserByID(userID)
	if err != nil || u.Blocked {
		return User{}, "", ErrNotFound
	}
	_, _ = s.db.Exec(`UPDATE sessions SET expires_at=? WHERE id=?`, now.Add(sessionTTL).Format(time.RFC3339), id)
	return u, csrf, nil
}

func (s *SQLite) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id=?`, hashToken(token))
	return err
}

// ------------------------------------------------------- login attempts

// RecordFailure stores a failed login and prunes attempts older than an hour.
func (s *SQLite) RecordFailure(login, ip string) error {
	now := s.now().UTC()
	if _, err := s.db.Exec(`DELETE FROM login_attempts WHERE at < ?`, now.Add(-time.Hour).Format(time.RFC3339)); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO login_attempts(id,login,ip,at) VALUES(?,?,?,?)`, NewID("a-"), login, ip, now.Format(time.RFC3339))
	return err
}

// Failures counts failed logins for a login or an IP within the window.
func (s *SQLite) Failures(login, ip string, window time.Duration) (int, error) {
	since := s.now().UTC().Add(-window).Format(time.RFC3339)
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM login_attempts WHERE at >= ? AND (login=? OR ip=?)`, since, login, ip).Scan(&n)
	return n, err
}

// OldestFailure is when the current streak started, for "try again in N min".
func (s *SQLite) OldestFailure(login, ip string, window time.Duration) (time.Time, bool) {
	since := s.now().UTC().Add(-window).Format(time.RFC3339)
	var at string
	if err := s.db.QueryRow(`SELECT MIN(at) FROM login_attempts WHERE at >= ? AND (login=? OR ip=?)`, since, login, ip).Scan(&at); err != nil || at == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, at)
	return t, err == nil
}

func (s *SQLite) ClearFailures(login string) error {
	_, err := s.db.Exec(`DELETE FROM login_attempts WHERE login=?`, login)
	return err
}

// ------------------------------------------------------------ changelog

type Change struct {
	ID, At, Actor, Entity, EntityID, Action, Summary string
}

// Changes lists the log, newest first, optionally for one entity type.
func (s *SQLite) Changes(entity string, limit, offset int) ([]Change, error) {
	q := `SELECT id, at, actor, entity, entity_id, action, summary FROM changelog`
	var args []any
	if entity != "" {
		q += ` WHERE entity=?`
		args = append(args, entity)
	}
	q += ` ORDER BY at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Change
	for rows.Next() {
		var c Change
		if err := rows.Scan(&c.ID, &c.At, &c.Actor, &c.Entity, &c.EntityID, &c.Action, &c.Summary); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
