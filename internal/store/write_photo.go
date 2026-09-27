package store

import "fmt"

// SetCoachPhoto records an uploaded photo: the base file name under
// uploads/ and the focus point of the 4:5 crop. The card, the coach's page,
// the list and the home teaser change.
func (s *SQLite) SetCoachPhoto(id, file string, fx, fy float64, actor, action string) error {
	if file == "" || fx < 0 || fx > 1 || fy < 0 || fy > 1 {
		return fmt.Errorf("%w: photo", ErrValidation)
	}
	r, err := s.CoachRow(id)
	if err != nil {
		return err
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE coaches SET photo=1, photo_file=?, focus_x=?, focus_y=?, updated_at=? WHERE id=?`, file, fx, fy, stamp, id); err != nil {
		return err
	}
	if err := s.touch(tx, stamp, "/", "/coaches", "/coaches/"+r.Slug); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "coach", id, action, r.Name.Get("en")); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

// ClearCoachPhoto removes the photo; the card falls back to initials.
func (s *SQLite) ClearCoachPhoto(id, actor string) error {
	r, err := s.CoachRow(id)
	if err != nil {
		return err
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE coaches SET photo=0, photo_file='', updated_at=? WHERE id=?`, stamp, id); err != nil {
		return err
	}
	if err := s.touch(tx, stamp, "/", "/coaches", "/coaches/"+r.Slug); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "coach", id, "photo", r.Name.Get("en")); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

// PhotoBase is the public URL prefix of an uploaded photo's derivatives:
// "<base>-400.webp" and "<base>-800.webp". The focus point is part of the
// name, so a re-crop gets a new URL and immutable caching stays honest.
func PhotoBase(file string, fx, fy float64) string {
	if file == "" {
		return ""
	}
	return fmt.Sprintf("/uploads/%s-%02d%02d", file, int(fx*99+0.5), int(fy*99+0.5))
}
