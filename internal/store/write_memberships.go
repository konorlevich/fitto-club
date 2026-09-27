package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
)

type MembershipInput struct {
	ID      string
	Slug    string
	Sort    int
	Visible bool
	Name    content.L
	Desc    content.L
	Terms   []content.Term // months 1, 3, 6, 12; Price 0 = not sold
	GiftPT  bool
}

var termMonths = []int{1, 3, 6, 12}

func (in MembershipInput) Validate() error {
	var bad []string
	if !slugRe.MatchString(in.Slug) {
		bad = append(bad, "slug")
	}
	if strings.TrimSpace(in.Name["en"]) == "" {
		bad = append(bad, "name_en")
	}
	sold := 0
	for _, t := range in.Terms {
		if t.Price < 0 || t.FreezeWeeks < 0 {
			bad = append(bad, fmt.Sprintf("term %d", t.Months))
		}
		if t.Price > 0 {
			sold++
		}
	}
	if in.Visible && sold == 0 {
		bad = append(bad, "no term sold")
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(bad, ", "))
	}
	return nil
}

type MembershipRow struct {
	content.Membership
	ID       string
	Visible  bool
	Deleted  bool
	AllTerms []content.Term // all four, sold or not
}

func (s *SQLite) MembershipRows() ([]MembershipRow, error) {
	terms := map[string]map[int]content.Term{}
	if err := s.each(`SELECT membership_id, months, price, freeze_weeks FROM membership_terms`, func(r *sql.Rows) error {
		var id string
		var t content.Term
		if err := r.Scan(&id, &t.Months, &t.Price, &t.FreezeWeeks); err != nil {
			return err
		}
		if terms[id] == nil {
			terms[id] = map[int]content.Term{}
		}
		terms[id][t.Months] = t
		return nil
	}); err != nil {
		return nil, err
	}
	var out []MembershipRow
	err := s.each(`SELECT id, slug, sort, visible, name_en, name_ru, name_ka, desc_en, desc_ru, desc_ka, gift_pt, updated_at, deleted_at FROM memberships ORDER BY sort, slug`, func(r *sql.Rows) error {
		var m MembershipRow
		var vis, gift int
		var del string
		var n, d [3]string
		if err := r.Scan(&m.ID, &m.Slug, &m.Sort, &vis, &n[0], &n[1], &n[2], &d[0], &d[1], &d[2], &gift, &m.Updated, &del); err != nil {
			return err
		}
		m.Name, m.Desc, m.GiftPT, m.Visible, m.Deleted = l3(n), l3(d), gift == 1, vis == 1, del != ""
		for _, mo := range termMonths {
			t := terms[m.ID][mo]
			t.Months = mo
			m.AllTerms = append(m.AllTerms, t)
			if t.Price > 0 {
				m.Terms = append(m.Terms, t)
			}
		}
		out = append(out, m)
		return nil
	})
	return out, err
}

func (s *SQLite) MembershipRow(id string) (MembershipRow, error) {
	rows, err := s.MembershipRows()
	if err != nil {
		return MembershipRow{}, err
	}
	for _, r := range rows {
		if r.ID == id {
			return r, nil
		}
	}
	return MembershipRow{}, ErrNotFound
}

var membershipPages = []string{"/", "/memberships"}

func (s *SQLite) SaveMembership(in MembershipInput, actor string) (string, error) {
	if err := in.Validate(); err != nil {
		return "", err
	}
	action := "save"
	if in.ID == "" {
		in.ID = NewID("m-")
		action = "create"
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO memberships(id,slug,sort,visible,name_en,name_ru,name_ka,desc_en,desc_ru,desc_ka,gift_pt,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET slug=excluded.slug, sort=excluded.sort, visible=excluded.visible, name_en=excluded.name_en, name_ru=excluded.name_ru, name_ka=excluded.name_ka,
		desc_en=excluded.desc_en, desc_ru=excluded.desc_ru, desc_ka=excluded.desc_ka, gift_pt=excluded.gift_pt, updated_at=excluded.updated_at`,
		in.ID, in.Slug, in.Sort, b2i(in.Visible), in.Name["en"], in.Name["ru"], in.Name["ka"], in.Desc["en"], in.Desc["ru"], in.Desc["ka"], b2i(in.GiftPT), stamp); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`DELETE FROM membership_terms WHERE membership_id=?`, in.ID); err != nil {
		return "", err
	}
	for _, t := range in.Terms {
		if t.Price > 0 {
			if _, err := tx.Exec(`INSERT INTO membership_terms VALUES(?,?,?,?)`, in.ID, t.Months, t.Price, t.FreezeWeeks); err != nil {
				return "", err
			}
		}
	}
	if err := s.touch(tx, stamp, membershipPages...); err != nil {
		return "", err
	}
	if err := s.logChange(tx, stamp, actor, "membership", in.ID, action, in.Name["en"]); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return in.ID, s.Reload()
}

func (s *SQLite) DeleteMembership(id, actor string) error {
	r, err := s.MembershipRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("memberships", "membership", id, true, membershipPages, actor, r.Name.Get("en"))
}

func (s *SQLite) RestoreMembership(id, actor string) error {
	r, err := s.MembershipRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("memberships", "membership", id, false, membershipPages, actor, r.Name.Get("en"))
}

func (s *SQLite) MoveMembership(id, dir, actor string) error {
	r, err := s.MembershipRow(id)
	if err != nil {
		return err
	}
	return s.Move("memberships", "id", id, dir, membershipPages, "membership", actor, r.Name.Get("en"))
}

// SetSingleVisit stores the drop-in price (settings.single_visit).
func (s *SQLite) SetSingleVisit(price int, actor string) error {
	if price <= 0 {
		return fmt.Errorf("%w: price", ErrValidation)
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO settings(key,value,updated_at) VALUES('single_visit',?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, fmt.Sprint(price), stamp); err != nil {
		return err
	}
	if err := s.touch(tx, stamp, membershipPages...); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "settings", "single_visit", "save", fmt.Sprint(price)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

// ----------------------------------------------------------------- massage

type MassageInput struct {
	ID        string
	Sort      int
	Visible   bool
	Name      content.L
	Minutes   int
	Price     int
	PackCount int
	PackPrice int
}

func (in MassageInput) Validate() error {
	var bad []string
	if strings.TrimSpace(in.Name["en"]) == "" {
		bad = append(bad, "name_en")
	}
	if in.Minutes <= 0 {
		bad = append(bad, "minutes")
	}
	if in.Price <= 0 {
		bad = append(bad, "price")
	}
	if (in.PackCount > 0) != (in.PackPrice > 0) {
		bad = append(bad, "pack")
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(bad, ", "))
	}
	return nil
}

type MassageRow struct {
	content.MassageService
	ID      string
	Visible bool
	Deleted bool
}

func (s *SQLite) MassageRows() ([]MassageRow, error) {
	var out []MassageRow
	err := s.each(`SELECT id, sort, visible, name_en, name_ru, name_ka, minutes, price, pack_count, pack_price, deleted_at FROM massage ORDER BY sort, id`, func(r *sql.Rows) error {
		var m MassageRow
		var vis int
		var del string
		var n [3]string
		if err := r.Scan(&m.ID, &m.Sort, &vis, &n[0], &n[1], &n[2], &m.Minutes, &m.Price, &m.PackCount, &m.PackPrice, &del); err != nil {
			return err
		}
		m.Name, m.Visible, m.Deleted = l3(n), vis == 1, del != ""
		out = append(out, m)
		return nil
	})
	return out, err
}

func (s *SQLite) MassageRow(id string) (MassageRow, error) {
	rows, err := s.MassageRows()
	if err != nil {
		return MassageRow{}, err
	}
	for _, r := range rows {
		if r.ID == id {
			return r, nil
		}
	}
	return MassageRow{}, ErrNotFound
}

var massagePages = []string{"/", "/massage"}

func (s *SQLite) SaveMassage(in MassageInput, actor string) (string, error) {
	if err := in.Validate(); err != nil {
		return "", err
	}
	action := "save"
	if in.ID == "" {
		in.ID = NewID("s-")
		action = "create"
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO massage(id,sort,visible,name_en,name_ru,name_ka,minutes,price,pack_count,pack_price,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET sort=excluded.sort, visible=excluded.visible, name_en=excluded.name_en, name_ru=excluded.name_ru, name_ka=excluded.name_ka,
		minutes=excluded.minutes, price=excluded.price, pack_count=excluded.pack_count, pack_price=excluded.pack_price, updated_at=excluded.updated_at`,
		in.ID, in.Sort, b2i(in.Visible), in.Name["en"], in.Name["ru"], in.Name["ka"], in.Minutes, in.Price, in.PackCount, in.PackPrice, stamp); err != nil {
		return "", err
	}
	if err := s.touch(tx, stamp, massagePages...); err != nil {
		return "", err
	}
	if err := s.logChange(tx, stamp, actor, "massage", in.ID, action, in.Name["en"]); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return in.ID, s.Reload()
}

func (s *SQLite) DeleteMassage(id, actor string) error {
	r, err := s.MassageRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("massage", "massage", id, true, massagePages, actor, r.Name.Get("en"))
}

func (s *SQLite) RestoreMassage(id, actor string) error {
	r, err := s.MassageRow(id)
	if err != nil {
		return err
	}
	return s.setDeleted("massage", "massage", id, false, massagePages, actor, r.Name.Get("en"))
}

func (s *SQLite) MoveMassage(id, dir, actor string) error {
	r, err := s.MassageRow(id)
	if err != nil {
		return err
	}
	return s.Move("massage", "id", id, dir, massagePages, "massage", actor, r.Name.Get("en"))
}
