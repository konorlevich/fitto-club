package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Backup writes a consistent copy of the database with VACUUM INTO: safe
// while the site is serving, and the result is a plain SQLite file that
// Open can use directly, which is the whole restore procedure (DEPLOY.md).
func (s *SQLite) Backup(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_ = os.Remove(path) // VACUUM INTO refuses to overwrite
	if _, err := s.db.Exec(`VACUUM INTO ?`, path); err != nil {
		return fmt.Errorf("backup to %s: %w", path, err)
	}
	return nil
}

// BackupLoop takes a daily snapshot into DATA_DIR/backups and keeps the last
// `keep`. It runs until ctx is cancelled; failures are reported, not fatal.
func (s *SQLite) BackupLoop(ctx context.Context, keep int, report func(error)) {
	dir := filepath.Join(s.dir, "backups")
	run := func() {
		name := filepath.Join(dir, "content-"+time.Now().UTC().Format("20060102")+".db")
		if err := s.Backup(name); err != nil {
			report(err)
			return
		}
		files, _ := filepath.Glob(filepath.Join(dir, "content-*.db"))
		sort.Strings(files)
		for len(files) > keep {
			_ = os.Remove(files[0])
			files = files[1:]
		}
	}
	run()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}

// RestoreFrom replaces the live database file with a backup. Used by the
// restore rehearsal; in production the procedure is to stop the service,
// copy the file, start it (DEPLOY.md).
func RestoreFrom(backup, dataDir string) error {
	if !strings.HasSuffix(backup, ".db") {
		return fmt.Errorf("not a database file: %s", backup)
	}
	b, err := os.ReadFile(backup)
	if err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(filepath.Join(dataDir, "content.db"+suffix))
	}
	return os.WriteFile(filepath.Join(dataDir, "content.db"), b, 0o644)
}
