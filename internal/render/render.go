// Package render parses templates once at boot and carries the per-page data
// and the helpers templates call. Templates never branch on locale: every
// localized string comes from .Copy or from a content.L through .T.
package render

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/site"
	"github.com/konorlevich/fitto-club/internal/store"
)

// Alt is one entry of the hreflang cluster: URL is absolute (hreflang needs
// it), Href is relative (the visible switcher stays on the visitor's host).
type Alt struct {
	Lang, URL, Href, Name string
}

type NavItem struct {
	Label, Href string
	Active      bool
}

// Page is the single data shape every template receives.
type Page struct {
	Lang   string
	Route  string // template name
	Path   string // locale-independent path, e.g. "/coaches/otar-chkadua"
	Cfg    site.Config
	Copy   *content.SiteCopy
	Snap   *store.Snapshot
	Now    time.Time // club time zone
	Assets map[string]string

	Title, Desc, Robots, Canonical, OGImage string
	Alternates                              []Alt
	Nav                                     []NavItem
	JSONLD                                  template.JS
	CSS                                     template.CSS
	JS                                      template.JS
	FontFaces                               template.CSS
	Preloads                                []string

	Status       content.Status
	VisitSurface string // "orange" or "black"
	NoVisit      bool   // 404 and privacy
	CTABar       bool

	// Page-specific data.
	Coach        *content.Coach
	Coaches      []content.Coach
	CoachClasses []content.Class
	Reviews      []content.Review
	Similar      []content.Coach
	FilterSpeaks string
	FilterTag    string
	TotalCoaches int
}

// ----------------------------------------------------------------- basics

// Asset returns a content-addressed URL, so immutable caching is safe.
func (p *Page) Asset(path string) string { return AssetURL(p.Assets, path) }

func AssetURL(assets map[string]string, path string) string {
	path = strings.TrimPrefix(path, "/")
	if h, ok := assets[path]; ok {
		return "/static/v/" + h + "/" + path
	}
	return "/static/" + path
}

func (p *Page) T(l content.L) string                { return l.Get(p.Lang) }
func (p *Page) F(format string, args ...any) string { return fmt.Sprintf(format, args...) }
func (p *Page) Href(path string) string             { return "/" + p.Lang + path }
func (p *Page) Tel() string                         { return "tel:" + content.Phone }
func (p *Page) PhoneDisplay() string                { return content.PhoneDisplay }
func (p *Page) DM() string                          { return content.InstagramDM }
func (p *Page) IG() string                          { return "https://www.instagram.com/" + content.Instagram + "/" }
func (p *Page) GoogleDir() string                   { return content.GoogleDirURL }
func (p *Page) AppleDir() string                    { return content.AppleDirURL }
func (p *Page) GoogleCard() string                  { return content.GoogleMapsURL }
func (p *Page) Year() int                           { return p.Now.Year() }
func (p *Page) Address() string                     { return content.Address.Street.Get(p.Lang) }
func (p *Page) Area() string                        { return content.Address.Area.Get(p.Lang) }
func (p *Page) Is(route string) bool                { return p.Route == route }
func (p *Page) Money(n int) string                  { return thousands(n) }

// Script is a string with the language it is written in.
type Script struct{ Text, Lang string }

// AddressOthers lists the street address in the other scripts, Georgian
// first: it is what a taxi driver reads.
func (p *Page) AddressOthers() []Script {
	var out []Script
	for _, l := range []string{"ka", "en", "ru"} {
		if l != p.Lang {
			out = append(out, Script{content.Address.Street[l], l})
		}
	}
	return out
}

// SoarlineURL is locale-aware and must equal creator.url in JSON-LD (§6).
func (p *Page) SoarlineURL() string {
	switch p.Lang {
	case "ka", "en", "es":
		return "https://soarline.studio/" + p.Lang
	}
	return "https://soarline.studio/en"
}

// OGLocale maps the page language to an Open Graph locale.
func OGLocale(lang string) string {
	return map[string]string{"en": "en_US", "ru": "ru_RU", "ka": "ka_GE"}[lang]
}

func (p *Page) OGLocale() string { return OGLocale(p.Lang) }

// thousands groups digits with a thin space, which reads the same in every
// locale here and never wraps: 1 550.
func thousands(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// ------------------------------------------------------------------- time

func (p *Page) DayName(iso int) string  { return p.Copy.Common.Days[iso-1] }
func (p *Page) DayShort(iso int) string { return p.Copy.Common.DaysShort[iso-1] }
func (p *Page) Today() int              { return content.ISODay(p.Now) }

// StatusText renders "Open until 23:00" / "Closed. Opens tomorrow at 08:00".
func (p *Page) StatusText() string {
	c := p.Copy.Common
	s := p.Status
	switch {
	case s.Open:
		return fmt.Sprintf(c.OpenUntil, s.Until)
	case s.NextDay == 0:
		return fmt.Sprintf(c.ClosedOpensAt, s.NextOpen)
	case s.NextDay == -1:
		return fmt.Sprintf(c.ClosedOpensDay, c.Tomorrow, s.NextOpen)
	default:
		return fmt.Sprintf(c.ClosedOpensDay, c.DaysOn[s.NextDay-1], s.NextOpen)
	}
}

// HoursRow is one line of the opening-hours table.
type HoursRow struct {
	Day     int
	Label   string
	Text    string
	IsToday bool
}

// HoursRows collapses identical consecutive days ("Mon-Fri 08:00-23:00").
func (p *Page) HoursRows() []HoursRow {
	w := p.Snap.Hours.Week
	var out []HoursRow
	for i := 0; i < 7; {
		j := i
		for j+1 < 7 && w[j+1] == w[i] {
			j++
		}
		label := p.DayShort(i + 1)
		if j > i {
			label += "–" + p.DayShort(j+1)
		}
		text := p.Copy.Common.Closed
		if !w[i].Closed() {
			text = w[i].Open + "–" + w[i].Close
		}
		today := p.Today()
		out = append(out, HoursRow{Day: i + 1, Label: label, Text: text, IsToday: today >= i+1 && today <= j+1})
		i = j + 1
	}
	return out
}

// DayRef is one day of the class schedule.
type DayRef struct {
	Day     int
	IsToday bool
	Slots   []store.SlotRef
}

// WeekFromToday starts the schedule list on today, so a member opening the
// page on Sunday evening sees Sunday first (IA flow 5).
func (p *Page) WeekFromToday() []DayRef {
	week := p.Snap.Week()
	today := p.Today()
	out := make([]DayRef, 0, 7)
	for i := range 7 {
		d := (today-1+i)%7 + 1
		out = append(out, DayRef{Day: d, IsToday: i == 0, Slots: week[d-1]})
	}
	return out
}

// WeekGrid is the desktop table: distinct start times as rows, Mon-Sun as
// columns.
type GridRow struct {
	Time  string
	Cells [7][]store.SlotRef
}

func (p *Page) WeekGrid() []GridRow {
	week := p.Snap.Week()
	var times []string
	for _, day := range week {
		for _, r := range day {
			if !slices.Contains(times, r.Slot.Start) {
				times = append(times, r.Slot.Start)
			}
		}
	}
	slices.Sort(times)
	rows := make([]GridRow, len(times))
	for i, t := range times {
		rows[i].Time = t
		for d, day := range week {
			for _, r := range day {
				if r.Slot.Start == t {
					rows[i].Cells[d] = append(rows[i].Cells[d], r)
				}
			}
		}
	}
	return rows
}

// NextClass is the home teaser line.
func (p *Page) NextClass() string {
	r, inDays, ok := p.Snap.NextSlot(p.Now)
	if !ok {
		return ""
	}
	c := p.Copy.Common
	when := c.Today
	switch {
	case inDays == 1:
		when = c.Tomorrow
	case inDays > 1:
		when = c.Days[r.Slot.Day-1]
	}
	return fmt.Sprintf(p.Copy.Home.ClassesNext, p.T(r.Class.Name), strings.ToLower(when), r.Slot.Start)
}

// ------------------------------------------------------------------ coaches

func (p *Page) CoachHref(slug string) string { return p.Href("/coaches/" + slug) }

func (p *Page) CoachPhoto(slug string, w int) string {
	return p.Asset(fmt.Sprintf("img/coaches/%s-%d.webp", slug, w))
}

func (p *Page) CoachSrcset(slug string) string {
	return p.CoachPhoto(slug, 400) + " 400w, " + p.CoachPhoto(slug, 800) + " 800w"
}

func (p *Page) CoachAlt(c content.Coach) string {
	return fmt.Sprintf(p.Copy.Coaches.PhotoAlt, p.T(c.Name))
}

// PT is the personal-training price as text, honest about "from" and USD.
func (p *Page) PT(c content.Coach) string {
	if c.PTPrice <= 0 {
		return ""
	}
	price := thousands(c.PTPrice) + " " + c.Currency()
	if c.Currency() == "USD" {
		price = "$" + thousands(c.PTPrice)
	}
	if c.PTFrom {
		return fmt.Sprintf(p.Copy.Common.From, price)
	}
	return price
}

// PTLine is the card line: "PT from 90 GEL" / "PT 120 GEL".
func (p *Page) PTLine(c content.Coach) string {
	if c.PTPrice <= 0 {
		return p.Copy.Coach.PriceOnAsk
	}
	price := thousands(c.PTPrice) + " " + c.Currency()
	if c.Currency() == "USD" {
		price = "$" + thousands(c.PTPrice)
	}
	if c.PTFrom {
		return fmt.Sprintf(p.Copy.Coaches.PTFrom, price)
	}
	return fmt.Sprintf(p.Copy.Coaches.PTPrice, price)
}

// PriceParts is a price split for typesetting: the number in Archivo Black,
// "from" and the currency in the text face.
type PriceParts struct{ Pre, Num, Unit string }

func (p *Page) PTParts(c content.Coach) PriceParts {
	pp := PriceParts{Num: thousands(c.PTPrice), Unit: c.Currency()}
	if c.Currency() == "USD" {
		pp.Num, pp.Unit = "$"+pp.Num, ""
	}
	if c.PTFrom {
		pp.Pre = p.FromPre()
		pp.Unit += p.FromPost()
	}
	return pp
}

// CoachLinkName is a coach's name for a schedule line; empty if unknown.
func (p *Page) CoachLinkName(slug string) string {
	if c, ok := p.Snap.Coach(slug); ok {
		return p.T(c.Name)
	}
	return ""
}

// CoachLink links a published coach; an unpublished one renders as plain
// text, so a class never points at a redirect (BRIEF.md §4).
func (p *Page) CoachLink(slug string) template.HTML {
	c, ok := p.Snap.Coach(slug)
	if !ok {
		return ""
	}
	name := template.HTMLEscapeString(p.T(c.Name))
	if !c.Published {
		return template.HTML(name)
	}
	return template.HTML(`<a href="` + template.HTMLEscapeString(p.CoachHref(slug)) + `">` + name + `</a>`)
}

// ZonesCtx feeds the zones partial: all six on About, three on the home
// page (a short list plus a link, rather than hiding photos at small widths).
type ZonesCtx struct {
	P       *Page
	Zones   []content.Zone
	Compact bool
}

// homeZones are the three that answer "what is it like": the floor, the
// separate functional room, and the honest one - the small lockers.
var homeZones = []string{"hall", "crossfit", "lockers"}

func (p *Page) AllZones() ZonesCtx { return ZonesCtx{P: p, Zones: content.Zones} }

func (p *Page) HomeZones() ZonesCtx {
	out := make([]content.Zone, 0, len(homeZones))
	for _, key := range homeZones {
		for _, z := range content.Zones {
			if z.Key == key {
				out = append(out, z)
			}
		}
	}
	return ZonesCtx{P: p, Zones: out, Compact: true}
}
func (p *Page) Partners() []content.Partner { return content.Partners }

func (p *Page) TagName(slug string) string {
	if t, ok := p.Snap.Tag(slug); ok {
		return p.T(t.Name)
	}
	return slug
}

// TagNames are the first n tag names of a coach.
func (p *Page) TagNames(c content.Coach, n int) []string {
	out := make([]string, 0, n)
	for i, t := range c.Tags {
		if i >= n {
			break
		}
		out = append(out, p.TagName(t))
	}
	return out
}

func langIndex(code string) int { return slices.Index(content.SpokenLanguages, code) }

func (p *Page) LangName(code string) string {
	if i := langIndex(code); i >= 0 {
		return p.Copy.Common.LangNames[i]
	}
	return code
}

func (p *Page) LangShort(code string) string {
	if i := langIndex(code); i >= 0 {
		return p.Copy.Common.LangShort[i]
	}
	return strings.ToUpper(code)
}

// FilterHref builds the one canonical form of a filter URL: speaks before
// tag. Any other order 301s to this (IA, URL strategy).
func (p *Page) FilterHref(speaks, tag string) string {
	base := p.Href("/coaches")
	parts := []string{}
	if speaks != "" {
		parts = append(parts, "speaks="+url.QueryEscape(speaks))
	}
	if tag != "" {
		parts = append(parts, "tag="+url.QueryEscape(tag))
	}
	if len(parts) == 0 {
		return base
	}
	return base + "?" + strings.Join(parts, "&")
}

// Toggle links: clicking the active chip clears that dimension.
func (p *Page) SpeaksHref(code string) string {
	if p.FilterSpeaks == code {
		return p.FilterHref("", p.FilterTag)
	}
	return p.FilterHref(code, p.FilterTag)
}

func (p *Page) TagHref(slug string) string {
	if p.FilterTag == slug {
		return p.FilterHref(p.FilterSpeaks, "")
	}
	return p.FilterHref(p.FilterSpeaks, slug)
}

func (p *Page) Filtered() bool { return p.FilterSpeaks != "" || p.FilterTag != "" }

// CardCtx carries the page into the coach-card partial inside a range.
type CardCtx struct {
	P         *Page
	C         content.Coach
	ShowPrice bool
}

func (p *Page) Card(c content.Coach, price bool) CardCtx { return CardCtx{p, c, price} }

// ------------------------------------------------------------------ reviews

type ReviewCtx struct {
	P *Page
	R content.Review
	// Clamp: in the home strip a long review shows its opening and folds
	// the rest into <details>; on a coach page it is shown whole.
	Clamp bool
}

func (p *Page) Rev(r content.Review) ReviewCtx      { return ReviewCtx{p, r, false} }
func (p *Page) RevShort(r content.Review) ReviewCtx { return ReviewCtx{p, r, true} }

// reviewBudget is roughly nine lines of the strip card at 375.
const reviewBudget = 320

// Split divides a review into the part shown and the part folded. Nothing
// is folded unless it saves real space (more than a third over budget), and
// a long first paragraph is cut at a sentence end, never mid-word. The
// author's text is never altered, only divided.
// ReviewParts is Split's result; templates cannot take two return values.
type ReviewParts struct{ Head, Tail []string }

func (c ReviewCtx) Split() ReviewParts {
	h, t := c.split()
	return ReviewParts{h, t}
}

func (c ReviewCtx) split() (head, tail []string) {
	paras := c.P.Paras(c.R.Text)
	total := 0
	for _, p := range paras {
		total += utf8.RuneCountInString(p)
	}
	if !c.Clamp || total <= reviewBudget*4/3 {
		return paras, nil
	}
	used := 0
	for i, p := range paras {
		n := utf8.RuneCountInString(p)
		if used+n <= reviewBudget {
			head = append(head, p)
			used += n
			continue
		}
		if used == 0 || used < reviewBudget/2 {
			if cut := sentenceCut(p, reviewBudget-used); cut > 0 {
				head = append(head, p[:cut])
				tail = append(tail, strings.TrimSpace(p[cut:]))
				return head, append(tail, paras[i+1:]...)
			}
		}
		return head, paras[i:]
	}
	return head, nil
}

// sentenceCut returns the byte index just after the last sentence end that
// fits within budget runes, or 0 if there is none.
func sentenceCut(s string, budget int) int {
	best, runes := 0, 0
	for i, r := range s {
		runes++
		if runes > budget {
			break
		}
		if r == '.' || r == '!' || r == '?' || r == '…' {
			next := i + utf8.RuneLen(r)
			if next >= len(s) || s[next] == ' ' || s[next] == '\n' {
				best = next
			}
		}
	}
	return best
}

func (p *Page) Stars(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 5 {
		n = 5
	}
	return strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
}

func (p *Page) StarsLabel(n int) string { return fmt.Sprintf(p.Copy.Common.StarsOf, n) }

// RatingLine uses the locale's decimal mark: 4.6 in English, 4,6 in
// Russian and Georgian.
func (p *Page) RatingLine() string {
	r := content.GoogleRating
	if p.Lang != "en" {
		r = strings.Replace(r, ".", ",", 1)
	}
	return fmt.Sprintf(p.Copy.Common.RatingLine, r, content.GoogleCount)
}

// Paras splits a review into paragraphs; blank lines separate them.
func (p *Page) Paras(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ------------------------------------------------------------------ misc

// Letter is one glyph of the bouncing word.
type Letter struct {
	Ch string
	I  int
}

// Letters splits a word into glyphs for the per-letter bounce. Runes, not
// bytes: the Russian and Georgian words are multi-byte.
func (p *Page) Letters(word string) []Letter {
	out := make([]Letter, 0, utf8.RuneCountInString(word))
	i := 0
	for _, r := range word {
		out = append(out, Letter{string(r), i})
		i++
	}
	return out
}

// FromPre and FromPost split the "from %s" format around the number, so the
// number can be set large on its own: "from | 129 | GEL" in English,
// "129 | GEL-დან" in Georgian, where "from" is a suffix.
func (p *Page) FromPre() string {
	pre, _, _ := strings.Cut(p.Copy.Common.From, "%s")
	return strings.TrimSpace(pre)
}

func (p *Page) FromPost() string {
	_, post, _ := strings.Cut(p.Copy.Common.From, "%s")
	return post
}

func (p *Page) TermLabel(months int) string {
	switch months {
	case 1:
		return p.Copy.Memberships.TermLabels[0]
	case 3:
		return p.Copy.Memberships.TermLabels[1]
	case 6:
		return p.Copy.Memberships.TermLabels[2]
	case 12:
		return p.Copy.Memberships.TermLabels[3]
	}
	return strconv.Itoa(months)
}

func (p *Page) PlaceSrc(name string, w int) string {
	return p.Asset(fmt.Sprintf("img/place/%s-%d.webp", name, w))
}

func (p *Page) PlaceSrcset(name string, widths ...int) string {
	parts := make([]string, 0, len(widths))
	for _, w := range widths {
		if _, ok := p.Assets[fmt.Sprintf("img/place/%s-%d.webp", name, w)]; ok {
			parts = append(parts, fmt.Sprintf("%s %dw", p.PlaceSrc(name, w), w))
		}
	}
	return strings.Join(parts, ", ")
}

func (p *Page) Seq(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// ------------------------------------------------------------------ parse

func funcs() template.FuncMap {
	return template.FuncMap{
		"add":      func(a, b int) int { return a + b },
		"ogLocale": OGLocale,
		"lower":    strings.ToLower,
		"join":     strings.Join,
	}
}

// Templates holds one parsed template per page: layout + partials + page.
type Templates map[string]*template.Template

func Parse(fsys fs.FS, pages []string) (Templates, error) {
	out := make(Templates, len(pages))
	for _, name := range pages {
		t, err := template.New(name).Funcs(funcs()).ParseFS(fsys,
			"web/templates/layout.html",
			"web/templates/partials/*.html",
			"web/templates/pages/"+name+".html",
		)
		if err != nil {
			return nil, fmt.Errorf("parsing page %q: %w", name, err)
		}
		for _, need := range []string{"layout", "content"} {
			if t.Lookup(need) == nil {
				return nil, fmt.Errorf("page %q: no {{define %q}}", name, need)
			}
		}
		out[name] = t
	}
	return out, nil
}
