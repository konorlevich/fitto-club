package content

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Every shipped locale passes the boot gate: no empty string, right shapes.
func TestLocalesComplete(t *testing.T) {
	fsys := os.DirFS("../..")
	for _, lang := range []string{"en", "ru", "ka"} {
		if _, err := LoadCopy(fsys, lang); err != nil {
			t.Errorf("%s: %v", lang, err)
		}
	}
}

// Formats with %s/%d must keep their verbs in every locale; a translator
// dropping one would print "%!(EXTRA ...)" on the page.
func TestFormatVerbsMatch(t *testing.T) {
	fsys := os.DirFS("../..")
	en, _ := LoadCopy(fsys, "en")
	for _, lang := range []string{"ru", "ka"} {
		c, _ := LoadCopy(fsys, lang)
		pairs := map[string][2]string{
			"open_until": {en.Common.OpenUntil, c.Common.OpenUntil}, "closed_opens_day": {en.Common.ClosedOpensDay, c.Common.ClosedOpensDay},
			"rating_line": {en.Common.RatingLine, c.Common.RatingLine}, "from": {en.Common.From, c.Common.From},
			"per_month": {en.Memberships.PerMonth, c.Memberships.PerMonth}, "freeze": {en.Memberships.Freeze, c.Memberships.Freeze},
			"pack": {en.Massage.Pack, c.Massage.Pack}, "found": {en.Coaches.Found, c.Coaches.Found},
			"classes_next": {en.Home.ClassesNext, c.Home.ClassesNext}, "coaches_all": {en.Home.CoachesAll, c.Home.CoachesAll},
		}
		for k, p := range pairs {
			for _, verb := range []string{"%s", "%d"} {
				if strings.Count(p[0], verb) != strings.Count(p[1], verb) {
					t.Errorf("%s %s: %q vs en %q", lang, k, p[1], p[0])
				}
			}
		}
	}
}

func TestStatus(t *testing.T) {
	h := SeedHours
	tz := time.FixedZone("Tbilisi", 4*3600)
	at := func(day, hm string) Status {
		d, _ := time.ParseInLocation("2006-01-02 15:04", day+" "+hm, tz)
		return h.StatusAt(d)
	}
	// 2026-09-28 is a Monday (08:00-23:00), 2026-09-27 a Sunday (09:00-22:00).
	cases := []struct {
		day, hm string
		want    Status
	}{
		{"2026-09-28", "07:59", Status{NextOpen: "08:00"}},
		{"2026-09-28", "08:00", Status{Open: true, Until: "23:00"}},
		{"2026-09-28", "22:59", Status{Open: true, Until: "23:00"}},
		{"2026-09-28", "23:00", Status{NextOpen: "08:00", NextDay: -1}},
		{"2026-09-27", "22:30", Status{NextOpen: "08:00", NextDay: -1}},
		{"2026-10-02", "23:30", Status{NextOpen: "09:00", NextDay: -1}}, // Friday night -> Saturday
	}
	for _, c := range cases {
		if got := at(c.day, c.hm); got != c.want {
			t.Errorf("%s %s: got %+v, want %+v", c.day, c.hm, got, c.want)
		}
	}
	// A closed special day moves the next opening to the following day.
	h.Special = []SpecialDay{{Date: "2026-09-29"}}
	d, _ := time.ParseInLocation("2006-01-02 15:04", "2026-09-28 23:30", tz)
	if got := h.StatusAt(d); got.NextDay != 3 || got.NextOpen != "08:00" {
		t.Errorf("special closed day: %+v", got)
	}
}

func TestPerMonthAndInitials(t *testing.T) {
	if got := (Term{Months: 12, Price: 1550}).PerMonth(); got != 129 {
		t.Errorf("per month: %d", got)
	}
	if got := (Coach{Name: L{"en": "Evan Kostylev"}}).Initials(); got != "EK" {
		t.Errorf("initials: %q", got)
	}
}
