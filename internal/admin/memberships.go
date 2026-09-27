package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/store"
)

type membershipItem struct {
	store.MembershipRow
	Name, Meta, TermsText string
	First, Last           bool
}

type membershipsData struct {
	Items       []membershipItem
	SingleVisit int
	Deleted     bool
	DelCount    int
	Moved       string
}

func (s *Server) termLabel(p *Page, months int) string {
	switch months {
	case 1:
		return p.T("membership.term_1")
	case 3:
		return p.T("membership.term_3")
	case 6:
		return p.T("membership.term_6")
	default:
		return p.T("membership.term_12")
	}
}

func (s *Server) membershipsList(c *ctx) {
	p := s.newPage(c, "memberships", c.copy(s).T("nav.memberships"))
	p.Action = &Link{Href: "/admin/memberships/new", Label: p.T("action.add")}
	q := c.r.URL.Query()
	d := membershipsData{Deleted: q.Get("deleted") == "1", Moved: q.Get("moved"), SingleVisit: s.Store.Current().SingleVisit}
	rows, err := s.Store.MembershipRows()
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	var live []store.MembershipRow
	for _, r := range rows {
		if r.Deleted {
			d.DelCount++
		}
		if r.Deleted == d.Deleted {
			live = append(live, r)
		}
	}
	for i, r := range live {
		it := membershipItem{MembershipRow: r, Name: p.L(r.Name), First: i == 0, Last: i == len(live)-1}
		var parts []string
		for _, t := range r.Terms {
			parts = append(parts, s.termLabel(p, t.Months))
		}
		if r.GiftPT {
			parts = append(parts, "+1 PT")
		}
		it.TermsText = strings.Join(parts, " · ")
		if !r.Visible {
			it.Meta = p.T("meta.hidden")
		}
		d.Items = append(d.Items, it)
	}
	p.Data = d
	s.render(c, "memberships", p, http.StatusOK)
}

type termView struct {
	Months      int
	Label       string
	Price       int
	FreezeWeeks int
}

type membershipFormData struct {
	New     bool
	In      store.MembershipInput
	Terms   []termView
	Deleted bool
	Back    string
}

func (s *Server) membershipForm(c *ctx) {
	p := s.newPage(c, "memberships", "")
	p.Back = "/admin/memberships"
	d := membershipFormData{Back: "/admin/memberships"}
	if id := c.r.PathValue("id"); id != "" {
		r, err := s.Store.MembershipRow(id)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.In = store.MembershipInput{ID: r.ID, Slug: r.Slug, Sort: r.Sort, Visible: r.Visible, Name: r.Name, Desc: r.Desc, Terms: r.AllTerms, GiftPT: r.GiftPT}
		d.Deleted = r.Deleted
		p.Title = p.L(r.Name)
	} else {
		d.New = true
		d.In = store.MembershipInput{Name: content.L{}, Desc: content.L{}, Visible: true}
		for _, m := range []int{1, 3, 6, 12} {
			d.In.Terms = append(d.In.Terms, content.Term{Months: m})
		}
		p.Title = p.T("membership.title_new")
	}
	for _, t := range d.In.Terms {
		d.Terms = append(d.Terms, termView{Months: t.Months, Label: s.termLabel(p, t.Months), Price: t.Price, FreezeWeeks: t.FreezeWeeks})
	}
	p.Data = d
	s.render(c, "membership", p, http.StatusOK)
}

func (s *Server) membershipPost(c *ctx) {
	p := s.newPage(c, "memberships", "")
	p.Back = "/admin/memberships"
	r := c.r
	d := membershipFormData{New: r.PathValue("id") == "", Back: "/admin/memberships"}
	in := store.MembershipInput{ID: r.PathValue("id"), Slug: strings.TrimSpace(r.FormValue("slug")), Visible: r.FormValue("visible") == "1",
		Name: localeValues(r, "name", s.Cfg.Locales), Desc: localeValues(r, "desc", s.Cfg.Locales), GiftPT: r.FormValue("gift_pt") == "1"}
	for _, m := range []int{1, 3, 6, 12} {
		ms := strconv.Itoa(m)
		price, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("price_" + ms)))
		freeze, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("freeze_" + ms)))
		in.Terms = append(in.Terms, content.Term{Months: m, Price: price, FreezeWeeks: freeze})
		d.Terms = append(d.Terms, termView{Months: m, Label: s.termLabel(p, m), Price: price, FreezeWeeks: freeze})
	}
	if !d.New {
		row, err := s.Store.MembershipRow(in.ID)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		in.Sort, d.Deleted = row.Sort, row.Deleted
		p.Title = p.L(in.Name)
	} else {
		rows, _ := s.Store.MembershipRows()
		in.Sort = len(rows) + 1
		p.Title = p.T("membership.title_new")
	}
	if in.Slug == "" {
		in.Slug = Slugify(in.Name["en"])
	}
	d.In = in
	if strings.TrimSpace(in.Name["en"]) == "" {
		p.Errors = append(p.Errors, FieldError{"name_en", p.T("field.empty")})
	}
	if in.Visible {
		for _, lang := range s.Cfg.Locales {
			if strings.TrimSpace(in.Name[lang]) == "" {
				p.Errors = append(p.Errors, FieldError{"name_" + lang, p.F("field.locale_missing", p.T("membership.field_name"), p.T("locale."+lang))})
			}
		}
		sold := 0
		for _, t := range in.Terms {
			if t.Price > 0 {
				sold++
			}
		}
		if sold == 0 {
			p.Errors = append(p.Errors, FieldError{"price_1", p.T("membership.error_no_term")})
		}
	}
	if len(p.Errors) == 0 {
		if _, err := s.Store.SaveMembership(in, c.user.Name); err != nil {
			if errors.Is(err, store.ErrValidation) || strings.Contains(err.Error(), "UNIQUE") {
				p.Errors = append(p.Errors, FieldError{"name_en", p.T("coach.error_slug_taken")})
			} else {
				p.Errors = append(p.Errors, FieldError{"", err.Error()})
			}
		}
	}
	if len(p.Errors) > 0 {
		p.Data = d
		s.render(c, "membership", p, http.StatusUnprocessableEntity)
		return
	}
	s.flash(c, "ok", p.T("flash.saved"), "/"+s.siteLang(c)+"/memberships", p.T("action.open_site"))
	s.redirect(c, "/admin/memberships")
}

func (s *Server) membershipDelete(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.DeleteMembership(id, c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.deleted"), "/admin/memberships/"+id+"/restore", c.copy(s).T("action.restore"))
	s.redirect(c, "/admin/memberships")
}

func (s *Server) membershipRestore(c *ctx) {
	if err := s.Store.RestoreMembership(c.r.PathValue("id"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.restored"), "", "")
	s.redirect(c, "/admin/memberships")
}

func (s *Server) membershipMove(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.MoveMembership(id, c.r.FormValue("dir"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.redirect(c, safeBack(c.r.FormValue("back"), "/admin/memberships")+"?moved="+id)
}

type singleData struct{ Price int }

func (s *Server) singleForm(c *ctx) {
	p := s.newPage(c, "memberships", c.copy(s).T("membership.single"))
	p.Back = "/admin/memberships"
	p.Data = singleData{Price: s.Store.Current().SingleVisit}
	s.render(c, "single", p, http.StatusOK)
}

func (s *Server) singlePost(c *ctx) {
	p := s.newPage(c, "memberships", c.copy(s).T("membership.single"))
	p.Back = "/admin/memberships"
	price, _ := strconv.Atoi(strings.TrimSpace(c.r.FormValue("price")))
	if price <= 0 {
		p.Errors = append(p.Errors, FieldError{"price", p.T("membership.error_price")})
		p.Data = singleData{Price: price}
		s.render(c, "single", p, http.StatusUnprocessableEntity)
		return
	}
	if err := s.Store.SetSingleVisit(price, c.user.Name); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	s.flash(c, "ok", p.T("flash.saved"), "/"+s.siteLang(c)+"/memberships", p.T("action.open_site"))
	s.redirect(c, "/admin/memberships")
}

// ----------------------------------------------------------------- massage

type massageItem struct {
	store.MassageRow
	Name, Meta  string
	First, Last bool
}

type massageData struct {
	Items    []massageItem
	Deleted  bool
	DelCount int
	Moved    string
}

func (s *Server) massageList(c *ctx) {
	p := s.newPage(c, "massage", c.copy(s).T("nav.massage"))
	p.Action = &Link{Href: "/admin/massage/new", Label: p.T("action.add")}
	q := c.r.URL.Query()
	d := massageData{Deleted: q.Get("deleted") == "1", Moved: q.Get("moved")}
	rows, err := s.Store.MassageRows()
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	var live []store.MassageRow
	for _, r := range rows {
		if r.Deleted {
			d.DelCount++
		}
		if r.Deleted == d.Deleted {
			live = append(live, r)
		}
	}
	for i, r := range live {
		it := massageItem{MassageRow: r, Name: p.L(r.Name), First: i == 0, Last: i == len(live)-1}
		if !r.Visible {
			it.Meta = p.T("meta.hidden")
		}
		d.Items = append(d.Items, it)
	}
	p.Data = d
	s.render(c, "massage", p, http.StatusOK)
}

type massageFormData struct {
	New     bool
	In      store.MassageInput
	Deleted bool
}

func (s *Server) massageForm(c *ctx) {
	p := s.newPage(c, "massage", "")
	p.Back = "/admin/massage"
	d := massageFormData{}
	if id := c.r.PathValue("id"); id != "" {
		r, err := s.Store.MassageRow(id)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.In = store.MassageInput{ID: r.ID, Sort: r.Sort, Visible: r.Visible, Name: r.Name, Minutes: r.Minutes, Price: r.Price, PackCount: r.PackCount, PackPrice: r.PackPrice}
		d.Deleted = r.Deleted
		p.Title = p.L(r.Name)
	} else {
		d.New = true
		d.In = store.MassageInput{Name: content.L{}, Visible: true, Minutes: 60}
		p.Title = p.T("massage.title_new")
	}
	p.Data = d
	s.render(c, "massage_form", p, http.StatusOK)
}

func (s *Server) massagePost(c *ctx) {
	p := s.newPage(c, "massage", "")
	p.Back = "/admin/massage"
	r := c.r
	d := massageFormData{New: r.PathValue("id") == ""}
	atoi := func(k string) int { n, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(k))); return n }
	in := store.MassageInput{ID: r.PathValue("id"), Visible: r.FormValue("visible") == "1", Name: localeValues(r, "name", s.Cfg.Locales),
		Minutes: atoi("minutes"), Price: atoi("price"), PackCount: atoi("pack_count"), PackPrice: atoi("pack_price")}
	if !d.New {
		row, err := s.Store.MassageRow(in.ID)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		in.Sort, d.Deleted = row.Sort, row.Deleted
		p.Title = p.L(in.Name)
	} else {
		rows, _ := s.Store.MassageRows()
		in.Sort = len(rows) + 1
		p.Title = p.T("massage.title_new")
	}
	d.In = in
	if strings.TrimSpace(in.Name["en"]) == "" {
		p.Errors = append(p.Errors, FieldError{"name_en", p.T("field.empty")})
	}
	if in.Visible {
		for _, lang := range s.Cfg.Locales {
			if strings.TrimSpace(in.Name[lang]) == "" {
				p.Errors = append(p.Errors, FieldError{"name_" + lang, p.F("field.locale_missing", p.T("massage.field_name"), p.T("locale."+lang))})
			}
		}
	}
	if in.Minutes <= 0 {
		p.Errors = append(p.Errors, FieldError{"minutes", p.T("massage.error_minutes")})
	}
	if in.Price <= 0 {
		p.Errors = append(p.Errors, FieldError{"price", p.T("membership.error_price")})
	}
	if (in.PackCount > 0) != (in.PackPrice > 0) {
		p.Errors = append(p.Errors, FieldError{"pack_count", p.T("massage.error_pack")})
	}
	if len(p.Errors) == 0 {
		if _, err := s.Store.SaveMassage(in, c.user.Name); err != nil {
			p.Errors = append(p.Errors, FieldError{"", err.Error()})
		}
	}
	if len(p.Errors) > 0 {
		p.Data = d
		s.render(c, "massage_form", p, http.StatusUnprocessableEntity)
		return
	}
	s.flash(c, "ok", p.T("flash.saved"), "/"+s.siteLang(c)+"/massage", p.T("action.open_site"))
	s.redirect(c, "/admin/massage")
}

func (s *Server) massageDelete(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.DeleteMassage(id, c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.deleted"), "/admin/massage/"+id+"/restore", c.copy(s).T("action.restore"))
	s.redirect(c, "/admin/massage")
}

func (s *Server) massageRestore(c *ctx) {
	if err := s.Store.RestoreMassage(c.r.PathValue("id"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.restored"), "", "")
	s.redirect(c, "/admin/massage")
}

func (s *Server) massageMove(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.MoveMassage(id, c.r.FormValue("dir"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.redirect(c, safeBack(c.r.FormValue("back"), "/admin/massage")+"?moved="+id)
}
