// Package store owns the editable content and the in-memory snapshot every
// page renders from. Pages never touch a database on the request path: they
// read an immutable *Snapshot, which is swapped atomically after a write.
package store

import (
	"slices"
	"sort"
	"time"

	"github.com/konorlevich/fitto-club/internal/content"
)

// Store is what the handlers need. The SQLite implementation (sqlite.go) is
// the production one; Memory serves the shipped seed and backs tests.
type Store interface {
	Current() *Snapshot
	Close() error
}

// Snapshot is the whole editable content set, already shaped for templates.
type Snapshot struct {
	// Gen changes on every rebuild, so response caches can key on it.
	Gen         uint64
	Memberships []content.Membership
	SingleVisit int
	Tags        []content.Tag
	Coaches     []content.Coach // every coach, published or not, in display order
	Reviews     []content.Review
	Classes     []content.Class
	Massage     []content.MassageService
	Hours       content.Hours
	// SlugHistory maps a former coach slug to the current one.
	SlugHistory map[string]string
	// LastMod is the content date per locale-independent path ("/coaches").
	LastMod map[string]string
}

// Published returns the coaches that have a public page, in display order.
func (s *Snapshot) Published() []content.Coach {
	out := make([]content.Coach, 0, len(s.Coaches))
	for _, c := range s.Coaches {
		if c.Published {
			out = append(out, c)
		}
	}
	return out
}

// Coach finds a coach by slug, published or not.
func (s *Snapshot) Coach(slug string) (content.Coach, bool) {
	for _, c := range s.Coaches {
		if c.Slug == slug {
			return c, true
		}
	}
	return content.Coach{}, false
}

// PublishedCoach is Coach restricted to published coaches; used for links, so
// an unpublished coach's name renders as plain text.
func (s *Snapshot) PublishedCoach(slug string) (content.Coach, bool) {
	c, ok := s.Coach(slug)
	return c, ok && c.Published
}

func (s *Snapshot) Tag(slug string) (content.Tag, bool) {
	for _, t := range s.Tags {
		if t.Slug == slug {
			return t, true
		}
	}
	return content.Tag{}, false
}

// TagsInUse are the tags at least one published coach carries. Only these get
// a filter chip, so no chip leads to an empty page.
func (s *Snapshot) TagsInUse() []content.Tag {
	var out []content.Tag
	for _, t := range s.Tags {
		for _, c := range s.Published() {
			if c.HasTag(t.Slug) {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// LanguagesInUse are the spoken languages at least one published coach has.
func (s *Snapshot) LanguagesInUse() []string {
	var out []string
	for _, l := range content.SpokenLanguages {
		for _, c := range s.Published() {
			if c.Speaks1(l) {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

// Filter returns published coaches matching an optional language and tag.
func (s *Snapshot) Filter(speaks, tag string) []content.Coach {
	var out []content.Coach
	for _, c := range s.Published() {
		if speaks != "" && !c.Speaks1(speaks) {
			continue
		}
		if tag != "" && !c.HasTag(tag) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// ReviewsFor returns the reviews linked to a coach, in stored order.
func (s *Snapshot) ReviewsFor(slug string) []content.Review {
	var out []content.Review
	for _, r := range s.Reviews {
		if slices.Contains(r.Coaches, slug) {
			out = append(out, r)
		}
	}
	return out
}

// HomeReviews returns the home-page selection with the page's own language
// first, then the rest, each group in the editor's order (BRIEF.md §4).
func (s *Snapshot) HomeReviews(lang string) []content.Review {
	var own, rest []content.Review
	for _, r := range s.Reviews {
		if !r.Home {
			continue
		}
		if r.Lang == lang {
			own = append(own, r)
		} else {
			rest = append(rest, r)
		}
	}
	bySort := func(a []content.Review) {
		sort.SliceStable(a, func(i, j int) bool { return a[i].Sort < a[j].Sort })
	}
	bySort(own)
	bySort(rest)
	return append(own, rest...)
}

// ClassesBy returns the classes a coach leads.
func (s *Snapshot) ClassesBy(slug string) []content.Class {
	var out []content.Class
	for _, c := range s.Classes {
		if c.Coach == slug {
			out = append(out, c)
		}
	}
	return out
}

// SimilarCoaches picks up to n other published coaches, most shared tags
// first, display order breaking ties.
func (s *Snapshot) SimilarCoaches(slug string, n int) []content.Coach {
	me, ok := s.Coach(slug)
	if !ok {
		return nil
	}
	type scored struct {
		c     content.Coach
		score int
	}
	var all []scored
	for _, c := range s.Published() {
		if c.Slug == slug {
			continue
		}
		sc := 0
		for _, t := range c.Tags {
			if me.HasTag(t) {
				sc++
			}
		}
		all = append(all, scored{c, sc})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })
	out := make([]content.Coach, 0, n)
	for i := 0; i < len(all) && i < n; i++ {
		out = append(out, all[i].c)
	}
	return out
}

// MinMassage is the cheapest massage, for the home teaser.
func (s *Snapshot) MinMassage() int {
	best := 0
	for _, m := range s.Massage {
		if m.Price > 0 && (best == 0 || m.Price < best) {
			best = m.Price
		}
	}
	return best
}

// MinPerMonth is the lowest per-month membership price.
func (s *Snapshot) MinPerMonth() int {
	best := 0
	for _, m := range s.Memberships {
		if p := m.Cheapest(); p > 0 && (best == 0 || p < best) {
			best = p
		}
	}
	return best
}

// SlotRef is one class occurrence in the weekly grid.
type SlotRef struct {
	Class content.Class
	Slot  content.Slot
}

// Week returns the slots grouped by ISO weekday (index 0 = Monday), each day
// sorted by start time.
func (s *Snapshot) Week() [7][]SlotRef {
	var w [7][]SlotRef
	for _, c := range s.Classes {
		for _, sl := range c.Slots {
			if sl.Day >= 1 && sl.Day <= 7 {
				w[sl.Day-1] = append(w[sl.Day-1], SlotRef{c, sl})
			}
		}
	}
	for d := range w {
		sort.SliceStable(w[d], func(i, j int) bool { return w[d][i].Slot.Start < w[d][j].Slot.Start })
	}
	return w
}

// NextSlot finds the next class starting after t (club time).
func (s *Snapshot) NextSlot(t time.Time) (SlotRef, int, bool) {
	week := s.Week()
	now := t.Format("15:04")
	today := content.ISODay(t)
	for i := range 8 {
		d := (today-1+i)%7 + 1
		for _, r := range week[d-1] {
			if i == 0 && r.Slot.Start <= now {
				continue
			}
			return r, i, true
		}
	}
	return SlotRef{}, 0, false
}

// Date is the content date for a path, falling back to the shipped baseline.
func (s *Snapshot) Date(path string) string {
	if d := s.LastMod[path]; d != "" {
		return d
	}
	return content.BaselineContentDate
}
