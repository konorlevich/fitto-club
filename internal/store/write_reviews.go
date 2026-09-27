package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
)

var monthRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

type ReviewInput struct {
	ID       string
	Author   string
	Lang     string
	Text     string
	URL      string
	Rating   int
	Date     string // YYYY-MM
	Home     bool
	Sort     int
	CoachIDs []string
}

func (in ReviewInput) Validate() error {
	var bad []string
	if strings.TrimSpace(in.Author) == "" {
		bad = append(bad, "author")
	}
	if strings.TrimSpace(in.Text) == "" {
		bad = append(bad, "text")
	}
	if !slices.Contains([]string{"en", "ru", "ka"}, in.Lang) {
		bad = append(bad, "lang")
	}
	if in.Rating < 1 || in.Rating > 5 {
		bad = append(bad, "rating")
	}
	if in.Date != "" && !monthRe.MatchString(in.Date) {
		bad = append(bad, "date")
	}
	if in.URL != "" {
		if u, err := url.Parse(in.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			bad = append(bad, "url")
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(bad, ", "))
	}
	return nil
}

type ReviewRow struct {
	content.Review
	CoachIDs []string
	Deleted  bool
	Updated  string
}

// ReviewRows lists reviews newest first, optionally only the deleted ones.
func (s *SQLite) ReviewRows(deleted bool) ([]ReviewRow, error) {
	links := map[string][]string{}
	if err := s.each(`SELECT review_id, coach_id FROM review_coaches`, func(r *sql.Rows) error {
		var rid, cid string
		if err := r.Scan(&rid, &cid); err != nil {
			return err
		}
		links[rid] = append(links[rid], cid)
		return nil
	}); err != nil {
		return nil, err
	}
	slugOf := map[string]string{}
	if err := s.each(`SELECT id, slug FROM coaches`, func(r *sql.Rows) error {
		var id, slug string
		if err := r.Scan(&id, &slug); err != nil {
			return err
		}
		slugOf[id] = slug
		return nil
	}); err != nil {
		return nil, err
	}
	where := `deleted_at=''`
	if deleted {
		where = `deleted_at<>''`
	}
	var out []ReviewRow
	err := s.each(`SELECT id, author, lang, body, url, rating, written, home, sort, updated_at, deleted_at FROM reviews WHERE `+where+` ORDER BY written DESC, updated_at DESC, id DESC`, func(r *sql.Rows) error {
		var v ReviewRow
		var home int
		var del string
		if err := r.Scan(&v.ID, &v.Author, &v.Lang, &v.Text, &v.URL, &v.Rating, &v.Date, &home, &v.Sort, &v.Updated, &del); err != nil {
			return err
		}
		v.Home, v.Deleted, v.CoachIDs = home == 1, del != "", links[v.ID]
		for _, cid := range v.CoachIDs {
			if slug := slugOf[cid]; slug != "" {
				v.Coaches = append(v.Coaches, slug)
			}
		}
		out = append(out, v)
		return nil
	})
	return out, err
}

func (s *SQLite) ReviewRow(id string) (ReviewRow, error) {
	for _, deleted := range []bool{false, true} {
		rows, err := s.ReviewRows(deleted)
		if err != nil {
			return ReviewRow{}, err
		}
		for _, r := range rows {
			if r.ID == id {
				return r, nil
			}
		}
	}
	return ReviewRow{}, ErrNotFound
}

// reviewPages are the URLs a review appears on: the home strip and the
// pages of the coaches it is linked to (old links included on a change).
type querier interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

func (s *SQLite) reviewPages(ex querier, id string, extra []string) []string {
	pages := []string{"/"}
	ids := append([]string(nil), extra...)
	rows, err := ex.Query(`SELECT coach_id FROM review_coaches WHERE review_id=?`, id)
	if err == nil {
		for rows.Next() {
			var cid string
			if rows.Scan(&cid) == nil {
				ids = append(ids, cid)
			}
		}
		rows.Close()
	}
	for _, cid := range ids {
		var slug string
		if ex.QueryRow(`SELECT slug FROM coaches WHERE id=?`, cid).Scan(&slug) == nil && slug != "" {
			pages = append(pages, "/coaches/"+slug)
		}
	}
	return pages
}

func (s *SQLite) SaveReview(in ReviewInput, actor string) (string, error) {
	if err := in.Validate(); err != nil {
		return "", err
	}
	action := "save"
	if in.ID == "" {
		in.ID = NewID("r-")
		action = "create"
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	pages := s.reviewPages(tx, in.ID, in.CoachIDs)
	if _, err := tx.Exec(`INSERT INTO reviews(id,author,lang,body,url,rating,written,home,sort,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET author=excluded.author, lang=excluded.lang, body=excluded.body, url=excluded.url, rating=excluded.rating,
		written=excluded.written, home=excluded.home, sort=excluded.sort, updated_at=excluded.updated_at`,
		in.ID, strings.TrimSpace(in.Author), in.Lang, strings.TrimSpace(in.Text), in.URL, in.Rating, in.Date, b2i(in.Home), in.Sort, stamp); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`DELETE FROM review_coaches WHERE review_id=?`, in.ID); err != nil {
		return "", err
	}
	for _, cid := range in.CoachIDs {
		if _, err := tx.Exec(`INSERT INTO review_coaches VALUES(?,?)`, in.ID, cid); err != nil {
			return "", fmt.Errorf("coach %q: %w", cid, err)
		}
	}
	if err := s.touch(tx, stamp, pages...); err != nil {
		return "", err
	}
	if err := s.logChange(tx, stamp, actor, "review", in.ID, action, strings.TrimSpace(in.Author)); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return in.ID, s.Reload()
}

// setDeleted soft-deletes or restores a row in one of the content tables.
func (s *SQLite) setDeleted(table, entity, id string, deleted bool, pages []string, actor, summary string) error {
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	val := ""
	action := "restore"
	if deleted {
		val, action = stamp, "delete"
	}
	res, err := tx.Exec(`UPDATE `+table+` SET deleted_at=?, updated_at=? WHERE id=?`, val, stamp, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := s.touch(tx, stamp, pages...); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, entity, id, action, summary); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

func (s *SQLite) DeleteReview(id, actor string) error {
	r, err := s.ReviewRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("reviews", "review", id, true, s.reviewPages(s.db, id, nil), actor, r.Author)
}

func (s *SQLite) RestoreReview(id, actor string) error {
	r, err := s.ReviewRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("reviews", "review", id, false, s.reviewPages(s.db, id, nil), actor, r.Author)
}
