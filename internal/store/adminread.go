package store

import (
	"database/sql"

	"github.com/konorlevich/fitto-club/internal/content"
)

// Admin reads see every row, including hidden and soft-deleted ones, which
// the public Snapshot filters out.

type CoachRow struct {
	content.Coach
	ID        string
	Deleted   bool
	PhotoFile string
	FocusX    float64
	FocusY    float64
}

// CoachRows lists every coach in display order.
func (s *SQLite) CoachRows() ([]CoachRow, error) {
	tags, langs := map[string][]string{}, map[string][]string{}
	if err := s.each(`SELECT coach_id, tag_slug FROM coach_tags ORDER BY sort`, func(r *sql.Rows) error {
		var c, t string
		if err := r.Scan(&c, &t); err != nil {
			return err
		}
		tags[c] = append(tags[c], t)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.each(`SELECT coach_id, lang FROM coach_langs ORDER BY sort`, func(r *sql.Rows) error {
		var c, l string
		if err := r.Scan(&c, &l); err != nil {
			return err
		}
		langs[c] = append(langs[c], l)
		return nil
	}); err != nil {
		return nil, err
	}
	var out []CoachRow
	err := s.each(`SELECT id, slug, sort, published, photo, instagram, name_en, name_ru, name_ka, bio_en, bio_ru, bio_ka,
		pt_price, pt_currency, pt_from, updated_at, deleted_at, photo_file, focus_x, focus_y FROM coaches ORDER BY sort, slug`, func(r *sql.Rows) error {
		var c CoachRow
		var deleted string
		var pub, photo, from int
		var n, b [3]string
		if err := r.Scan(&c.ID, &c.Slug, &c.Sort, &pub, &photo, &c.Instagram, &n[0], &n[1], &n[2], &b[0], &b[1], &b[2],
			&c.PTPrice, &c.PTCurrency, &from, &c.Updated, &deleted, &c.PhotoFile, &c.FocusX, &c.FocusY); err != nil {
			return err
		}
		c.Published, c.Deleted = pub == 1, deleted != ""
		c.Photo, c.PTFrom = photo == 1, from == 1
		c.PhotoBase = PhotoBase(c.PhotoFile, c.FocusX, c.FocusY)
		c.Name, c.Bio = l3(n), l3(b)
		c.Tags, c.Speaks = tags[c.ID], langs[c.ID]
		c.Updated = dateOnly(c.Updated)
		out = append(out, c)
		return nil
	})
	return out, err
}

// CoachRow finds one coach by id.
func (s *SQLite) CoachRow(id string) (CoachRow, error) {
	rows, err := s.CoachRows()
	if err != nil {
		return CoachRow{}, err
	}
	for _, r := range rows {
		if r.ID == id {
			return r, nil
		}
	}
	return CoachRow{}, ErrNotFound
}
