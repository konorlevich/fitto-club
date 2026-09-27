package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/render"
	"github.com/konorlevich/fitto-club/internal/site"
	"github.com/konorlevich/fitto-club/internal/store"
)

type obj = map[string]any

// jsonLD builds the page's @graph from the same snapshot that renders the
// page, so markup can never claim what the page does not show (§6 "visible
// truth"). No AggregateRating: the Google score is shown as text only, and a
// rating computed from a hand-picked subset would be self-serving.
func (s *Server) jsonLD(p *render.Page) template.JS {
	base := s.Cfg.BaseURL
	club := base + "/#club"
	c := p.Copy
	graph := []obj{
		{
			"@type": "WebSite", "@id": base + "/#website", "url": base + "/", "name": c.Common.SiteName,
			"inLanguage": p.Lang,
			"creator": obj{"@type": "Organization", "@id": "https://soarline.studio/#organization",
				"name": "Soarline Studio", "url": p.SoarlineURL()},
			"creditText": "Website created by Soarline Studio",
		},
		s.clubLD(p, club),
	}
	page := obj{
		"@type": "WebPage", "@id": p.Canonical + "#webpage", "url": p.Canonical,
		"name": p.Title, "description": p.Desc, "inLanguage": p.Lang,
		"isPartOf": obj{"@id": base + "/#website"}, "about": obj{"@id": club},
		"dateModified": p.Snap.Date(p.Path),
	}
	if p.Canonical != "" {
		graph = append(graph, page)
	}
	switch p.Route {
	case "memberships":
		var offers []obj
		for _, m := range p.Snap.Memberships {
			for _, t := range m.Terms {
				if t.Price <= 0 {
					continue
				}
				offers = append(offers, obj{"@type": "Offer",
					"name":  p.T(m.Name) + " · " + p.TermLabel(t.Months),
					"price": t.Price, "priceCurrency": "GEL", "seller": obj{"@id": club}})
			}
		}
		offers = append(offers, obj{"@type": "Offer", "name": c.Memberships.SingleLabel,
			"price": p.Snap.SingleVisit, "priceCurrency": "GEL", "seller": obj{"@id": club}})
		graph = append(graph, obj{"@type": "OfferCatalog", "name": c.Memberships.TypesHeading, "itemListElement": offers})
	case "coach":
		k := p.Coach
		person := obj{"@type": "Person", "@id": p.Canonical + "#person", "name": p.T(k.Name),
			"jobTitle": c.Coach.PTHeading, "description": p.T(k.Bio), "worksFor": obj{"@id": club},
			"knowsLanguage": k.Speaks, "url": p.Canonical}
		if k.Photo {
			person["image"] = base + p.CoachPhoto(k.Slug, 800)
		}
		if k.Instagram != "" {
			person["sameAs"] = []string{"https://www.instagram.com/" + k.Instagram + "/"}
		}
		if k.PTPrice > 0 {
			person["makesOffer"] = obj{"@type": "Offer", "name": c.Coach.PTHeading,
				"price": k.PTPrice, "priceCurrency": k.Currency(),
				"priceSpecification": obj{"@type": "UnitPriceSpecification", "price": k.PTPrice,
					"priceCurrency": k.Currency(), "unitCode": "HUR"}}
		}
		graph = append(graph, person)
		for _, r := range p.Reviews {
			graph = append(graph, reviewLD(r, obj{"@id": club}))
		}
		graph = append(graph, obj{"@type": "BreadcrumbList", "itemListElement": []obj{
			{"@type": "ListItem", "position": 1, "name": c.Nav.Home, "item": base + "/" + p.Lang + "/"},
			{"@type": "ListItem", "position": 2, "name": c.Nav.Coaches, "item": base + "/" + p.Lang + "/coaches"},
			{"@type": "ListItem", "position": 3, "name": p.T(k.Name), "item": p.Canonical},
		}})
	case "classes":
		for _, cl := range p.Snap.Classes {
			course := obj{"@type": "Course", "name": p.T(cl.Name), "description": p.T(cl.Desc),
				"provider": obj{"@id": club}, "inLanguage": cl.Langs}
			var inst []obj
			for _, sl := range cl.Slots {
				ci := obj{"@type": "CourseInstance", "courseMode": "onsite", "location": obj{"@id": club},
					"courseSchedule": obj{"@type": "Schedule", "repeatFrequency": "P1W",
						"byDay": "https://schema.org/" + dayNames[sl.Day-1], "startTime": sl.Start}}
				if e := sl.End(); e != "" {
					ci["courseSchedule"].(obj)["endTime"] = e
				}
				if coach, ok := p.Snap.PublishedCoach(cl.Coach); ok {
					ci["instructor"] = obj{"@type": "Person", "name": p.T(coach.Name), "url": base + p.CoachHref(coach.Slug)}
				}
				inst = append(inst, ci)
			}
			course["hasCourseInstance"] = inst
			if cl.Price > 0 {
				course["offers"] = obj{"@type": "Offer", "price": cl.Price, "priceCurrency": "GEL", "category": "Paid"}
			}
			graph = append(graph, course)
		}
	case "massage":
		var offers []obj
		for _, m := range p.Snap.Massage {
			offers = append(offers, obj{"@type": "Offer", "name": fmt.Sprintf("%s, %d %s", p.T(m.Name), m.Minutes, c.Common.Minutes),
				"price": m.Price, "priceCurrency": "GEL"})
		}
		graph = append(graph, obj{"@type": "Service", "name": c.Massage.Heading, "serviceType": "Massage",
			"provider": obj{"@id": club}, "areaServed": "Tbilisi",
			"hasOfferCatalog": obj{"@type": "OfferCatalog", "name": c.Massage.PriceHeading, "itemListElement": offers}})
	case "home":
		for _, r := range p.Reviews {
			graph = append(graph, reviewLD(r, obj{"@id": club}))
		}
	}
	b, _ := json.Marshal(obj{"@context": "https://schema.org", "@graph": graph})
	return template.JS(b)
}

var dayNames = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

func (s *Server) clubLD(p *render.Page, id string) obj {
	var spec []obj
	h := p.Snap.Hours
	for i, d := range h.Week {
		if d.Closed() {
			continue
		}
		spec = append(spec, obj{"@type": "OpeningHoursSpecification", "dayOfWeek": "https://schema.org/" + dayNames[i],
			"opens": d.Open, "closes": d.Close})
	}
	for _, sd := range h.Special {
		o := obj{"@type": "OpeningHoursSpecification", "validFrom": sd.Date, "validThrough": sd.Date}
		if sd.Open == "" {
			o["opens"], o["closes"] = "00:00", "00:00"
		} else {
			o["opens"], o["closes"] = sd.Open, sd.Close
		}
		spec = append(spec, o)
	}
	return obj{
		"@type": "HealthClub", "@id": id, "name": p.Copy.Common.SiteName, "url": s.Cfg.BaseURL + "/" + p.Lang + "/",
		"description": p.Copy.Common.Tagline, "telephone": content.Phone,
		"image": s.Cfg.BaseURL + p.PlaceSrc("hall-neon", 960), "logo": s.Cfg.BaseURL + p.Asset("img/brand/icon-512.png"),
		"priceRange": "20–1550 GEL", "currenciesAccepted": "GEL",
		"address": obj{"@type": "PostalAddress", "streetAddress": content.Address.Street.Get(p.Lang),
			"addressLocality": "Tbilisi", "addressRegion": "Vake", "addressCountry": "GE"},
		"geo":                       obj{"@type": "GeoCoordinates", "latitude": content.Lat, "longitude": content.Lon},
		"hasMap":                    content.GoogleMapsURL,
		"sameAs":                    []string{"https://www.instagram.com/" + content.Instagram + "/", content.GoogleMapsURL},
		"openingHoursSpecification": spec,
		"foundingDate":              content.Founded,
		"founder":                   obj{"@type": "Person", "name": "Otar Chkadua"},
		"knowsLanguage":             []string{"ka", "en", "ru"},
		"amenityFeature": []obj{
			{"@type": "LocationFeatureSpecification", "name": "Showers", "value": true},
			{"@type": "LocationFeatureSpecification", "name": "Lockers", "value": true},
			{"@type": "LocationFeatureSpecification", "name": "Massage room", "value": true},
		},
	}
}

func reviewLD(r content.Review, item obj) obj {
	return obj{"@type": "Review", "itemReviewed": item, "inLanguage": r.Lang,
		"author": obj{"@type": "Person", "name": r.Author}, "reviewBody": r.Text,
		"reviewRating": obj{"@type": "Rating", "ratingValue": r.Rating, "bestRating": 5, "worstRating": 1},
		"publisher":    obj{"@type": "Organization", "name": "Google"}}
}

// ------------------------------------------------------------------- files

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if s.Cfg.NoIndex {
		// Pre-launch: nothing here is for the index yet (NOINDEX=1).
		fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
		return
	}
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /admin\n\nSitemap: %s/sitemap.xml\n# LLM summary: %s/llms.txt\n",
		s.Cfg.BaseURL, s.Cfg.BaseURL)
}

type smEntry struct{ Path, LastMod string }

// entries lists every indexable path with its real content date - never
// build time (checklist §6). Filtered coach lists, privacy and 404 are out.
func (s *Server) entries(snap *store.Snapshot) []smEntry {
	out := []smEntry{{"/", snap.Date("/")}}
	for _, sec := range sections {
		if sec.Path == "/privacy" {
			continue
		}
		out = append(out, smEntry{sec.Path, snap.Date(sec.Path)})
	}
	for _, c := range snap.Published() {
		p := "/coaches/" + c.Slug
		d := max(c.Updated, snap.Date(p))
		out = append(out, smEntry{p, d})
	}
	return out
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	for _, e := range s.entries(s.Store.Current()) {
		for _, lang := range s.Cfg.Locales {
			fmt.Fprintf(&b, "  <url>\n    <loc>%s/%s%s</loc>\n    <lastmod>%s</lastmod>\n", s.Cfg.BaseURL, lang, e.Path, e.LastMod)
			for _, alt := range s.Cfg.Locales {
				fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=\"%s\" href=\"%s/%s%s\"/>\n", alt, s.Cfg.BaseURL, alt, e.Path)
			}
			fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=\"x-default\" href=\"%s/%s%s\"/>\n", s.Cfg.BaseURL, site.DefaultLocale, e.Path)
			b.WriteString("  </url>\n")
		}
	}
	b.WriteString("</urlset>\n")
	_, _ = w.Write([]byte(b.String()))
}

// llms mirrors the on-page facts as Markdown for AI assistants (§6).
func (s *Server) llms(w http.ResponseWriter, r *http.Request, lang string) {
	snap := s.Store.Current()
	c := s.Copy[lang]
	p := s.newPage(lang, "llms", "/", snap, s.now())
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n> %s. %s\n\n", c.Common.SiteName, c.Common.Tagline, c.Home.Description)
	fmt.Fprintf(&b, "- %s: %s, %s (%s)\n", c.Visit.AddressHead, p.Address(), p.Area(), content.Address.Street["ka"])
	fmt.Fprintf(&b, "- %s: %s\n- Instagram: https://www.instagram.com/%s/\n- Google Maps: %s\n", c.Common.ContactHeading, content.PhoneDisplay, content.Instagram, content.GoogleMapsURL)
	fmt.Fprintf(&b, "- %s: ", c.Common.HoursHeading)
	for i, row := range p.HoursRows() {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(row.Label + " " + row.Text)
	}
	fmt.Fprintf(&b, "\n\n## %s\n\n%s\n\n", c.Memberships.Heading, c.Memberships.FreeText)
	fmt.Fprintf(&b, "- %s: %d GEL\n", c.Memberships.SingleLabel, snap.SingleVisit)
	for _, m := range snap.Memberships {
		for _, t := range m.Terms {
			if t.Price > 0 {
				fmt.Fprintf(&b, "- %s, %s: %d GEL\n", p.T(m.Name), p.TermLabel(t.Months), t.Price)
			}
		}
	}
	fmt.Fprintf(&b, "\n## %s\n\n", c.Coaches.Heading)
	for _, k := range snap.Published() {
		langs := make([]string, 0, len(k.Speaks))
		for _, l := range k.Speaks {
			langs = append(langs, p.LangName(l))
		}
		fmt.Fprintf(&b, "- [%s](%s/%s/coaches/%s): %s. %s. %s\n", p.T(k.Name), s.Cfg.BaseURL, lang, k.Slug,
			strings.Join(p.TagNames(k, 5), ", "), strings.Join(langs, ", "), p.PTLine(k))
	}
	fmt.Fprintf(&b, "\n## %s\n\n", c.Classes.Heading)
	for _, cl := range snap.Classes {
		var slots []string
		for _, sl := range cl.Slots {
			slots = append(slots, p.DayShort(sl.Day)+" "+sl.Start)
		}
		fmt.Fprintf(&b, "- %s: %s. %d GEL %s\n", p.T(cl.Name), strings.Join(slots, ", "), cl.Price, p.T(cl.PriceNote))
	}
	fmt.Fprintf(&b, "\n## %s\n\n", c.Massage.Heading)
	for _, m := range snap.Massage {
		fmt.Fprintf(&b, "- %s, %d %s: %d GEL\n", p.T(m.Name), m.Minutes, c.Common.Minutes, m.Price)
	}
	_, _ = w.Write([]byte(b.String()))
}

func (s *Server) manifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	b, _ := json.Marshal(obj{
		"name": "Fitto Club", "short_name": "Fitto", "start_url": "/", "display": "standalone",
		"background_color": "#000000", "theme_color": "#000000",
		"icons": []obj{
			{"src": render.AssetURL(s.Assets, "img/brand/icon-192.png"), "sizes": "192x192", "type": "image/png"},
			{"src": render.AssetURL(s.Assets, "img/brand/icon-512.png"), "sizes": "512x512", "type": "image/png"},
		},
	})
	_, _ = w.Write(b)
}
