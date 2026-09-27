package admin

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/konorlevich/fitto-club/internal/store"
)

const historyPerPage = 50

type historyItem struct {
	historyLine
	Action     string
	RestoreURL string // set for deletions that can be undone
}

type historyData struct {
	Items    []historyItem
	Entity   string
	Entities []string
	Page     int
	HasMore  bool
}

var historyEntities = []string{"coach", "class", "review", "membership", "massage", "tag", "hours", "special", "user", "settings"}

func (s *Server) history(c *ctx) {
	p := s.newPage(c, "history", c.copy(s).T("nav.history"))
	q := c.r.URL.Query()
	d := historyData{Entity: q.Get("entity"), Entities: historyEntities}
	if !slices.Contains(historyEntities, d.Entity) {
		d.Entity = ""
	}
	d.Page, _ = strconv.Atoi(q.Get("page"))
	if d.Page < 1 {
		d.Page = 1
	}
	changes, err := s.Store.Changes(d.Entity, historyPerPage+1, (d.Page-1)*historyPerPage)
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	if len(changes) > historyPerPage {
		d.HasMore, changes = true, changes[:historyPerPage]
	}
	for _, ch := range changes {
		it := historyItem{historyLine: s.historyLine(p, ch), Action: ch.Action}
		if ch.Action == "delete" && s.stillDeleted(ch) {
			it.RestoreURL = restoreURL(ch)
		}
		d.Items = append(d.Items, it)
	}
	p.Data = d
	s.render(c, "history", p, http.StatusOK)
}

func restoreURL(ch store.Change) string {
	switch ch.Entity {
	case "coach":
		return "/admin/coaches/" + ch.EntityID + "/restore"
	case "class":
		return "/admin/classes/" + ch.EntityID + "/restore"
	case "review":
		return "/admin/reviews/" + ch.EntityID + "/restore"
	case "membership":
		return "/admin/memberships/" + ch.EntityID + "/restore"
	case "massage":
		return "/admin/massage/" + ch.EntityID + "/restore"
	}
	return ""
}

// stillDeleted hides the restore button once the record is back.
func (s *Server) stillDeleted(ch store.Change) bool {
	switch ch.Entity {
	case "coach":
		r, err := s.Store.CoachRow(ch.EntityID)
		return err == nil && r.Deleted
	case "class":
		r, err := s.Store.ClassRow(ch.EntityID)
		return err == nil && r.Deleted
	case "review":
		r, err := s.Store.ReviewRow(ch.EntityID)
		return err == nil && r.Deleted
	case "membership":
		r, err := s.Store.MembershipRow(ch.EntityID)
		return err == nil && r.Deleted
	case "massage":
		r, err := s.Store.MassageRow(ch.EntityID)
		return err == nil && r.Deleted
	}
	return false
}

func entityLabel(p *Page, e string) string {
	return strings.ToUpper(p.T("entity." + e)[:1]) + p.T("entity." + e)[1:]
}
