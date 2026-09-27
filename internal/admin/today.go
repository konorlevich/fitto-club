package admin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/store"
)

// Attention is one item of "needs attention" on Today (IA: Content
// Hierarchy). Href is where it gets fixed; empty for the club's pending facts.
type Attention struct {
	Text, Href, Note string
}

type todayData struct {
	StatusText string
	Open       bool
	HoursText  string
	Attention  []Attention
	Pending    []string // facts awaiting the club, grouped into one item
	Classes    []todayClass
	Recent     []historyLine
}

type todayClass struct {
	Start, End, Name, Coach, Href string
}

type historyLine struct {
	When, Text, Href string
}

func (s *Server) today(c *ctx) {
	if c.r.URL.Path != "/admin" && c.r.URL.Path != "/admin/" {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	snap := s.Store.Current()
	p := s.newPage(c, "today", c.copy(s).T("today.title"))
	d := todayData{}

	st := snap.Hours.StatusAt(c.now)
	d.Open = st.Open
	switch {
	case st.Open:
		d.StatusText = p.F("today.open_until", st.Until)
	case st.NextDay == 0:
		d.StatusText = p.F("today.closed_opens_at", st.NextOpen)
	case st.NextDay == -1:
		d.StatusText = p.F("today.closed_opens_day", p.T("today.tomorrow"), st.NextOpen)
	case st.NextDay > 0:
		d.StatusText = p.F("today.closed_opens_day", p.T(fmt.Sprintf("day_on.%d", st.NextDay)), st.NextOpen)
	default:
		d.StatusText = p.T("today.closed_today")
	}
	today, special := snap.Hours.On(c.now)
	d.HoursText = p.Day(p.Today())
	if today.Closed() {
		d.HoursText += " · " + p.T("today.closed_today")
	} else {
		d.HoursText += " · " + today.Open + "–" + today.Close
	}
	if special != nil && p.L(special.Note) != "" {
		d.HoursText += " · " + p.L(special.Note)
	}

	d.Attention = s.attention(snap, p)
	d.Pending = content.Pending
	for _, r := range snap.Week()[p.Today()-1] {
		tc := todayClass{Start: r.Slot.Start, End: r.Slot.End(), Name: p.L(r.Class.Name), Href: "/admin/classes/k-" + r.Class.Slug + "?back=/admin"}
		if c, ok := snap.Coach(r.Class.Coach); ok {
			tc.Coach = p.L(c.Name)
		}
		d.Classes = append(d.Classes, tc)
	}

	if changes, err := s.Store.Changes("", 5, 0); err == nil {
		for _, ch := range changes {
			d.Recent = append(d.Recent, s.historyLine(p, ch))
		}
	}
	p.Data = d
	s.render(c, "today", p, http.StatusOK)
}

// attention builds the list in the IA's order: visible things with a missing
// translation, classes led by an unpublished coach, published coaches without
// a photo, drafts, and finally facts the club has not confirmed.
func (s *Server) attention(snap *store.Snapshot, p *Page) []Attention {
	var out []Attention
	missing := func(l content.L) []string {
		var m []string
		for _, lang := range s.Cfg.Locales {
			if strings.TrimSpace(l[lang]) == "" {
				m = append(m, p.T("locale."+lang))
			}
		}
		return m
	}
	rows, _ := s.Store.CoachRows()
	coachByID := map[string]store.CoachRow{}
	for _, r := range rows {
		coachByID[r.ID] = r
		if r.Deleted || !r.Published {
			continue
		}
		for _, l := range [][]string{missing(r.Name), missing(r.Bio)} {
			if len(l) > 0 {
				out = append(out, Attention{Text: p.F("attention.missing_locale", p.L(r.Name), strings.Join(l, ", ")), Href: "/admin/coaches/" + r.ID})
				break
			}
		}
	}
	for _, m := range snap.Memberships {
		if l := append(missing(m.Name), missing(m.Desc)...); len(l) > 0 {
			out = append(out, Attention{Text: p.F("attention.missing_locale", p.L(m.Name), strings.Join(dedupe(l), ", ")), Href: "/admin/memberships/m-" + m.Slug})
		}
	}
	for _, k := range snap.Classes {
		if l := append(missing(k.Name), missing(k.Desc)...); len(l) > 0 {
			out = append(out, Attention{Text: p.F("attention.missing_locale", p.L(k.Name), strings.Join(dedupe(l), ", ")), Href: "/admin/classes/k-" + k.Slug})
		}
	}
	for _, t := range snap.Tags {
		if l := missing(t.Name); len(l) > 0 {
			out = append(out, Attention{Text: p.F("attention.missing_locale", p.L(t.Name), strings.Join(l, ", ")), Href: "/admin/tags/" + t.Slug})
		}
	}
	for _, k := range snap.Classes {
		if k.Coach != "" {
			if _, ok := snap.PublishedCoach(k.Coach); !ok {
				out = append(out, Attention{Text: p.F("attention.coach_unpublished", p.L(k.Name)), Href: "/admin/classes/k-" + k.Slug})
			}
		}
	}
	for _, r := range rows {
		if !r.Deleted && r.Published && !r.Photo {
			out = append(out, Attention{Text: p.F("attention.no_photo", p.L(r.Name)), Href: "/admin/coaches/" + r.ID})
		}
	}
	for _, r := range rows {
		if !r.Deleted && !r.Published {
			out = append(out, Attention{Text: p.F("attention.draft", p.L(r.Name)), Href: "/admin/coaches/" + r.ID})
		}
	}
	return out
}

func dedupe(in []string) []string {
	var out []string
	for _, s := range in {
		found := false
		for _, o := range out {
			if o == s {
				found = true
				break
			}
		}
		if !found {
			out = append(out, s)
		}
	}
	return out
}

// historyLine turns a changelog row into "Otar changed coach “Gocha”".
func (s *Server) historyLine(p *Page, ch store.Change) historyLine {
	entity := p.T("entity." + ch.Entity)
	actor := ch.Actor
	if actor == "" {
		actor = "—"
	}
	key := "history." + ch.Action
	if _, ok := p.C[key]; !ok {
		key = "history.save"
	}
	href := ""
	switch ch.Entity {
	case "coach":
		href = "/admin/coaches/" + ch.EntityID
	case "class":
		href = "/admin/classes/" + ch.EntityID
	case "membership":
		href = "/admin/memberships/" + ch.EntityID
	case "massage":
		href = "/admin/massage/" + ch.EntityID
	case "review":
		href = "/admin/reviews/" + ch.EntityID
	case "tag":
		href = "/admin/tags/" + ch.EntityID
	case "hours", "special":
		href = "/admin/hours"
	case "user":
		if p.User != nil && p.User.IsOwner() {
			href = "/admin/users/" + ch.EntityID
		}
	}
	return historyLine{When: p.When(ch.At), Text: p.F(key, actor, entity, ch.Summary), Href: href}
}
