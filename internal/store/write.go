package store

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
)

// Writes are the admin's (separate brief). Every write runs in one
// transaction with its page revisions and changelog row, then rebuilds the
// snapshot: the page cache keys on Snapshot.Gen, so the next request renders
// fresh bytes and the sitemap's lastmod moves for exactly the touched URLs
// (checklist §6, §13).

var ErrValidation = errors.New("validation")

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type CoachInput struct {
	ID, Slug   string
	Sort       int
	Published  bool
	Photo      bool
	Instagram  string
	Name, Bio  content.L
	Tags       []string
	Speaks     []string
	PTPrice    int
	PTCurrency string
	PTFrom     bool
}

// Validate: a published coach must be complete in every locale, like the
// file copy's boot gate. Drafts may be partial.
func (in CoachInput) Validate() error {
	var bad []string
	if !slugRe.MatchString(in.Slug) {
		bad = append(bad, "slug (lowercase latin, digits and hyphens)")
	}
	if in.PTCurrency != "" && in.PTCurrency != "GEL" && in.PTCurrency != "USD" {
		bad = append(bad, "currency (GEL or USD)")
	}
	for _, l := range in.Speaks {
		if !slices.Contains(content.SpokenLanguages, l) {
			bad = append(bad, "language "+l)
		}
	}
	if in.Published {
		for _, lang := range []string{"en", "ru", "ka"} {
			if strings.TrimSpace(in.Name[lang]) == "" || strings.TrimSpace(in.Bio[lang]) == "" {
				bad = append(bad, "name and bio in "+lang)
			}
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(bad, ", "))
	}
	return nil
}

// writeCoach replaces a coach and its child rows (delete + insert inside the
// caller's transaction, checklist §12 "replace-all-in-transaction").
func (s *SQLite) writeCoach(ex execer, in CoachInput, stamp string) error {
	cur := in.PTCurrency
	if cur == "" {
		cur = "GEL"
	}
	if _, err := ex.Exec(`INSERT INTO coaches(id,slug,sort,published,photo,instagram,name_en,name_ru,name_ka,bio_en,bio_ru,bio_ka,pt_price,pt_currency,pt_from,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET slug=excluded.slug, sort=excluded.sort, published=excluded.published, photo=excluded.photo,
		instagram=excluded.instagram, name_en=excluded.name_en, name_ru=excluded.name_ru, name_ka=excluded.name_ka,
		bio_en=excluded.bio_en, bio_ru=excluded.bio_ru, bio_ka=excluded.bio_ka, pt_price=excluded.pt_price,
		pt_currency=excluded.pt_currency, pt_from=excluded.pt_from, updated_at=excluded.updated_at`,
		in.ID, in.Slug, in.Sort, b2i(in.Published), b2i(in.Photo), strings.TrimPrefix(in.Instagram, "@"),
		in.Name["en"], in.Name["ru"], in.Name["ka"], in.Bio["en"], in.Bio["ru"], in.Bio["ka"],
		in.PTPrice, cur, b2i(in.PTFrom), stamp); err != nil {
		return err
	}
	if _, err := ex.Exec(`DELETE FROM coach_tags WHERE coach_id=?`, in.ID); err != nil {
		return err
	}
	for i, t := range in.Tags {
		if _, err := ex.Exec(`INSERT INTO coach_tags VALUES(?,?,?)`, in.ID, t, i); err != nil {
			return fmt.Errorf("tag %q: %w", t, err)
		}
	}
	if _, err := ex.Exec(`DELETE FROM coach_langs WHERE coach_id=?`, in.ID); err != nil {
		return err
	}
	for i, l := range in.Speaks {
		if _, err := ex.Exec(`INSERT INTO coach_langs VALUES(?,?,?)`, in.ID, l, i); err != nil {
			return err
		}
	}
	return nil
}

// SaveCoach creates or updates a coach. A slug change keeps the old slug in
// history (301), and the touched pages get a new content date: the coach's
// page, the list, the home page (it shows four coaches), and the pages that
// name them (classes, massage for the massage therapist).
func (s *SQLite) SaveCoach(in CoachInput, actor string) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if in.ID == "" {
		in.ID = NewID("c-")
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var oldSlug string
	_ = tx.QueryRow(`SELECT slug FROM coaches WHERE id=?`, in.ID).Scan(&oldSlug)
	if oldSlug != "" && oldSlug != in.Slug {
		if _, err := tx.Exec(`INSERT INTO slug_history VALUES(?,?) ON CONFLICT(old_slug) DO UPDATE SET coach_id=excluded.coach_id`, oldSlug, in.ID); err != nil {
			return err
		}
		// A slug that is taken back by its coach stops redirecting.
		if _, err := tx.Exec(`DELETE FROM slug_history WHERE old_slug=?`, in.Slug); err != nil {
			return err
		}
	}
	if err := s.writeCoach(tx, in, stamp); err != nil {
		return err
	}
	pages := []string{"/", "/coaches", "/coaches/" + in.Slug, "/classes", "/massage"}
	if err := s.touch(tx, stamp, pages...); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "coach", in.ID, "save", in.Slug); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

// DeleteCoach soft-deletes: the row stays, its URL 301s to the list.
func (s *SQLite) DeleteCoach(id, actor string) error {
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var slug string
	if err := tx.QueryRow(`SELECT slug FROM coaches WHERE id=?`, id).Scan(&slug); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE coaches SET deleted_at=?, updated_at=? WHERE id=?`, stamp, stamp, id); err != nil {
		return err
	}
	if err := s.touch(tx, stamp, "/", "/coaches", "/classes", "/massage"); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "coach", id, "delete", slug); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

// SetHours replaces the weekly hours. Hours appear on every page (the visit
// block and the footer) but the content that changed is the visit info, so
// only the home page's date moves - a footer change is not a content change
// for the other URLs (checklist §6).
func (s *SQLite) SetHours(week [7]content.DayHours, actor string) error {
	for i, d := range week {
		if (d.Open == "") != (d.Close == "") || (d.Open != "" && d.Open >= d.Close) {
			return fmt.Errorf("%w: day %d hours", ErrValidation, i+1)
		}
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, d := range week {
		if _, err := tx.Exec(`UPDATE hours SET opens=?, closes=?, updated_at=? WHERE day=?`, d.Open, d.Close, stamp, i+1); err != nil {
			return err
		}
	}
	if err := s.touch(tx, stamp, "/"); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "hours", "week", "save", ""); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

func (s *SQLite) touch(ex execer, stamp string, pages ...string) error {
	if s.seeding {
		return nil
	}
	for _, p := range pages {
		if _, err := ex.Exec(`INSERT INTO page_revisions(page, updated_at) VALUES(?,?)
			ON CONFLICT(page) DO UPDATE SET updated_at=excluded.updated_at`, p, stamp); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLite) logChange(ex execer, stamp, actor, entity, id, action, summary string) error {
	_, err := ex.Exec(`INSERT INTO changelog VALUES(?,?,?,?,?,?,?)`, NewID("l-"), stamp, actor, entity, id, action, summary)
	return err
}
