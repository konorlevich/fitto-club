package admin

import (
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/konorlevich/fitto-club/internal/store"
)

const reviewsPerPage = 30

type reviewListItem struct {
	store.ReviewRow
	CoachNames string
	Excerpt    string
}

type reviewsData struct {
	Items    []reviewListItem
	Deleted  bool
	DelCount int
	Page     int
	HasMore  bool
	Coach    string
	Coaches  []coachOption
}

type coachOption struct {
	ID, Name string
	Selected bool
}

// DetectLang guesses the language a review was written in from its script:
// Mkhedruli → ka, Cyrillic → ru, Latin → en. Mixed text takes the majority.
func DetectLang(text string) string {
	var ka, cy, la int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Georgian, r):
			ka++
		case unicode.Is(unicode.Cyrillic, r):
			cy++
		case unicode.Is(unicode.Latin, r):
			la++
		}
	}
	switch {
	case ka == 0 && cy == 0 && la == 0:
		return ""
	case ka >= cy && ka >= la:
		return "ka"
	case cy >= la:
		return "ru"
	default:
		return "en"
	}
}

func (s *Server) coachOptions(p *Page, selected []string) []coachOption {
	rows, _ := s.Store.CoachRows()
	var out []coachOption
	for _, r := range rows {
		if r.Deleted {
			continue
		}
		name := p.L(r.Name)
		if !r.Published {
			name += " · " + p.T("meta.draft")
		}
		sel := false
		for _, id := range selected {
			if id == r.ID {
				sel = true
			}
		}
		out = append(out, coachOption{ID: r.ID, Name: name, Selected: sel})
	}
	return out
}

func excerpt(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	cut := string(r[:n])
	if i := strings.LastIndexByte(cut, ' '); i > n/2 {
		cut = cut[:i]
	}
	return cut + "…"
}

func (s *Server) reviewsList(c *ctx) {
	p := s.newPage(c, "reviews", c.copy(s).T("nav.reviews"))
	p.Action = &Link{Href: "/admin/reviews/new", Label: p.T("action.add")}
	q := c.r.URL.Query()
	d := reviewsData{Deleted: q.Get("deleted") == "1", Coach: q.Get("coach")}
	d.Page, _ = strconv.Atoi(q.Get("page"))
	if d.Page < 1 {
		d.Page = 1
	}
	rows, err := s.Store.ReviewRows(d.Deleted)
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	if !d.Deleted {
		del, _ := s.Store.ReviewRows(true)
		d.DelCount = len(del)
	}
	d.Coaches = s.coachOptions(p, []string{d.Coach})
	names := map[string]string{}
	for _, o := range d.Coaches {
		names[o.ID] = strings.TrimSuffix(o.Name, " · "+p.T("meta.draft"))
	}
	var filtered []store.ReviewRow
	for _, r := range rows {
		if d.Coach != "" {
			found := false
			for _, cid := range r.CoachIDs {
				if cid == d.Coach {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, r)
	}
	start := (d.Page - 1) * reviewsPerPage
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + reviewsPerPage
	if end < len(filtered) {
		d.HasMore = true
	} else {
		end = len(filtered)
	}
	for _, r := range filtered[start:end] {
		var cn []string
		for _, cid := range r.CoachIDs {
			if n := names[cid]; n != "" {
				cn = append(cn, n)
			}
		}
		d.Items = append(d.Items, reviewListItem{ReviewRow: r, CoachNames: strings.Join(cn, ", "), Excerpt: excerpt(r.Text, 90)})
	}
	p.Data = d
	s.render(c, "reviews", p, http.StatusOK)
}

type reviewFormData struct {
	New     bool
	In      store.ReviewInput
	Coaches []coachOption
	Deleted bool
	Back    string
}

func (s *Server) reviewForm(c *ctx) {
	p := s.newPage(c, "reviews", "")
	p.Back = "/admin/reviews"
	d := reviewFormData{Back: safeBack(c.r.URL.Query().Get("back"), "/admin/reviews")}
	if id := c.r.PathValue("id"); id != "" {
		r, err := s.Store.ReviewRow(id)
		if err != nil {
			s.errorPage(c, http.StatusNotFound)
			return
		}
		d.In = store.ReviewInput{ID: r.ID, Author: r.Author, Lang: r.Lang, Text: r.Text, URL: r.URL, Rating: r.Rating,
			Date: r.Date, Home: r.Home, Sort: r.Sort, CoachIDs: r.CoachIDs}
		d.Deleted = r.Deleted
		p.Title = p.T("review.title_edit")
	} else {
		d.New = true
		d.In = store.ReviewInput{Rating: 5, Date: c.now.Format("2006-01"), Sort: 50}
		p.Title = p.T("review.title_new")
	}
	d.Coaches = s.coachOptions(p, d.In.CoachIDs)
	p.Data = d
	s.render(c, "review", p, http.StatusOK)
}

func (s *Server) reviewPost(c *ctx) {
	p := s.newPage(c, "reviews", "")
	p.Back = "/admin/reviews"
	r := c.r
	d := reviewFormData{New: r.PathValue("id") == "", Back: safeBack(r.FormValue("back"), "/admin/reviews")}
	rating, _ := strconv.Atoi(r.FormValue("rating"))
	sort, _ := strconv.Atoi(r.FormValue("sort"))
	if sort == 0 {
		sort = 50
	}
	d.In = store.ReviewInput{
		ID: r.PathValue("id"), Author: strings.TrimSpace(r.FormValue("author")), Lang: r.FormValue("lang"),
		Text: strings.TrimSpace(r.FormValue("text")), URL: strings.TrimSpace(r.FormValue("url")), Rating: rating,
		Date: strings.TrimSpace(r.FormValue("date")), Home: r.FormValue("home") == "1", Sort: sort, CoachIDs: r.Form["coach"],
	}
	if d.In.Lang == "" {
		d.In.Lang = DetectLang(d.In.Text)
	}
	if d.New {
		p.Title = p.T("review.title_new")
	} else {
		p.Title = p.T("review.title_edit")
	}
	if d.In.Author == "" {
		p.Errors = append(p.Errors, FieldError{"author", p.T("field.empty")})
	}
	if d.In.Text == "" {
		p.Errors = append(p.Errors, FieldError{"text", p.T("field.empty")})
	}
	if d.In.Lang == "" {
		p.Errors = append(p.Errors, FieldError{"lang", p.T("review.error_lang")})
	}
	if d.In.Rating < 1 || d.In.Rating > 5 {
		p.Errors = append(p.Errors, FieldError{"rating", p.T("review.error_rating")})
	}
	if len(p.Errors) == 0 {
		if _, err := s.Store.SaveReview(d.In, c.user.Name); err != nil {
			field := "url"
			if !strings.Contains(err.Error(), "url") {
				field = "date"
			}
			p.Errors = append(p.Errors, FieldError{field, p.T("review.error_" + field)})
		}
	}
	if len(p.Errors) > 0 {
		d.Coaches = s.coachOptions(p, d.In.CoachIDs)
		p.Data = d
		s.render(c, "review", p, http.StatusUnprocessableEntity)
		return
	}
	link := "/" + s.siteLang(c) + "/#reviews"
	if len(d.In.CoachIDs) > 0 {
		if row, err := s.Store.CoachRow(d.In.CoachIDs[0]); err == nil && row.Published {
			link = "/" + s.siteLang(c) + "/coaches/" + row.Slug
		}
	}
	s.flash(c, "ok", p.T("flash.saved"), link, p.T("action.open_site"))
	s.redirect(c, d.Back)
}

func (s *Server) reviewDelete(c *ctx) {
	if err := s.Store.DeleteReview(c.r.PathValue("id"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	p := s.newPage(c, "reviews", "")
	s.flash(c, "ok", p.T("flash.deleted"), "/admin/reviews/"+c.r.PathValue("id")+"/restore", p.T("action.restore"))
	s.redirect(c, "/admin/reviews")
}

func (s *Server) reviewRestore(c *ctx) {
	if err := s.Store.RestoreReview(c.r.PathValue("id"), c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	s.flash(c, "ok", c.copy(s).T("flash.restored"), "", "")
	s.redirect(c, "/admin/reviews")
}
