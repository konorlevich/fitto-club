package store

import (
	"fmt"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
)

// RestoreCoach undoes a soft delete; the coach comes back with the
// published flag it had, so its URL answers 200 again.
func (s *SQLite) RestoreCoach(id, actor string) error {
	r, err := s.CoachRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("coaches", "coach", id, false, []string{"/", "/coaches", "/coaches/" + r.Slug, "/classes", "/massage"}, actor, r.Name.Get("en"))
}

// MoveCoach changes display order on the list and the home teaser.
func (s *SQLite) MoveCoach(id, dir, actor string) error {
	r, err := s.CoachRow(id)
	if err != nil {
		return err
	}
	return s.Move("coaches", "id", id, dir, []string{"/", "/coaches"}, "coach", actor, r.Name.Get("en"))
}

type TagInput struct {
	Slug string
	Name content.L
	Sort int
}

// SaveTag upserts a tag of the specialty dictionary.
func (s *SQLite) SaveTag(in TagInput, actor string) error {
	if !slugRe.MatchString(in.Slug) {
		return fmt.Errorf("%w: slug", ErrValidation)
	}
	if strings.TrimSpace(in.Name["en"]) == "" {
		return fmt.Errorf("%w: name_en", ErrValidation)
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	action := "save"
	var exists int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM tags WHERE slug=?`, in.Slug).Scan(&exists)
	if exists == 0 {
		action = "create"
		if in.Sort == 0 {
			_ = tx.QueryRow(`SELECT COALESCE(MAX(sort),0)+1 FROM tags`).Scan(&in.Sort)
		}
		if _, err := tx.Exec(`INSERT INTO tags VALUES(?,?,?,?,?)`, in.Slug, in.Sort, in.Name["en"], in.Name["ru"], in.Name["ka"]); err != nil {
			return err
		}
	} else if _, err := tx.Exec(`UPDATE tags SET name_en=?, name_ru=?, name_ka=? WHERE slug=?`, in.Name["en"], in.Name["ru"], in.Name["ka"], in.Slug); err != nil {
		return err
	}
	pages := []string{"/coaches"}
	for _, slug := range s.coachSlugsWithTag(tx, in.Slug) {
		pages = append(pages, "/coaches/"+slug)
	}
	if err := s.touch(tx, stamp, pages...); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "tag", in.Slug, action, in.Name["en"]); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

func (s *SQLite) coachSlugsWithTag(ex querier, tag string) []string {
	var out []string
	rows, err := ex.Query(`SELECT c.slug FROM coaches c JOIN coach_tags t ON t.coach_id=c.id WHERE t.tag_slug=?`, tag)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		if rows.Scan(&slug) == nil {
			out = append(out, slug)
		}
	}
	return out
}

// DeleteTag removes a tag; coach_tags rows cascade, so the tag simply
// disappears from every coach.
func (s *SQLite) DeleteTag(slug, actor string) error {
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	pages := []string{"/coaches"}
	for _, cs := range s.coachSlugsWithTag(tx, slug) {
		pages = append(pages, "/coaches/"+cs)
	}
	var name string
	_ = tx.QueryRow(`SELECT name_en FROM tags WHERE slug=?`, slug).Scan(&name)
	res, err := tx.Exec(`DELETE FROM tags WHERE slug=?`, slug)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := s.touch(tx, stamp, pages...); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "tag", slug, "delete", name); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

func (s *SQLite) MoveTag(slug, dir, actor string) error {
	var name string
	_ = s.db.QueryRow(`SELECT name_en FROM tags WHERE slug=?`, slug).Scan(&name)
	return s.Move("tags", "slug", slug, dir, []string{"/coaches"}, "tag", actor, name)
}
