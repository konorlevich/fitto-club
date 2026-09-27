// Package content holds the site's data model, the per-locale copy bundles and
// the shipped seed data. The editable types (memberships, coaches, reviews,
// classes, massage, hours) are loaded into the store; everything else here is
// typed data compiled into the binary and changed by deploy (BRIEF.md §4).
package content

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// L is one string in every locale. Get falls back to English so a missing
// translation in editable data degrades to the reference text rather than to
// an empty element; the file copy has its own, stricter, completeness gate.
type L map[string]string

func (l L) Get(lang string) string {
	if v := strings.TrimSpace(l[lang]); v != "" {
		return v
	}
	return l["en"]
}

// Term is one sellable duration of a membership.
type Term struct {
	Months      int
	Price       int // GEL; 0 = this term is not sold and is hidden
	FreezeWeeks int // 0 = no freeze
}

func (t Term) PerMonth() int {
	if t.Months <= 0 || t.Price <= 0 {
		return 0
	}
	return (t.Price + t.Months/2) / t.Months
}

type Membership struct {
	Slug    string
	Name    L
	Desc    L
	Terms   []Term // month, 3, 6, 12 - only terms with a price are kept
	GiftPT  bool
	Sort    int
	Updated string
}

// Cheapest is the lowest per-month price across the sold terms.
func (m Membership) Cheapest() int {
	best := 0
	for _, t := range m.Terms {
		if p := t.PerMonth(); p > 0 && (best == 0 || p < best) {
			best = p
		}
	}
	return best
}

type Tag struct {
	Slug string
	Name L
	Sort int
}

// Languages a coach can speak. Order is the display order.
var SpokenLanguages = []string{"ka", "en", "ru", "es", "de"}

type Coach struct {
	Slug       string
	Name       L // Latin / Cyrillic / Mkhedruli
	Photo      bool
	Instagram  string // handle without @
	Bio        L
	Tags       []string // tag slugs
	Speaks     []string // SpokenLanguages codes
	PTPrice    int      // 0 = not stated
	PTCurrency string   // "GEL" (default) or "USD"
	PTFrom     bool     // "from 90"
	Published  bool
	Sort       int
	Updated    string
	Meme       *Meme
}

func (c Coach) Initials() string {
	parts := strings.Fields(c.Name.Get("en"))
	var b strings.Builder
	for _, p := range parts {
		if len(b.String()) >= 2 {
			break
		}
		b.WriteString(strings.ToUpper(p[:1]))
	}
	return b.String()
}

func (c Coach) HasTag(slug string) bool { return slices.Contains(c.Tags, slug) }
func (c Coach) Speaks1(l string) bool   { return slices.Contains(c.Speaks, l) }
func (c Coach) Currency() string {
	if c.PTCurrency == "" {
		return "GEL"
	}
	return c.PTCurrency
}

// Meme is one captioned still from the club's reels. At most one per page.
type Meme struct {
	Image   string // path under static/img
	W, H    int
	Caption L
	Alt     L
}

type Review struct {
	ID      string
	Author  string
	Lang    string // en | ru | ka - the language it was written in
	Text    string // shown in the original, never translated
	URL     string
	Rating  int
	Date    string // YYYY-MM, approximate (Google shows relative dates)
	Coaches []string
	Home    bool // selected for the home page strip
	Sort    int
}

// Weekday is ISO: 1 = Monday ... 7 = Sunday.
type Slot struct {
	Day     int
	Start   string // "19:00"
	Minutes int
}

func (s Slot) End() string {
	t, err := time.Parse("15:04", s.Start)
	if err != nil || s.Minutes <= 0 {
		return ""
	}
	return t.Add(time.Duration(s.Minutes) * time.Minute).Format("15:04")
}

type Class struct {
	Slug      string
	Name      L
	Desc      L
	Coach     string // coach slug, optional
	Slots     []Slot
	Price     int // GEL, 0 = not stated
	PriceNote L
	Langs     []string
	Sort      int
	Updated   string
}

type MassageService struct {
	Name      L
	Minutes   int
	Price     int
	PackCount int
	PackPrice int
	Sort      int
}

// DayHours is one weekday. An empty Open means closed all day.
type DayHours struct {
	Open, Close string
}

func (d DayHours) Closed() bool { return d.Open == "" }

type SpecialDay struct {
	Date        string // 2006-01-02
	Open, Close string // empty Open = closed
	Note        L
}

type Hours struct {
	Week    [7]DayHours // index 0 = Monday
	Special []SpecialDay
	Updated string
}

// On returns the hours that apply on a calendar date, special days first.
func (h Hours) On(t time.Time) (DayHours, *SpecialDay) {
	d := t.Format("2006-01-02")
	for i := range h.Special {
		if h.Special[i].Date == d {
			return DayHours{h.Special[i].Open, h.Special[i].Close}, &h.Special[i]
		}
	}
	return h.Week[ISODay(t)-1], nil
}

func ISODay(t time.Time) int {
	wd := int(t.Weekday())
	if wd == 0 {
		return 7
	}
	return wd
}

// Status is the "open now" answer, always with a time attached.
type Status struct {
	Open     bool
	Until    string // when open: closing time today
	NextOpen string // when closed: next opening time
	NextDay  int    // when closed: ISO weekday of the next opening, 0 = today
}

// Key identifies the status for caching: it changes exactly when the text a
// visitor sees would change.
func (s Status) Key() string {
	if s.Open {
		return "o" + s.Until
	}
	return fmt.Sprintf("c%s%d", s.NextOpen, s.NextDay)
}

// StatusAt computes the status at t (already in the club's time zone).
func (h Hours) StatusAt(t time.Time) Status {
	now := t.Format("15:04")
	today, _ := h.On(t)
	if !today.Closed() && now >= today.Open && now < today.Close {
		return Status{Open: true, Until: today.Close}
	}
	if !today.Closed() && now < today.Open {
		return Status{NextOpen: today.Open}
	}
	for i := 1; i <= 7; i++ {
		day := t.AddDate(0, 0, i)
		dh, _ := h.On(day)
		if !dh.Closed() {
			nd := ISODay(day)
			if i == 1 {
				nd = -1 // tomorrow
			}
			return Status{NextOpen: dh.Open, NextDay: nd}
		}
	}
	return Status{}
}

// Zone is one area of the club, typed data (changes by deploy).
type Zone struct {
	Key   string
	Name  L
	Text  L
	Photo string // base name under static/img/place, without width suffix
	W, H  int
	Alt   L
}

// Partner is typed data for the About page.
type Partner struct {
	Name string
	Text L
	URL  string
}
