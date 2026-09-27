package admin

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/store"
)

type hoursDay struct {
	Day         int
	Open, Close string
	Closed      bool
}

type hoursData struct {
	Week    []hoursDay
	Special []content.SpecialDay
	Past    []content.SpecialDay
	Today   string
}

func (s *Server) hoursForm(c *ctx) {
	snap := s.Store.Current()
	p := s.newPage(c, "hours", c.copy(s).T("nav.hours"))
	p.Action = &Link{Href: "/admin/hours/special/new", Label: p.T("action.add")}
	d := hoursData{Today: c.now.Format("2006-01-02")}
	for i, h := range snap.Hours.Week {
		d.Week = append(d.Week, hoursDay{Day: i + 1, Open: h.Open, Close: h.Close, Closed: h.Closed()})
	}
	d.Special, d.Past = splitSpecial(snap.Hours.Special, d.Today)
	p.Data = d
	s.render(c, "hours", p, http.StatusOK)
}

func splitSpecial(all []content.SpecialDay, today string) (upcoming, past []content.SpecialDay) {
	for _, sd := range all {
		if sd.Date >= today {
			upcoming = append(upcoming, sd)
		} else {
			past = append(past, sd)
		}
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].Date < upcoming[j].Date })
	sort.Slice(past, func(i, j int) bool { return past[i].Date > past[j].Date })
	return
}

func (s *Server) hoursPost(c *ctx) {
	p := s.newPage(c, "hours", c.copy(s).T("nav.hours"))
	var week [7]content.DayHours
	d := hoursData{Today: c.now.Format("2006-01-02")}
	for i := range 7 {
		n := i + 1
		hd := hoursDay{Day: n, Open: strings.TrimSpace(c.r.FormValue(fmt.Sprintf("open_%d", n))),
			Close: strings.TrimSpace(c.r.FormValue(fmt.Sprintf("close_%d", n))), Closed: c.r.FormValue(fmt.Sprintf("closed_%d", n)) == "1"}
		if hd.Closed {
			week[i] = content.DayHours{}
		} else {
			week[i] = content.DayHours{Open: hd.Open, Close: hd.Close}
			switch {
			case !store.ValidClock(hd.Open) || !store.ValidClock(hd.Close):
				p.Errors = append(p.Errors, FieldError{fmt.Sprintf("open_%d", n), p.F("hours.error_time", p.Day(n))})
			case hd.Open >= hd.Close:
				p.Errors = append(p.Errors, FieldError{fmt.Sprintf("close_%d", n), p.F("hours.error_order", p.Day(n))})
			}
		}
		d.Week = append(d.Week, hd)
	}
	snap := s.Store.Current()
	d.Special, d.Past = splitSpecial(snap.Hours.Special, d.Today)
	if len(p.Errors) == 0 {
		if err := s.Store.SetHours(week, c.user.Name); err != nil {
			p.Errors = append(p.Errors, FieldError{"", err.Error()})
		}
	}
	if len(p.Errors) > 0 {
		p.Data = d
		s.render(c, "hours", p, http.StatusUnprocessableEntity)
		return
	}
	s.flash(c, "ok", p.T("flash.saved"), s.siteVisitURL(c), p.T("action.open_site"))
	s.redirect(c, "/admin/hours")
}

// siteVisitURL points at the "getting here" block, where hours are shown.
func (s *Server) siteVisitURL(c *ctx) string { return "/" + s.siteLang(c) + "/#visit" }

type specialData struct {
	New    bool
	Day    content.SpecialDay
	Closed bool
	Back   string
}

func (s *Server) specialForm(c *ctx) {
	p := s.newPage(c, "hours", "")
	p.Back = "/admin/hours"
	d := specialData{New: true, Back: safeBack(c.r.URL.Query().Get("back"), "/admin/hours")}
	if date := c.r.PathValue("date"); date != "" {
		d.New = false
		found := false
		for _, sd := range s.Store.Current().Hours.Special {
			if sd.Date == date {
				d.Day, found = sd, true
			}
		}
		if !found {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.Closed = d.Day.Closed()
		p.Title = p.T("special.title_edit")
	} else {
		d.Closed = true
		p.Title = p.T("special.title_new")
	}
	p.Data = d
	s.render(c, "special", p, http.StatusOK)
}

func (s *Server) specialPost(c *ctx) {
	p := s.newPage(c, "hours", "")
	p.Back = "/admin/hours"
	d := specialData{New: c.r.PathValue("date") == "", Back: safeBack(c.r.FormValue("back"), "/admin/hours")}
	d.Day = content.SpecialDay{
		Date:  strings.TrimSpace(c.r.FormValue("date")),
		Open:  strings.TrimSpace(c.r.FormValue("open")),
		Close: strings.TrimSpace(c.r.FormValue("close")),
		Note:  localeValues(c.r, "note", s.Cfg.Locales),
	}
	d.Closed = c.r.FormValue("closed") == "1"
	if d.Closed {
		d.Day.Open, d.Day.Close = "", ""
	}
	if d.New {
		p.Title = p.T("special.title_new")
	} else {
		p.Title = p.T("special.title_edit")
		d.Day.Date = c.r.PathValue("date")
	}
	if d.Day.Date == "" {
		p.Errors = append(p.Errors, FieldError{"date", p.T("field.empty")})
	}
	if !d.Closed {
		switch {
		case !store.ValidClock(d.Day.Open) || !store.ValidClock(d.Day.Close):
			p.Errors = append(p.Errors, FieldError{"open", p.T("special.error_time")})
		case d.Day.Open >= d.Day.Close:
			p.Errors = append(p.Errors, FieldError{"close", p.T("special.error_order")})
		}
	}
	if len(p.Errors) == 0 {
		if err := s.Store.SaveSpecialDay(d.Day, c.user.Name); err != nil {
			p.Errors = append(p.Errors, FieldError{"date", p.T("special.error_date")})
		}
	}
	if len(p.Errors) > 0 {
		p.Data = d
		s.render(c, "special", p, http.StatusUnprocessableEntity)
		return
	}
	s.flash(c, "ok", p.T("flash.saved"), s.siteVisitURL(c), p.T("action.open_site"))
	s.redirect(c, d.Back)
}

func (s *Server) specialDelete(c *ctx) {
	if err := s.Store.DeleteSpecialDay(c.r.PathValue("date"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.deleted"), "", "")
	s.redirect(c, "/admin/hours")
}

// localeValues reads name_en, name_ru, name_ka from a form into an L.
func localeValues(r *http.Request, name string, locales []string) content.L {
	l := content.L{}
	for _, lang := range locales {
		l[lang] = strings.TrimSpace(r.FormValue(name + "_" + lang))
	}
	return l
}
