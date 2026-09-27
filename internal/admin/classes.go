package admin

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/store"
)

type classListItem struct {
	store.ClassRow
	Name, Meta, CoachName, SlotsText string
	First, Last                      bool
}

type classesData struct {
	Items    []classListItem
	Deleted  bool
	DelCount int
	Moved    string
}

func (s *Server) coachName(p *Page, id string) (string, bool) {
	if id == "" {
		return "", false
	}
	r, err := s.Store.CoachRow(id)
	if err != nil {
		return "", false
	}
	return p.L(r.Name), r.Published && !r.Deleted
}

func slotsText(p *Page, slots []content.Slot) string {
	var parts []string
	for _, sl := range slots {
		parts = append(parts, p.DayShort(sl.Day)+" "+sl.Start)
	}
	return strings.Join(parts, " · ")
}

func (s *Server) classesList(c *ctx) {
	p := s.newPage(c, "classes", c.copy(s).T("nav.classes"))
	p.Action = &Link{Href: "/admin/classes/new", Label: p.T("action.add")}
	q := c.r.URL.Query()
	d := classesData{Deleted: q.Get("deleted") == "1", Moved: q.Get("moved")}
	rows, err := s.Store.ClassRows()
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	var live []store.ClassRow
	for _, r := range rows {
		if r.Deleted {
			d.DelCount++
		}
		if r.Deleted == d.Deleted {
			live = append(live, r)
		}
	}
	for i, r := range live {
		it := classListItem{ClassRow: r, Name: p.L(r.Name), First: i == 0, Last: i == len(live)-1, SlotsText: slotsText(p, r.Slots)}
		if name, pub := s.coachName(p, r.CoachID); name != "" {
			it.CoachName = name
			if !pub {
				it.Meta = p.T("class.coach_unpublished_short")
			}
		}
		if !r.Visible {
			it.Meta = strings.TrimPrefix(it.Meta+" · "+p.T("meta.hidden"), " · ")
		}
		d.Items = append(d.Items, it)
	}
	p.Data = d
	s.render(c, "classes", p, http.StatusOK)
}

type slotView struct {
	N          int
	Day        int
	DayName    string
	Start, End string
	Minutes    int
}

type classFormData struct {
	New       bool
	In        store.ClassInput
	Deleted   bool
	Coaches   []coachOption
	Langs     []langOption
	Slots     []slotView
	Back      string
	SiteLangs []string
}

func (s *Server) classForm(c *ctx) {
	p := s.newPage(c, "classes", "")
	p.Back = "/admin/classes"
	d := classFormData{Back: safeBack(c.r.URL.Query().Get("back"), "/admin/classes")}
	if id := c.r.PathValue("id"); id != "" {
		r, err := s.Store.ClassRow(id)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.In = store.ClassInput{ID: r.ID, Slug: r.Slug, Sort: r.Sort, Visible: r.Visible, CoachID: r.CoachID, Name: r.Name, Desc: r.Desc,
			Price: r.Price, PriceNote: r.PriceNote, Langs: r.Langs, Slots: r.Slots}
		d.Deleted = r.Deleted
		p.Title = p.L(r.Name)
		for i, sl := range r.Slots {
			d.Slots = append(d.Slots, slotView{N: i, Day: sl.Day, DayName: p.Day(sl.Day), Start: sl.Start, End: sl.End(), Minutes: sl.Minutes})
		}
	} else {
		d.New = true
		d.In = store.ClassInput{Name: content.L{}, Desc: content.L{}, PriceNote: content.L{}, Visible: true, Langs: []string{"ka", "ru", "en"}}
		p.Title = p.T("class.title_new")
	}
	d.Coaches = s.coachOptions(p, []string{d.In.CoachID})
	d.Langs = s.classLangOptions(p, d.In.Langs)
	p.Data = d
	s.render(c, "class", p, http.StatusOK)
}

func (s *Server) classLangOptions(p *Page, selected []string) []langOption {
	var out []langOption
	for _, l := range s.Cfg.Locales {
		sel := false
		for _, sl := range selected {
			if sl == l {
				sel = true
			}
		}
		out = append(out, langOption{Code: l, Name: p.T("speaks." + l), Selected: sel})
	}
	return out
}

func (s *Server) classPost(c *ctx) {
	p := s.newPage(c, "classes", "")
	p.Back = "/admin/classes"
	r := c.r
	d := classFormData{New: r.PathValue("id") == "", Back: safeBack(r.FormValue("back"), "/admin/classes")}
	price, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("price")))
	in := store.ClassInput{
		ID: r.PathValue("id"), Slug: strings.TrimSpace(r.FormValue("slug")), Visible: r.FormValue("visible") == "1",
		CoachID: r.FormValue("coach"), Name: localeValues(r, "name", s.Cfg.Locales), Desc: localeValues(r, "desc", s.Cfg.Locales),
		Price: price, PriceNote: localeValues(r, "price_note", s.Cfg.Locales), Langs: r.Form["lang"],
	}
	if !d.New {
		row, err := s.Store.ClassRow(in.ID)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		in.Sort, in.Slots = row.Sort, row.Slots // slots are edited through their own form
		d.Deleted = row.Deleted
		p.Title = p.L(in.Name)
	} else {
		rows, _ := s.Store.ClassRows()
		in.Sort = len(rows) + 1
		p.Title = p.T("class.title_new")
	}
	if in.Slug == "" {
		in.Slug = Slugify(in.Name["en"])
	}
	d.In = in
	if strings.TrimSpace(in.Name["en"]) == "" {
		p.Errors = append(p.Errors, FieldError{"name_en", p.T("field.empty")})
	}
	if in.Slug == "" {
		p.Errors = append(p.Errors, FieldError{"slug", p.T("coach.error_slug")})
	}
	if in.Visible {
		for _, lang := range s.Cfg.Locales {
			if strings.TrimSpace(in.Name[lang]) == "" {
				p.Errors = append(p.Errors, FieldError{"name_" + lang, p.F("field.locale_missing", p.T("class.field_name"), p.T("locale."+lang))})
			}
		}
	}
	if len(p.Errors) == 0 {
		if _, err := s.Store.SaveClass(in, c.user.Name); err != nil {
			if errors.Is(err, store.ErrValidation) || strings.Contains(err.Error(), "UNIQUE") {
				p.Errors = append(p.Errors, FieldError{"slug", p.T("coach.error_slug_taken")})
			} else {
				p.Errors = append(p.Errors, FieldError{"", err.Error()})
			}
		}
	}
	if len(p.Errors) > 0 {
		d.Coaches = s.coachOptions(p, []string{in.CoachID})
		d.Langs = s.classLangOptions(p, in.Langs)
		for i, sl := range in.Slots {
			d.Slots = append(d.Slots, slotView{N: i, Day: sl.Day, DayName: p.Day(sl.Day), Start: sl.Start, End: sl.End(), Minutes: sl.Minutes})
		}
		p.Data = d
		s.render(c, "class", p, http.StatusUnprocessableEntity)
		return
	}
	if d.New {
		rows, _ := s.Store.ClassRows()
		for _, row := range rows {
			if row.Slug == in.Slug {
				s.flash(c, "ok", p.T("flash.saved"), "", "")
				s.redirect(c, "/admin/classes/"+row.ID)
				return
			}
		}
	}
	s.flash(c, "ok", p.T("flash.saved"), "/"+s.siteLang(c)+"/classes", p.T("action.open_site"))
	s.redirect(c, d.Back)
}

func (s *Server) classDelete(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.DeleteClass(id, c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.deleted"), "/admin/classes/"+id+"/restore", c.copy(s).T("action.restore"))
	s.redirect(c, "/admin/classes")
}

func (s *Server) classRestore(c *ctx) {
	if err := s.Store.RestoreClass(c.r.PathValue("id"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.restored"), "", "")
	s.redirect(c, "/admin/classes/"+c.r.PathValue("id"))
}

func (s *Server) classMove(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.MoveClass(id, c.r.FormValue("dir"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.redirect(c, safeBack(c.r.FormValue("back"), "/admin/classes")+"?moved="+id)
}

// ------------------------------------------------------------------ slots

type slotFormData struct {
	New       bool
	ClassID   string
	ClassName string
	Classes   []classOption // when the class is chosen on this screen (from the grid)
	N         int
	Slot      content.Slot
	Back      string
	Durations []int
	Conflicts []string
}

type classOption struct {
	ID, Name string
	Selected bool
}

var durations = []int{30, 45, 60, 75, 90, 120}

func (s *Server) classOptions(p *Page, selected string) []classOption {
	rows, _ := s.Store.ClassRows()
	var out []classOption
	for _, r := range rows {
		if r.Deleted {
			continue
		}
		out = append(out, classOption{ID: r.ID, Name: p.L(r.Name), Selected: r.ID == selected})
	}
	return out
}

// slotForm serves /admin/classes/{id}/slots/{n|new} and /admin/schedule/new.
func (s *Server) slotForm(c *ctx) {
	p := s.newPage(c, "schedule", c.copy(s).T("slot.title"))
	q := c.r.URL.Query()
	d := slotFormData{Back: safeBack(q.Get("back"), "/admin/schedule"), Durations: durations}
	p.Back = d.Back
	d.ClassID = c.r.PathValue("id")
	d.New = c.r.PathValue("n") == "" || c.r.PathValue("n") == "new"
	if d.ClassID == "" {
		d.Classes = s.classOptions(p, q.Get("class"))
		d.ClassID = q.Get("class")
	} else {
		row, err := s.Store.ClassRow(d.ClassID)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.ClassName = p.L(row.Name)
		if !d.New {
			d.N, _ = strconv.Atoi(c.r.PathValue("n"))
			if d.N < 0 || d.N >= len(row.Slots) {
				s.errorPage(c, http.StatusNotFound)
				return
			}
			d.Slot = row.Slots[d.N]
		}
	}
	if d.New {
		d.Slot = content.Slot{Day: p.Today(), Start: "19:00", Minutes: 60}
		if day, _ := strconv.Atoi(q.Get("day")); day >= 1 && day <= 7 {
			d.Slot.Day = day
		}
		if st := q.Get("start"); store.ValidClock(st) {
			d.Slot.Start = st
		}
	}
	p.Data = d
	s.render(c, "slot", p, http.StatusOK)
}

func (s *Server) slotPost(c *ctx) {
	p := s.newPage(c, "schedule", c.copy(s).T("slot.title"))
	r := c.r
	d := slotFormData{Back: safeBack(r.FormValue("back"), "/admin/schedule"), Durations: durations}
	p.Back = d.Back
	d.ClassID = r.PathValue("id")
	if d.ClassID == "" {
		d.ClassID = r.FormValue("class")
		d.Classes = s.classOptions(p, d.ClassID)
	}
	d.New = r.PathValue("n") == "" || r.PathValue("n") == "new"
	day, _ := strconv.Atoi(r.FormValue("day"))
	minutes, _ := strconv.Atoi(r.FormValue("minutes"))
	if custom, err := strconv.Atoi(strings.TrimSpace(r.FormValue("minutes_custom"))); err == nil && custom > 0 {
		minutes = custom
	}
	d.Slot = content.Slot{Day: day, Start: strings.TrimSpace(r.FormValue("start")), Minutes: minutes}
	row, err := s.Store.ClassRow(d.ClassID)
	if err != nil {
		if d.ClassID == "" && len(d.Classes) > 0 {
			p.Errors = append(p.Errors, FieldError{"class", p.T("slot.error_class")})
		} else {
			s.errorPage(c, http.StatusNotFound)
			return
		}
	} else {
		d.ClassName = p.L(row.Name)
	}
	if day < 1 || day > 7 {
		p.Errors = append(p.Errors, FieldError{"day", p.T("slot.error_day")})
	}
	if !store.ValidClock(d.Slot.Start) {
		p.Errors = append(p.Errors, FieldError{"start", p.T("slot.error_start")})
	}
	if minutes < 15 || minutes > 240 {
		p.Errors = append(p.Errors, FieldError{"minutes", p.T("slot.error_minutes")})
	}
	if len(p.Errors) > 0 {
		p.Data = d
		s.render(c, "slot", p, http.StatusUnprocessableEntity)
		return
	}
	slots := append([]content.Slot(nil), row.Slots...)
	if d.New {
		slots = append(slots, d.Slot)
	} else {
		d.N, _ = strconv.Atoi(r.PathValue("n"))
		if d.N < 0 || d.N >= len(slots) {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		slots[d.N] = d.Slot
	}
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].Day != slots[j].Day {
			return slots[i].Day < slots[j].Day
		}
		return slots[i].Start < slots[j].Start
	})
	in := classInputFromRow(row)
	in.Slots = slots
	if _, err := s.Store.SaveClass(in, c.user.Name); err != nil {
		p.Errors = append(p.Errors, FieldError{"", err.Error()})
		p.Data = d
		s.render(c, "slot", p, http.StatusUnprocessableEntity)
		return
	}
	s.flash(c, "ok", p.T("flash.saved"), "/"+s.siteLang(c)+"/classes", p.T("action.open_site"))
	back := d.Back
	if strings.HasPrefix(back, "/admin/schedule") {
		sep := "?"
		if strings.Contains(back, "?") {
			sep = "&"
		}
		back += fmt.Sprintf("%ssaved=%s-%d-%s", sep, row.ID, d.Slot.Day, strings.ReplaceAll(d.Slot.Start, ":", ""))
		if !strings.Contains(back, "day=") {
			back += fmt.Sprintf("&day=%d", d.Slot.Day)
		}
	}
	s.redirect(c, back)
}

func (s *Server) slotDelete(c *ctx) {
	row, err := s.Store.ClassRow(c.r.PathValue("id"))
	if err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	n, _ := strconv.Atoi(c.r.PathValue("n"))
	if n < 0 || n >= len(row.Slots) {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	in := classInputFromRow(row)
	in.Slots = append(append([]content.Slot(nil), row.Slots[:n]...), row.Slots[n+1:]...)
	if _, err := s.Store.SaveClass(in, c.user.Name); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.deleted"), "", "")
	s.redirect(c, safeBack(c.r.FormValue("back"), "/admin/classes/"+row.ID))
}

func classInputFromRow(r store.ClassRow) store.ClassInput {
	return store.ClassInput{ID: r.ID, Slug: r.Slug, Sort: r.Sort, Visible: r.Visible, CoachID: r.CoachID, Name: r.Name, Desc: r.Desc,
		Price: r.Price, PriceNote: r.PriceNote, Langs: r.Langs, Slots: r.Slots}
}
