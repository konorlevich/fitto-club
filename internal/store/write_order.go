package store

import (
	"fmt"
	"slices"
)

// Move swaps a row with its neighbour in display order and renumbers the
// list 1..n, so sort values stay dense. dir is "up" or "down". Deleted rows
// keep their sort but are skipped.
func (s *SQLite) Move(table, keyCol, id, dir string, pages []string, entity, actor, summary string) error {
	if dir != "up" && dir != "down" {
		return fmt.Errorf("%w: dir", ErrValidation)
	}
	stamp := s.stamp()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	where := ""
	if table != "tags" {
		where = ` WHERE deleted_at=''`
	}
	rows, err := tx.Query(`SELECT ` + keyCol + ` FROM ` + table + where + ` ORDER BY sort, ` + keyCol)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, k)
	}
	rows.Close()
	i := slices.Index(ids, id)
	if i < 0 {
		return ErrNotFound
	}
	j := i - 1
	if dir == "down" {
		j = i + 1
	}
	if j < 0 || j >= len(ids) {
		return nil // already at the edge: nothing to do
	}
	ids[i], ids[j] = ids[j], ids[i]
	for n, k := range ids {
		if _, err := tx.Exec(`UPDATE `+table+` SET sort=? WHERE `+keyCol+`=?`, n+1, k); err != nil {
			return err
		}
	}
	if err := s.touch(tx, stamp, pages...); err != nil {
		return err
	}
	if err := s.logChange(tx, stamp, actor, entity, id, "move", summary); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.Reload()
}
