package store

import (
	"fmt"
	"regexp"
	"time"

	"github.com/konorlevich/fitto-club/internal/content"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var clockRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// ValidClock reports whether s is "HH:MM".
func ValidClock(s string) bool { return clockRe.MatchString(s) }

// SaveSpecialDay upserts a holiday or a shortened day. Empty Open means
// closed all day. Only the home page carries the visit block whose content
// changes, so only its date moves (like SetHours).
func (s *SQLite) SaveSpecialDay(d content.SpecialDay, actor string) error {
	if !dateRe.MatchString(d.Date) {
		return fmt.Errorf("%w: date", ErrValidation)
	}
	if _, err := time.Parse("2006-01-02", d.Date); err != nil {
		return fmt.Errorf("%w: date", ErrValidation)
	}
	if d.Open != "" || d.Close != "" {
		if !ValidClock(d.Open) || !ValidClock(d.Close) || d.Open >= d.Close {
			return fmt.Errorf("%w: hours", ErrValidation)
		}
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO special_days(day,opens,closes,note_en,note_ru,note_ka) VALUES(?,?,?,?,?,?)
		ON CONFLICT(day) DO UPDATE SET opens=excluded.opens, closes=excluded.closes, note_en=excluded.note_en, note_ru=excluded.note_ru, note_ka=excluded.note_ka`,
		d.Date, d.Open, d.Close, d.Note["en"], d.Note["ru"], d.Note["ka"]); err != nil {
		return err
	}
	if err := s.touch(tx, stamp, "/"); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "special", d.Date, "save", d.Date); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}

// DeleteSpecialDay removes a special day. It is a small dated fact, not a
// record with history: the changelog keeps the trace.
func (s *SQLite) DeleteSpecialDay(date, actor string) error {
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM special_days WHERE day=?`, date)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := s.touch(tx, stamp, "/"); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, "special", date, "delete", date); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}
