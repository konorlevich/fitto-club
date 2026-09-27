package store

import (
	"testing"
	"time"

	"github.com/konorlevich/fitto-club/internal/content"
)

func open(t *testing.T) *SQLite {
	t.Helper()
	s, err := Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// The seeded database must render exactly what the seed says (TASKS #19:
// pages byte-identical to the seed version).
func TestSeedRoundTrip(t *testing.T) {
	s := open(t)
	got, want := s.Current(), SeedSnapshot()
	if len(got.Coaches) != len(want.Coaches) || len(got.Published()) != 12 {
		t.Fatalf("coaches: got %d (%d published), want %d (12 published)", len(got.Coaches), len(got.Published()), len(want.Coaches))
	}
	for i := range want.Coaches {
		g, w := got.Coaches[i], want.Coaches[i]
		if g.Slug != w.Slug || g.PTPrice != w.PTPrice || g.Currency() != w.Currency() || g.PTFrom != w.PTFrom ||
			g.Published != w.Published || len(g.Tags) != len(w.Tags) || len(g.Speaks) != len(w.Speaks) || g.Name["ka"] != w.Name["ka"] {
			t.Errorf("coach %d: got %+v, want %+v", i, g, w)
		}
	}
	if len(got.ReviewsFor("otar-chkadua")) != 4 || len(got.ReviewsFor("gocha-butbaia")) != 0 {
		t.Errorf("review links: otar %d (want 4), gocha %d (want 0)", len(got.ReviewsFor("otar-chkadua")), len(got.ReviewsFor("gocha-butbaia")))
	}
	if len(got.Classes) != 3 || got.Classes[1].Coach != "anastasia-novikova" || len(got.Classes[0].Slots) != 6 {
		t.Errorf("classes not round-tripped: %+v", got.Classes)
	}
	if got.Hours.Week != want.Hours.Week {
		t.Errorf("hours: got %v", got.Hours.Week)
	}
	if got.SingleVisit != 20 || got.MinPerMonth() != want.MinPerMonth() {
		t.Errorf("prices: single %d, min/month %d", got.SingleVisit, got.MinPerMonth())
	}
	// Seeding is not an editorial change: no page gets a fresh date.
	if len(got.LastMod) != 0 {
		t.Errorf("seed stamped revisions: %v", got.LastMod)
	}
}

func TestReopenDoesNotReseed(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	gen := s.Current().Gen
	s.Close()
	s2, err := Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if n := len(s2.Current().Coaches); n != 13 {
		t.Fatalf("reopened db has %d coaches, want 13", n)
	}
	if s2.Current().Gen == gen {
		t.Error("a new snapshot must get a new generation")
	}
}

func coachInput(s *SQLite, slug string) CoachInput {
	c, _ := s.Current().Coach(slug)
	return CoachInput{ID: "c-" + c.Slug, Slug: c.Slug, Sort: c.Sort, Published: c.Published, Photo: c.Photo,
		Instagram: c.Instagram, Name: c.Name, Bio: c.Bio, Tags: c.Tags, Speaks: c.Speaks,
		PTPrice: c.PTPrice, PTCurrency: c.Currency(), PTFrom: c.PTFrom}
}

// One edit moves exactly the touched URLs (TASKS #19, checklist §6).
func TestSaveCoachStampsOnlyTouchedPages(t *testing.T) {
	s := open(t)
	s.now = func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC) }
	gen := s.Current().Gen
	in := coachInput(s, "gocha-butbaia")
	in.PTPrice = 90
	if err := s.SaveCoach(in, "test"); err != nil {
		t.Fatal(err)
	}
	snap := s.Current()
	if snap.Gen == gen {
		t.Error("save must rebuild the snapshot")
	}
	c, _ := snap.Coach("gocha-butbaia")
	if c.PTPrice != 90 {
		t.Errorf("price not saved: %d", c.PTPrice)
	}
	if snap.Date("/coaches/gocha-butbaia") != "2026-10-05" || snap.Date("/coaches") != "2026-10-05" {
		t.Errorf("touched pages not stamped: %v", snap.LastMod)
	}
	if snap.Date("/memberships") != content.BaselineContentDate || snap.Date("/coaches/otar-chkadua") != content.BaselineContentDate {
		t.Errorf("untouched pages moved: %v", snap.LastMod)
	}
}

func TestSlugChangeKeepsHistory(t *testing.T) {
	s := open(t)
	in := coachInput(s, "vlad-zapolskikh")
	in.Slug = "vladislav-zapolskikh"
	if err := s.SaveCoach(in, "test"); err != nil {
		t.Fatal(err)
	}
	snap := s.Current()
	if got := snap.SlugHistory["vlad-zapolskikh"]; got != "vladislav-zapolskikh" {
		t.Fatalf("history: %v", snap.SlugHistory)
	}
	// Reviews and classes follow the coach by id, not by slug.
	if n := len(snap.ReviewsFor("vladislav-zapolskikh")); n != 3 {
		t.Errorf("reviews after rename: %d, want 3", n)
	}
	// Renaming back removes the redirect for the reclaimed slug.
	in.Slug = "vlad-zapolskikh"
	if err := s.SaveCoach(in, "test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Current().SlugHistory["vlad-zapolskikh"]; ok {
		t.Error("a reclaimed slug must not redirect to itself")
	}
}

func TestSoftDeleteUnpublishes(t *testing.T) {
	s := open(t)
	if err := s.DeleteCoach("c-joni-nadoyan", "test"); err != nil {
		t.Fatal(err)
	}
	c, ok := s.Current().Coach("joni-nadoyan")
	if !ok || c.Published {
		t.Fatalf("soft-deleted coach must stay known and unpublished: ok=%v %+v", ok, c)
	}
}

func TestValidation(t *testing.T) {
	s := open(t)
	in := coachInput(s, "gocha-butbaia")
	in.Bio = content.L{"en": "only english"}
	if err := s.SaveCoach(in, "test"); err == nil {
		t.Error("a published coach with missing translations must be rejected")
	}
	in.Slug = "Bad Slug"
	in.Published = false
	if err := s.SaveCoach(in, "test"); err == nil {
		t.Error("a malformed slug must be rejected")
	}
	var week [7]content.DayHours
	week[0] = content.DayHours{Open: "23:00", Close: "08:00"}
	if err := s.SetHours(week, "test"); err == nil {
		t.Error("closing before opening must be rejected")
	}
}

// The restore rehearsal (checklist §4): back up, change, restore, and the
// change is gone.
func TestBackupRestoreRehearsal(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	bak := dir + "/backups/test.db"
	if err := s.Backup(bak); err != nil {
		t.Fatal(err)
	}
	in := coachInput(s, "gocha-butbaia")
	in.PTPrice = 999
	if err := s.SaveCoach(in, "test"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if err := RestoreFrom(bak, dir); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if c, _ := s2.Current().Coach("gocha-butbaia"); c.PTPrice != 85 {
		t.Errorf("restored price = %d, want the backed-up 85", c.PTPrice)
	}
}
