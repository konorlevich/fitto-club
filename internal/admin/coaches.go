package admin

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/store"
)

type coachListItem struct {
	store.CoachRow
	Name, Meta string
	Speaks     string
	First      bool
	Last       bool
}

type coachesData struct {
	Items    []coachListItem
	Deleted  bool
	DelCount int
	Moved    string
}

func (s *Server) coachesList(c *ctx) {
	p := s.newPage(c, "coaches", c.copy(s).T("nav.coaches"))
	p.Action = &Link{Href: "/admin/coaches/new", Label: p.T("action.add")}
	q := c.r.URL.Query()
	d := coachesData{Deleted: q.Get("deleted") == "1", Moved: q.Get("moved")}
	rows, err := s.Store.CoachRows()
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	var live []store.CoachRow
	for _, r := range rows {
		if r.Deleted {
			d.DelCount++
		}
		if r.Deleted == d.Deleted {
			live = append(live, r)
		}
	}
	for i, r := range live {
		it := coachListItem{CoachRow: r, Name: p.L(r.Name), First: i == 0, Last: i == len(live)-1}
		var meta []string
		if !r.Published {
			meta = append(meta, p.T("meta.draft"))
		}
		if !r.Photo {
			meta = append(meta, p.T("coach.no_photo_short"))
		}
		it.Meta = strings.Join(meta, " · ")
		var sp []string
		for _, l := range r.Speaks {
			sp = append(sp, strings.ToUpper(l))
		}
		it.Speaks = strings.Join(sp, " · ")
		d.Items = append(d.Items, it)
	}
	p.Data = d
	s.render(c, "coaches", p, http.StatusOK)
}

type coachFormData struct {
	New       bool
	In        store.CoachInput
	Row       store.CoachRow
	Deleted   bool
	Tags      []tagOption
	Langs     []langOption
	Back      string
	PhotoURL  string
	Published bool
}

type tagOption struct {
	Slug, Name string
	Selected   bool
}

type langOption struct {
	Code, Name string
	Selected   bool
}

func (s *Server) tagOptions(p *Page, selected []string) []tagOption {
	var out []tagOption
	for _, t := range s.Store.Current().Tags {
		sel := false
		for _, sl := range selected {
			if sl == t.Slug {
				sel = true
			}
		}
		out = append(out, tagOption{Slug: t.Slug, Name: p.L(t.Name), Selected: sel})
	}
	return out
}

func (s *Server) langOptions(p *Page, selected []string) []langOption {
	var out []langOption
	for _, l := range content.SpokenLanguages {
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

func (s *Server) coachForm(c *ctx) {
	p := s.newPage(c, "coaches", "")
	p.Back = "/admin/coaches"
	d := coachFormData{Back: safeBack(c.r.URL.Query().Get("back"), "/admin/coaches")}
	if id := c.r.PathValue("id"); id != "" {
		r, err := s.Store.CoachRow(id)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.Row, d.Deleted = r, r.Deleted
		d.In = store.CoachInput{ID: r.ID, Slug: r.Slug, Sort: r.Sort, Published: r.Published, Photo: r.Photo, Instagram: r.Instagram,
			Name: r.Name, Bio: r.Bio, Tags: r.Tags, Speaks: r.Speaks, PTPrice: r.PTPrice, PTCurrency: r.Currency(), PTFrom: r.PTFrom}
		d.PhotoURL = s.coachPhotoURL(r, 400)
		p.Title = p.L(r.Name)
		if !r.Published {
			p.Action = nil
		}
	} else {
		d.New = true
		d.In = store.CoachInput{Name: content.L{}, Bio: content.L{}, PTCurrency: "GEL", Speaks: []string{"ka"}}
		p.Title = p.T("coach.title_new")
	}
	d.Tags = s.tagOptions(p, d.In.Tags)
	d.Langs = s.langOptions(p, d.In.Speaks)
	p.Data = d
	s.render(c, "coach", p, http.StatusOK)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify makes a URL slug from a Latin name: lowercase ASCII, hyphens.
func Slugify(name string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		if unicode.Is(unicode.Mn, r) {
			continue // strip accents
		}
		if r < 128 {
			b.WriteRune(r)
		}
	}
	return strings.Trim(nonSlug.ReplaceAllString(b.String(), "-"), "-")
}

func (s *Server) coachPost(c *ctx) {
	p := s.newPage(c, "coaches", "")
	p.Back = "/admin/coaches"
	r := c.r
	d := coachFormData{New: r.PathValue("id") == "", Back: safeBack(r.FormValue("back"), "/admin/coaches")}
	price, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("pt_price")))
	in := store.CoachInput{
		ID: r.PathValue("id"), Slug: strings.TrimSpace(r.FormValue("slug")),
		Published: r.FormValue("published") == "1" && r.FormValue("draft") == "",
		Instagram: strings.TrimPrefix(strings.TrimSpace(r.FormValue("instagram")), "@"),
		Name:      localeValues(r, "name", s.Cfg.Locales), Bio: localeValues(r, "bio", s.Cfg.Locales),
		Tags: r.Form["tag"], Speaks: r.Form["speaks"],
		PTPrice: price, PTCurrency: r.FormValue("pt_currency"), PTFrom: r.FormValue("pt_from") == "1",
	}
	if !d.New {
		row, err := s.Store.CoachRow(in.ID)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.Row = row
		in.Sort, in.Photo = row.Sort, row.Photo
		d.PhotoURL = s.coachPhotoURL(row, 400)
		p.Title = p.L(in.Name)
	} else {
		p.Title = p.T("coach.title_new")
		rows, _ := s.Store.CoachRows()
		in.Sort = len(rows) + 1
	}
	if in.Slug == "" {
		in.Slug = Slugify(in.Name["en"])
	}
	d.In, d.Published = in, r.FormValue("published") == "1"

	if strings.TrimSpace(in.Name["en"]) == "" {
		p.Errors = append(p.Errors, FieldError{"name_en", p.T("field.empty")})
	}
	if in.Slug == "" {
		p.Errors = append(p.Errors, FieldError{"slug", p.T("coach.error_slug")})
	}
	if in.Published {
		for _, lang := range s.Cfg.Locales {
			for _, f := range []struct {
				name string
				l    content.L
			}{{"name", in.Name}, {"bio", in.Bio}} {
				if strings.TrimSpace(f.l[lang]) == "" {
					p.Errors = append(p.Errors, FieldError{f.name + "_" + lang, p.F("field.locale_missing", p.T("coach.field_"+f.name), p.T("locale."+lang))})
				}
			}
		}
	}
	if len(p.Errors) == 0 {
		if err := s.Store.SaveCoach(in, c.user.Name); err != nil {
			switch {
			case errors.Is(err, store.ErrValidation) && strings.Contains(err.Error(), "slug"):
				p.Errors = append(p.Errors, FieldError{"slug", p.T("coach.error_slug")})
			case strings.Contains(err.Error(), "UNIQUE"):
				p.Errors = append(p.Errors, FieldError{"slug", p.T("coach.error_slug_taken")})
			default:
				p.Errors = append(p.Errors, FieldError{"", err.Error()})
			}
		}
	}
	if len(p.Errors) > 0 {
		d.Tags = s.tagOptions(p, in.Tags)
		d.Langs = s.langOptions(p, in.Speaks)
		p.Data = d
		s.render(c, "coach", p, http.StatusUnprocessableEntity)
		return
	}
	// A new coach lands on its own page so the photo block appears.
	if d.New {
		rows, _ := s.Store.CoachRows()
		for _, row := range rows {
			if row.Slug == in.Slug {
				s.flash(c, "ok", p.T("flash.saved"), "", "")
				s.redirect(c, "/admin/coaches/"+row.ID)
				return
			}
		}
	}
	link := ""
	if in.Published {
		link = "/" + s.siteLang(c) + "/coaches/" + in.Slug
	}
	s.flash(c, "ok", p.T("flash.saved"), link, p.T("action.open_site"))
	s.redirect(c, d.Back)
}

func (s *Server) coachDelete(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.DeleteCoach(id, c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.deleted"), "/admin/coaches/"+id+"/restore", c.copy(s).T("action.restore"))
	s.redirect(c, "/admin/coaches")
}

func (s *Server) coachRestore(c *ctx) {
	if err := s.Store.RestoreCoach(c.r.PathValue("id"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.restored"), "", "")
	s.redirect(c, "/admin/coaches/"+c.r.PathValue("id"))
}

func (s *Server) coachMove(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.MoveCoach(id, c.r.FormValue("dir"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.redirect(c, safeBack(c.r.FormValue("back"), "/admin/coaches")+"?moved="+id)
}

// coachPhotoURL is the card image: an uploaded file or the shipped one.
func (s *Server) coachPhotoURL(r store.CoachRow, w int) string {
	if !r.Photo {
		return ""
	}
	if r.PhotoFile != "" {
		return "/uploads/" + r.PhotoFile + "-" + strconv.Itoa(w) + ".webp"
	}
	return "/static/v/" + s.Assets["img/coaches/"+r.Slug+"-"+strconv.Itoa(w)+".webp"] + "/img/coaches/" + r.Slug + "-" + strconv.Itoa(w) + ".webp"
}

// ------------------------------------------------------------------- tags

type tagRow struct {
	content.Tag
	Name  string
	Used  int
	First bool
	Last  bool
}

type tagsData struct {
	Items []tagRow
	Moved string
}

func (s *Server) tagsList(c *ctx) {
	p := s.newPage(c, "coaches", c.copy(s).T("nav.tags"))
	p.Back = "/admin/coaches"
	snap := s.Store.Current()
	d := tagsData{Moved: c.r.URL.Query().Get("moved")}
	for i, t := range snap.Tags {
		used := 0
		for _, co := range snap.Coaches {
			if co.HasTag(t.Slug) {
				used++
			}
		}
		d.Items = append(d.Items, tagRow{Tag: t, Name: p.L(t.Name), Used: used, First: i == 0, Last: i == len(snap.Tags)-1})
	}
	p.Data = d
	s.render(c, "tags", p, http.StatusOK)
}

type tagFormData struct {
	New  bool
	In   store.TagInput
	Used int
}

func (s *Server) tagForm(c *ctx) {
	p := s.newPage(c, "coaches", "")
	p.Back = "/admin/tags"
	d := tagFormData{New: true, In: store.TagInput{Name: content.L{}}}
	if slug := c.r.PathValue("slug"); slug != "" {
		t, ok := s.Store.Current().Tag(slug)
		if !ok {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.New, d.In = false, store.TagInput{Slug: t.Slug, Name: t.Name, Sort: t.Sort}
		for _, co := range s.Store.Current().Coaches {
			if co.HasTag(slug) {
				d.Used++
			}
		}
		p.Title = p.L(t.Name)
	} else {
		p.Title = p.T("tag.title_new")
	}
	p.Data = d
	s.render(c, "tag", p, http.StatusOK)
}

func (s *Server) tagPost(c *ctx) {
	p := s.newPage(c, "coaches", "")
	p.Back = "/admin/tags"
	d := tagFormData{New: c.r.PathValue("slug") == ""}
	d.In = store.TagInput{Slug: c.r.PathValue("slug"), Name: localeValues(c.r, "name", s.Cfg.Locales)}
	if d.New {
		d.In.Slug = strings.TrimSpace(c.r.FormValue("slug"))
		if d.In.Slug == "" {
			d.In.Slug = Slugify(d.In.Name["en"])
		}
		p.Title = p.T("tag.title_new")
	} else {
		p.Title = p.L(d.In.Name)
	}
	for _, lang := range s.Cfg.Locales {
		if strings.TrimSpace(d.In.Name[lang]) == "" {
			p.Errors = append(p.Errors, FieldError{"name_" + lang, p.T("field.empty")})
		}
	}
	if d.New {
		if _, exists := s.Store.Current().Tag(d.In.Slug); exists {
			p.Errors = append(p.Errors, FieldError{"name_en", p.T("tag.error_exists")})
		}
	}
	if len(p.Errors) == 0 {
		if err := s.Store.SaveTag(d.In, c.user.Name); err != nil {
			p.Errors = append(p.Errors, FieldError{"name_en", p.T("coach.error_slug")})
		}
	}
	if len(p.Errors) > 0 {
		p.Data = d
		s.render(c, "tag", p, http.StatusUnprocessableEntity)
		return
	}
	s.flash(c, "ok", p.T("flash.saved"), "", "")
	s.redirect(c, "/admin/tags")
}

func (s *Server) tagDelete(c *ctx) {
	if err := s.Store.DeleteTag(c.r.PathValue("slug"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.deleted"), "", "")
	s.redirect(c, "/admin/tags")
}

func (s *Server) tagMove(c *ctx) {
	slug := c.r.PathValue("slug")
	if err := s.Store.MoveTag(slug, c.r.FormValue("dir"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.redirect(c, "/admin/tags?moved="+slug)
}
