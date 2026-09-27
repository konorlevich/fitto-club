package admin

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/store"
)

// The weekly grid: days across, half-hours down, 07:00–23:00. Slots are
// absolutely positioned blocks in their start cell, so overlaps (allowed,
// the club has several zones) sit side by side instead of breaking rowspans.
const (
	gridStart = 7
	gridEnd   = 23
	gridStep  = 30
)

type gridEvent struct {
	Key      string // classID-day-HHMM, matched by ?saved=
	Name     string
	Coach    string
	Start    string
	End      string
	Href     string
	Rows     int          // half-hours
	Lane     int          // 0..Lanes-1 when overlapping
	Lanes    int          // concurrent events at this start
	Conflict string       // names of overlapping slots
	Style    template.CSS // height / left / width (calc+var are safe here)
	Saved    bool
}

type gridCell struct {
	Day     int
	Time    string
	Href    string
	Aria    string
	Events  []gridEvent
	Covered bool // a slot lies over this cell: no "+" link underneath it
	Now     bool
	NowPct  int
}

type gridRow struct {
	Time  string // "19:00" on full hours, "" on half hours
	Cells [7]gridCell
}

type scheduleData struct {
	Day      int // selected day on phones
	Today    int
	Rows     []gridRow
	Days     []int
	Saved    string
	HasSlots bool
}

func minutesOf(clock string) int {
	h, _ := strconv.Atoi(clock[:2])
	m, _ := strconv.Atoi(clock[3:])
	return h*60 + m
}

func (s *Server) schedule(c *ctx) {
	snap := s.Store.Current()
	p := s.newPage(c, "schedule", c.copy(s).T("nav.schedule"))
	p.Wide = true
	d := scheduleData{Today: p.Today(), Saved: c.r.URL.Query().Get("saved")}
	d.Day = d.Today
	if day, _ := strconv.Atoi(c.r.URL.Query().Get("day")); day >= 1 && day <= 7 {
		d.Day = day
	}
	for i := 1; i <= 7; i++ {
		d.Days = append(d.Days, i)
	}
	p.Action = &Link{Href: fmt.Sprintf("/admin/schedule/new?day=%d&back=/admin/schedule?day=%d", d.Day, d.Day), Label: p.T("action.add")}

	nRows := (gridEnd - gridStart) * 60 / gridStep
	d.Rows = make([]gridRow, nRows)
	for r := range d.Rows {
		min := gridStart*60 + r*gridStep
		clock := fmt.Sprintf("%02d:%02d", min/60, min%60)
		if min%60 == 0 {
			d.Rows[r].Time = clock
		}
		for day := 1; day <= 7; day++ {
			d.Rows[r].Cells[day-1] = gridCell{Day: day, Time: clock,
				Href: fmt.Sprintf("/admin/schedule/new?day=%d&start=%s&back=/admin/schedule?day=%d", day, clock, day),
				Aria: p.F("schedule.add_aria", p.DayShort(day), clock)}
		}
	}
	// Now line in today's column.
	nowMin := c.now.Hour()*60 + c.now.Minute()
	if nowMin >= gridStart*60 && nowMin < gridEnd*60 {
		r := (nowMin - gridStart*60) / gridStep
		d.Rows[r].Cells[d.Today-1].Now = true
		d.Rows[r].Cells[d.Today-1].NowPct = (nowMin - gridStart*60 - r*gridStep) * 100 / gridStep
	}

	week := snap.Week()
	for day := 1; day <= 7; day++ {
		events := week[day-1]
		d.HasSlots = d.HasSlots || len(events) > 0
		// Lanes: events that overlap in time share the width.
		type placed struct {
			ev   store.SlotRef
			s, e int
			lane int
		}
		var pl []placed
		for _, ev := range events {
			st := minutesOf(ev.Slot.Start)
			pl = append(pl, placed{ev: ev, s: st, e: st + ev.Slot.Minutes})
		}
		for i := range pl {
			used := map[int]bool{}
			for j := range pl {
				if j != i && pl[j].s < pl[i].e && pl[i].s < pl[j].e && j < i {
					used[pl[j].lane] = true
				}
			}
			for used[pl[i].lane] {
				pl[i].lane++
			}
		}
		for i, x := range pl {
			if x.s < gridStart*60 || x.s >= gridEnd*60 {
				continue
			}
			r := (x.s - gridStart*60) / gridStep
			rows := (x.ev.Slot.Minutes + gridStep - 1) / gridStep
			lanes := 1
			var conflicts []string
			for j, y := range pl {
				if j != i && y.s < x.e && x.s < y.e {
					conflicts = append(conflicts, p.L(y.ev.Class.Name)+" "+y.ev.Slot.Start)
					if y.lane+1 > lanes {
						lanes = y.lane + 1
					}
				}
			}
			if x.lane+1 > lanes {
				lanes = x.lane + 1
			}
			ge := gridEvent{
				Key:  fmt.Sprintf("%s-%d-%s", "k-"+x.ev.Class.Slug, day, strings.ReplaceAll(x.ev.Slot.Start, ":", "")),
				Name: p.L(x.ev.Class.Name), Start: x.ev.Slot.Start, End: x.ev.Slot.End(), Rows: rows, Lane: x.lane, Lanes: lanes,
				Href: fmt.Sprintf("/admin/classes/k-%s/slots/%d?back=/admin/schedule?day=%d", x.ev.Class.Slug, slotIndex(x.ev.Class, x.ev.Slot), day),
			}
			if co, ok := snap.Coach(x.ev.Class.Coach); ok {
				ge.Coach = p.L(co.Name)
			}
			if len(conflicts) > 0 {
				ge.Conflict = p.F("schedule.conflict", strings.Join(conflicts, ", "))
			}
			ge.Saved = d.Saved != "" && ge.Key == d.Saved
			offset := (x.s - gridStart*60 - r*gridStep) * 100 / gridStep
			ge.Style = template.CSS(fmt.Sprintf("top:%d%%;height:calc(%d * var(--grid-half) - 3px);left:%d%%;width:%d%%",
				offset, rows, x.lane*100/lanes, 100/lanes))
			d.Rows[r].Cells[day-1].Events = append(d.Rows[r].Cells[day-1].Events, ge)
			for k := r; k < r+rows && k < nRows; k++ {
				d.Rows[k].Cells[day-1].Covered = true
			}
		}
	}
	p.Data = d
	s.render(c, "schedule", p, http.StatusOK)
}

func slotIndex(c content.Class, sl content.Slot) int {
	for i, x := range c.Slots {
		if x == sl {
			return i
		}
	}
	return 0
}
