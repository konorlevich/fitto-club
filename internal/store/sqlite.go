package store

// SQLite on a Railway volume (BRIEF.md §4): one instance, one or two
// editors, a few hundred rows written a handful of times a month. A file on
// a volume needs no second service and a backup is a file copy. The driver
// is modernc.org/sqlite, pure Go, so the deploy stays one static binary.

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type SQLite struct {
	db   *sql.DB
	dir  string
	snap atomic.Pointer[Snapshot]
	log  *logrus.Logger
	// seeding suppresses revision stamps while the shipped defaults are
	// imported: that content already existed at BaselineContentDate, and a
	// fresh environment must publish the same dates as an old one.
	seeding bool
	// now is the clock; tests pin it.
	now func() time.Time
}

// Open connects, migrates, seeds an empty database and loads the snapshot.
// Any failure is fatal to boot.
func Open(dataDir string, log *logrus.Logger) (*SQLite, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "uploads"), 0o755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	// WAL: readers never block the writer. busy_timeout: a concurrent save
	// waits instead of failing. foreign_keys: off by default in SQLite, and
	// ON DELETE CASCADE silently does nothing without it.
	dsn := "file:" + filepath.Join(dataDir, "content.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	s := &SQLite{db: db, dir: dataDir, log: log, now: time.Now}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM coaches`).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		if err := s.seed(SeedSnapshot()); err != nil {
			return nil, fmt.Errorf("seeding: %w", err)
		}
		if log != nil {
			log.Info("content database seeded from the shipped defaults")
		}
	}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLite) Current() *Snapshot { return s.snap.Load() }
func (s *SQLite) Close() error       { return s.db.Close() }
func (s *SQLite) UploadDir() string  { return filepath.Join(s.dir, "uploads") }

func (s *SQLite) stamp() string { return s.now().UTC().Format(time.RFC3339) }

var idSeq atomic.Int64

// NewID is sortable and portable: no AUTOINCREMENT, no SERIAL.
func NewID(prefix string) string {
	return fmt.Sprintf("%s%d%03d", prefix, time.Now().UTC().UnixMicro(), idSeq.Add(1)%1000)
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ------------------------------------------------------------------ seed

func (s *SQLite) seed(snap *Snapshot) error {
	s.seeding = true
	defer func() { s.seeding = false }()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	base := content.BaselineContentDate
	if _, err := tx.Exec(`INSERT INTO settings(key,value,updated_at) VALUES('single_visit',?,?)`, fmt.Sprint(snap.SingleVisit), base); err != nil {
		return err
	}
	for _, m := range snap.Memberships {
		id := "m-" + m.Slug
		if _, err := tx.Exec(`INSERT INTO memberships(id,slug,sort,name_en,name_ru,name_ka,desc_en,desc_ru,desc_ka,gift_pt,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, m.Slug, m.Sort, m.Name["en"], m.Name["ru"], m.Name["ka"],
			m.Desc["en"], m.Desc["ru"], m.Desc["ka"], b2i(m.GiftPT), m.Updated); err != nil {
			return err
		}
		for _, t := range m.Terms {
			if _, err := tx.Exec(`INSERT INTO membership_terms VALUES(?,?,?,?)`, id, t.Months, t.Price, t.FreezeWeeks); err != nil {
				return err
			}
		}
	}
	for _, t := range snap.Tags {
		if _, err := tx.Exec(`INSERT INTO tags VALUES(?,?,?,?,?)`, t.Slug, t.Sort, t.Name["en"], t.Name["ru"], t.Name["ka"]); err != nil {
			return err
		}
	}
	for _, c := range snap.Coaches {
		in := CoachInput{
			ID: "c-" + c.Slug, Slug: c.Slug, Sort: c.Sort, Published: c.Published, Photo: c.Photo,
			Instagram: c.Instagram, Name: c.Name, Bio: c.Bio, Tags: c.Tags, Speaks: c.Speaks,
			PTPrice: c.PTPrice, PTCurrency: c.Currency(), PTFrom: c.PTFrom,
		}
		if err := s.writeCoach(tx, in, base); err != nil {
			return err
		}
	}
	for _, r := range snap.Reviews {
		if _, err := tx.Exec(`INSERT INTO reviews(id,author,lang,body,url,rating,written,home,sort,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			r.ID, r.Author, r.Lang, r.Text, r.URL, r.Rating, r.Date, b2i(r.Home), r.Sort, base); err != nil {
			return err
		}
		for _, slug := range r.Coaches {
			if _, err := tx.Exec(`INSERT INTO review_coaches VALUES(?,?)`, r.ID, "c-"+slug); err != nil {
				return err
			}
		}
	}
	for _, c := range snap.Classes {
		coachID := ""
		if c.Coach != "" {
			coachID = "c-" + c.Coach
		}
		id := "k-" + c.Slug
		if _, err := tx.Exec(`INSERT INTO classes(id,slug,sort,coach_id,name_en,name_ru,name_ka,desc_en,desc_ru,desc_ka,price,price_note_en,price_note_ru,price_note_ka,langs,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, c.Slug, c.Sort, coachID,
			c.Name["en"], c.Name["ru"], c.Name["ka"], c.Desc["en"], c.Desc["ru"], c.Desc["ka"], c.Price,
			c.PriceNote["en"], c.PriceNote["ru"], c.PriceNote["ka"], strings.Join(c.Langs, ","), c.Updated); err != nil {
			return err
		}
		for i, sl := range c.Slots {
			if _, err := tx.Exec(`INSERT INTO class_slots VALUES(?,?,?,?,?)`, fmt.Sprintf("%s-%d", id, i), id, sl.Day, sl.Start, sl.Minutes); err != nil {
				return err
			}
		}
	}
	for i, m := range snap.Massage {
		if _, err := tx.Exec(`INSERT INTO massage(id,sort,name_en,name_ru,name_ka,minutes,price,pack_count,pack_price,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			fmt.Sprintf("s-%02d", i+1), m.Sort, m.Name["en"], m.Name["ru"], m.Name["ka"], m.Minutes, m.Price, m.PackCount, m.PackPrice, base); err != nil {
			return err
		}
	}
	for i, d := range snap.Hours.Week {
		if _, err := tx.Exec(`INSERT INTO hours VALUES(?,?,?,?)`, i+1, d.Open, d.Close, base); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ------------------------------------------------------------------ read

// Reload rebuilds the snapshot from the database and swaps it in atomically.
func (s *SQLite) Reload() error {
	snap := &Snapshot{Gen: nextGen(), SlugHistory: map[string]string{}, LastMod: map[string]string{}}
	var sv string
	_ = s.db.QueryRow(`SELECT value FROM settings WHERE key='single_visit'`).Scan(&sv)
	fmt.Sscan(sv, &snap.SingleVisit)

	terms := map[string][]content.Term{}
	if err := s.each(`SELECT membership_id, months, price, freeze_weeks FROM membership_terms ORDER BY months`, func(r *sql.Rows) error {
		var id string
		var t content.Term
		if err := r.Scan(&id, &t.Months, &t.Price, &t.FreezeWeeks); err != nil {
			return err
		}
		terms[id] = append(terms[id], t)
		return nil
	}); err != nil {
		return fmt.Errorf("terms: %w", err)
	}
	if err := s.each(`SELECT id, slug, sort, name_en, name_ru, name_ka, desc_en, desc_ru, desc_ka, gift_pt, updated_at
		FROM memberships WHERE visible=1 AND deleted_at='' ORDER BY sort, slug`, func(r *sql.Rows) error {
		var m content.Membership
		var id string
		var n, d [3]string
		var gift int
		if err := r.Scan(&id, &m.Slug, &m.Sort, &n[0], &n[1], &n[2], &d[0], &d[1], &d[2], &gift, &m.Updated); err != nil {
			return err
		}
		m.Name, m.Desc, m.GiftPT, m.Terms = l3(n), l3(d), gift == 1, terms[id]
		snap.Memberships = append(snap.Memberships, m)
		return nil
	}); err != nil {
		return fmt.Errorf("memberships: %w", err)
	}

	if err := s.each(`SELECT slug, sort, name_en, name_ru, name_ka FROM tags ORDER BY sort, slug`, func(r *sql.Rows) error {
		var t content.Tag
		var n [3]string
		if err := r.Scan(&t.Slug, &t.Sort, &n[0], &n[1], &n[2]); err != nil {
			return err
		}
		t.Name = l3(n)
		snap.Tags = append(snap.Tags, t)
		return nil
	}); err != nil {
		return fmt.Errorf("tags: %w", err)
	}

	slugOf := map[string]string{}
	coachTags := map[string][]string{}
	coachLangs := map[string][]string{}
	if err := s.each(`SELECT coach_id, tag_slug FROM coach_tags ORDER BY sort`, func(r *sql.Rows) error {
		var c, t string
		if err := r.Scan(&c, &t); err != nil {
			return err
		}
		coachTags[c] = append(coachTags[c], t)
		return nil
	}); err != nil {
		return err
	}
	if err := s.each(`SELECT coach_id, lang FROM coach_langs ORDER BY sort`, func(r *sql.Rows) error {
		var c, l string
		if err := r.Scan(&c, &l); err != nil {
			return err
		}
		coachLangs[c] = append(coachLangs[c], l)
		return nil
	}); err != nil {
		return err
	}
	// Soft-deleted coaches stay in the snapshot as unpublished: their URL must
	// still 301 to the list rather than 404 (BRIEF.md §4).
	if err := s.each(`SELECT id, slug, sort, published, photo, instagram, name_en, name_ru, name_ka, bio_en, bio_ru, bio_ka,
		pt_price, pt_currency, pt_from, updated_at, deleted_at, photo_file, focus_x, focus_y FROM coaches ORDER BY sort, slug`, func(r *sql.Rows) error {
		var c content.Coach
		var id, deleted, file string
		var pub, photo, from int
		var fx, fy float64
		var n, b [3]string
		if err := r.Scan(&id, &c.Slug, &c.Sort, &pub, &photo, &c.Instagram, &n[0], &n[1], &n[2], &b[0], &b[1], &b[2],
			&c.PTPrice, &c.PTCurrency, &from, &c.Updated, &deleted, &file, &fx, &fy); err != nil {
			return err
		}
		c.Published = pub == 1 && deleted == ""
		c.Photo, c.PTFrom = photo == 1, from == 1
		c.PhotoBase = PhotoBase(file, fx, fy)
		c.Name, c.Bio = l3(n), l3(b)
		c.Tags, c.Speaks = coachTags[id], coachLangs[id]
		c.Updated = dateOnly(c.Updated)
		slugOf[id] = c.Slug
		snap.Coaches = append(snap.Coaches, c)
		return nil
	}); err != nil {
		return fmt.Errorf("coaches: %w", err)
	}
	if err := s.each(`SELECT old_slug, coach_id FROM slug_history`, func(r *sql.Rows) error {
		var old, id string
		if err := r.Scan(&old, &id); err != nil {
			return err
		}
		if cur := slugOf[id]; cur != "" && cur != old {
			snap.SlugHistory[old] = cur
		}
		return nil
	}); err != nil {
		return err
	}

	revCoaches := map[string][]string{}
	if err := s.each(`SELECT review_id, coach_id FROM review_coaches`, func(r *sql.Rows) error {
		var rid, cid string
		if err := r.Scan(&rid, &cid); err != nil {
			return err
		}
		if slug := slugOf[cid]; slug != "" {
			revCoaches[rid] = append(revCoaches[rid], slug)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := s.each(`SELECT id, author, lang, body, url, rating, written, home, sort FROM reviews WHERE deleted_at='' ORDER BY sort, written DESC`, func(r *sql.Rows) error {
		var v content.Review
		var home int
		if err := r.Scan(&v.ID, &v.Author, &v.Lang, &v.Text, &v.URL, &v.Rating, &v.Date, &home, &v.Sort); err != nil {
			return err
		}
		v.Home, v.Coaches = home == 1, revCoaches[v.ID]
		snap.Reviews = append(snap.Reviews, v)
		return nil
	}); err != nil {
		return fmt.Errorf("reviews: %w", err)
	}

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
		return err
	}
	if err := s.each(`SELECT id, slug, sort, coach_id, name_en, name_ru, name_ka, desc_en, desc_ru, desc_ka, price,
		price_note_en, price_note_ru, price_note_ka, langs, updated_at FROM classes WHERE visible=1 AND deleted_at='' ORDER BY sort, slug`, func(r *sql.Rows) error {
		var c content.Class
		var id, coachID, langs string
		var n, d, p [3]string
		if err := r.Scan(&id, &c.Slug, &c.Sort, &coachID, &n[0], &n[1], &n[2], &d[0], &d[1], &d[2], &c.Price,
			&p[0], &p[1], &p[2], &langs, &c.Updated); err != nil {
			return err
		}
		c.Name, c.Desc, c.PriceNote, c.Coach, c.Slots = l3(n), l3(d), l3(p), slugOf[coachID], slots[id]
		if langs != "" {
			c.Langs = strings.Split(langs, ",")
		}
		snap.Classes = append(snap.Classes, c)
		return nil
	}); err != nil {
		return fmt.Errorf("classes: %w", err)
	}

	if err := s.each(`SELECT sort, name_en, name_ru, name_ka, minutes, price, pack_count, pack_price FROM massage
		WHERE visible=1 AND deleted_at='' ORDER BY sort`, func(r *sql.Rows) error {
		var m content.MassageService
		var n [3]string
		if err := r.Scan(&m.Sort, &n[0], &n[1], &n[2], &m.Minutes, &m.Price, &m.PackCount, &m.PackPrice); err != nil {
			return err
		}
		m.Name = l3(n)
		snap.Massage = append(snap.Massage, m)
		return nil
	}); err != nil {
		return fmt.Errorf("massage: %w", err)
	}

	if err := s.each(`SELECT day, opens, closes FROM hours ORDER BY day`, func(r *sql.Rows) error {
		var d int
		var h content.DayHours
		if err := r.Scan(&d, &h.Open, &h.Close); err != nil {
			return err
		}
		if d >= 1 && d <= 7 {
			snap.Hours.Week[d-1] = h
		}
		return nil
	}); err != nil {
		return fmt.Errorf("hours: %w", err)
	}
	if err := s.each(`SELECT day, opens, closes, note_en, note_ru, note_ka FROM special_days ORDER BY day`, func(r *sql.Rows) error {
		var sd content.SpecialDay
		var n [3]string
		if err := r.Scan(&sd.Date, &sd.Open, &sd.Close, &n[0], &n[1], &n[2]); err != nil {
			return err
		}
		sd.Note = l3(n)
		snap.Hours.Special = append(snap.Hours.Special, sd)
		return nil
	}); err != nil {
		return err
	}

	if err := s.each(`SELECT page, updated_at FROM page_revisions`, func(r *sql.Rows) error {
		var p, at string
		if err := r.Scan(&p, &at); err != nil {
			return err
		}
		snap.LastMod[p] = dateOnly(at)
		return nil
	}); err != nil {
		return err
	}

	sort.SliceStable(snap.Coaches, func(i, j int) bool { return snap.Coaches[i].Sort < snap.Coaches[j].Sort })
	s.snap.Store(snap)
	return nil
}

func (s *SQLite) each(q string, fn func(*sql.Rows) error) error {
	rows, err := s.db.Query(q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func l3(v [3]string) content.L { return content.L{"en": v[0], "ru": v[1], "ka": v[2]} }

// dateOnly keeps YYYY-MM-DD of an RFC 3339 stamp: the sitemap's lastmod is
// a date, and a date is what the content honestly knows.
func dateOnly(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// SetClock pins the store's clock; tests use it to move time.
func (s *SQLite) SetClock(f func() time.Time) { s.now = f }
