package store

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
)

type ClassInput struct {
	ID        string
	Slug      string
	Sort      int
	Visible   bool
	CoachID   string
	Name      content.L
	Desc      content.L
	Price     int
	PriceNote content.L
	Langs     []string
	Slots     []content.Slot
}

func (in ClassInput) Validate() error {
	var bad []string
	if !slugRe.MatchString(in.Slug) {
		bad = append(bad, "slug")
	}
	if strings.TrimSpace(in.Name["en"]) == "" {
		bad = append(bad, "name_en")
	}
	for _, l := range in.Langs {
		if !slices.Contains([]string{"en", "ru", "ka"}, l) {
			bad = append(bad, "lang "+l)
		}
	}
	for i, sl := range in.Slots {
		if sl.Day < 1 || sl.Day > 7 || !ValidClock(sl.Start) || sl.Minutes < 15 || sl.Minutes > 240 {
			bad = append(bad, fmt.Sprintf("slot %d", i+1))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(bad, ", "))
	}
	return nil
}

type ClassRow struct {
	content.Class
	ID      string
	CoachID string
	Visible bool
	Deleted bool
}

// ClassRows lists every class, hidden and deleted included.
func (s *SQLite) ClassRows() ([]ClassRow, error) {
	slots := map[string][]content.Slot{}
	if err := s.each(`SELECT class_id, day, start, minutes FROM class_slots ORDER BY day, start`, func(r *sql.Rows) error {
		var id string
		var sl content.Slot
		if err := r.Scan(&id, &sl.Day, &sl.Start, &sl.Minutes); err != nil {
			return err
		}
		slots[id] = append(slots[id], sl)
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
	var out []ClassRow
	err := s.each(`SELECT id, slug, sort, visible, coach_id, name_en, name_ru, name_ka, desc_en, desc_ru, desc_ka, price,
		price_note_en, price_note_ru, price_note_ka, langs, updated_at, deleted_at FROM classes ORDER BY sort, slug`, func(r *sql.Rows) error {
		var c ClassRow
		var vis int
		var langs, del string
		var n, d, p [3]string
		if err := r.Scan(&c.ID, &c.Slug, &c.Sort, &vis, &c.CoachID, &n[0], &n[1], &n[2], &d[0], &d[1], &d[2], &c.Price,
			&p[0], &p[1], &p[2], &langs, &c.Updated, &del); err != nil {
			return err
		}
		c.Name, c.Desc, c.PriceNote, c.Coach, c.Slots = l3(n), l3(d), l3(p), slugOf[c.CoachID], slots[c.ID]
		c.Visible, c.Deleted = vis == 1, del != ""
		if langs != "" {
			c.Langs = strings.Split(langs, ",")
		}
		out = append(out, c)
		return nil
	})
	return out, err
}

func (s *SQLite) ClassRow(id string) (ClassRow, error) {
	rows, err := s.ClassRows()
	if err != nil {
		return ClassRow{}, err
	}
	for _, r := range rows {
		if r.ID == id {
			return r, nil
		}
	}
	return ClassRow{}, ErrNotFound
}

func classPages(coachSlugs ...string) []string {
	pages := []string{"/", "/classes"}
	for _, sl := range coachSlugs {
		if sl != "" {
			pages = append(pages, "/coaches/"+sl)
		}
	}
	return pages
}

// SaveClass writes a class and replaces its slots in one transaction
// (checklist §12). The class list, the home teaser and the pages of the old
// and new coach get a fresh date.
func (s *SQLite) SaveClass(in ClassInput, actor string) (string, error) {
	if err := in.Validate(); err != nil {
		return "", err
	}
	action := "save"
	if in.ID == "" {
		in.ID = NewID("k-")
		action = "create"
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var oldCoach string
	_ = tx.QueryRow(`SELECT coach_id FROM classes WHERE id=?`, in.ID).Scan(&oldCoach)
	if _, err := tx.Exec(`INSERT INTO classes(id,slug,sort,visible,coach_id,name_en,name_ru,name_ka,desc_en,desc_ru,desc_ka,price,price_note_en,price_note_ru,price_note_ka,langs,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET slug=excluded.slug, sort=excluded.sort, visible=excluded.visible, coach_id=excluded.coach_id,
		name_en=excluded.name_en, name_ru=excluded.name_ru, name_ka=excluded.name_ka, desc_en=excluded.desc_en, desc_ru=excluded.desc_ru, desc_ka=excluded.desc_ka,
		price=excluded.price, price_note_en=excluded.price_note_en, price_note_ru=excluded.price_note_ru, price_note_ka=excluded.price_note_ka, langs=excluded.langs, updated_at=excluded.updated_at`,
		in.ID, in.Slug, in.Sort, b2i(in.Visible), in.CoachID, in.Name["en"], in.Name["ru"], in.Name["ka"], in.Desc["en"], in.Desc["ru"], in.Desc["ka"],
		in.Price, in.PriceNote["en"], in.PriceNote["ru"], in.PriceNote["ka"], strings.Join(in.Langs, ","), stamp); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`DELETE FROM class_slots WHERE class_id=?`, in.ID); err != nil {
		return "", err
	}
	for i, sl := range in.Slots {
		if _, err := tx.Exec(`INSERT INTO class_slots VALUES(?,?,?,?,?)`, fmt.Sprintf("%s-%d", in.ID, i), in.ID, sl.Day, sl.Start, sl.Minutes); err != nil {
			return "", err
		}
	}
	var slugs []string
	for _, cid := range []string{oldCoach, in.CoachID} {
		var slug string
		if cid != "" && tx.QueryRow(`SELECT slug FROM coaches WHERE id=?`, cid).Scan(&slug) == nil {
			slugs = append(slugs, slug)
		}
	}
	if err := s.touch(tx, stamp, classPages(slugs...)...); err != nil {
		return "", err
	}
	if err := s.logChange(tx, stamp, actor, "class", in.ID, action, in.Name["en"]); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return in.ID, s.Reload()
}

func (s *SQLite) DeleteClass(id, actor string) error {
	r, err := s.ClassRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("classes", "class", id, true, classPages(r.Coach), actor, r.Name.Get("en"))
}

func (s *SQLite) RestoreClass(id, actor string) error {
	r, err := s.ClassRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("classes", "class", id, false, classPages(r.Coach), actor, r.Name.Get("en"))
}

func (s *SQLite) MoveClass(id, dir, actor string) error {
	r, err := s.ClassRow(id)
	if err != nil {
		return err
	}
	return s.Move("classes", "id", id, dir, classPages(), "class", actor, r.Name.Get("en"))
}
