// Package render prints tasks, projects and dates for humans (colour when the output is a terminal).
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/api"
)

// Style wraps text in ANSI colours when enabled (a terminal, NO_COLOR unset, --no-color not given).
type Style struct{ On bool }

func (s Style) wrap(code, text string) string {
	if !s.On || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s Style) Bold(t string) string   { return s.wrap("1", t) }
func (s Style) Dim(t string) string    { return s.wrap("2", t) }
func (s Style) Red(t string) string    { return s.wrap("31", t) }
func (s Style) Green(t string) string  { return s.wrap("32", t) }
func (s Style) Yellow(t string) string { return s.wrap("33", t) }
func (s Style) Blue(t string) string   { return s.wrap("34", t) }
func (s Style) Violet(t string) string { return s.wrap("35", t) }

// Dates formats moments in the user's time zone and calendar system (Jalali or Gregorian).
type Dates struct {
	Loc    *time.Location
	Jalali bool
	Now    time.Time
}

func NewDates(user *api.User) Dates {
	loc := user.Location()
	return Dates{Loc: loc, Jalali: user != nil && user.CalendarSystem == "jalali", Now: time.Now().In(loc)}
}

// dayNumber counts calendar days (not 24-hour spans, which DST changes stretch or shrink).
func dayNumber(t time.Time) int {
	y, m, d := t.Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

// DaysFromToday is 0 for today, 1 for tomorrow, -1 for yesterday, in the user's time zone.
func (d Dates) DaysFromToday(t time.Time) int {
	return dayNumber(t.In(d.Loc)) - dayNumber(d.Now.In(d.Loc))
}

// Date is "20 Aban" / "Nov 20", with the year when it isn't this year's.
func (d Dates) Date(t time.Time) string {
	t = t.In(d.Loc)
	if d.Jalali {
		jy, jm, jd := ToJalali(t.Year(), int(t.Month()), t.Day())
		ny, _, _ := ToJalali(d.Now.Year(), int(d.Now.Month()), d.Now.Day())
		out := fmt.Sprintf("%d %s", jd, jalaliMonths[jm-1])
		if jy != ny {
			out += fmt.Sprintf(" %d", jy)
		}
		return out
	}
	if t.Year() != d.Now.Year() {
		return t.Format("Jan 2, 2006")
	}
	return t.Format("Jan 2")
}

// Day is relative when close: "today", "tomorrow", "yesterday", a weekday within the next week, else Date.
func (d Dates) Day(t time.Time) string {
	switch n := d.DaysFromToday(t); {
	case n == 0:
		return "today"
	case n == 1:
		return "tomorrow"
	case n == -1:
		return "yesterday"
	case n > 1 && n < 7:
		return t.In(d.Loc).Format("Monday")
	default:
		return d.Date(t)
	}
}

// Long is "Wednesday, 20 Aban 1405" / "Wednesday, Nov 11, 2026".
func (d Dates) Long(t time.Time) string {
	t = t.In(d.Loc)
	if d.Jalali {
		jy, jm, jd := ToJalali(t.Year(), int(t.Month()), t.Day())
		return fmt.Sprintf("%s, %d %s %d", t.Format("Monday"), jd, jalaliMonths[jm-1], jy)
	}
	return t.Format("Monday, Jan 2, 2006")
}

// Due describes a deadline: "today 17:00", "tomorrow", "Friday 09:30", "12 Azar".
func (d Dates) Due(t api.Task) string {
	if t.DueAt == nil {
		return ""
	}
	out := d.Day(*t.DueAt)
	if t.DueHasTime {
		out += " " + t.DueAt.In(d.Loc).Format("15:04")
	}
	return out
}

// Overdue matches the web app: a timed deadline in the past, or a whole-day one before today.
func (d Dates) Overdue(t api.Task) bool {
	if t.DueAt == nil || t.Closed() {
		return false
	}
	if t.DueHasTime {
		return t.DueAt.Before(d.Now)
	}
	return d.DaysFromToday(*t.DueAt) < 0
}

// --- tasks -------------------------------------------------------------------------------------------------

var statusGlyph = map[string]string{"todo": "○", "in_progress": "◐", "blocked": "⊘", "done": "✓", "cancelled": "✗"}

// Context resolves ids to names for task lines.
type Context struct {
	Style    Style
	Dates    Dates
	Me       *api.User
	Projects map[int]api.Project
	// TimeOnly drops the day from deadlines, for lists already grouped under a day heading.
	TimeOnly bool
}

func (c Context) priority(p string) string {
	switch p {
	case "p0":
		return c.Style.Red("P0")
	case "p1":
		return c.Style.Yellow("P1")
	case "p2":
		return c.Style.Blue("P2")
	}
	return "  "
}

// TaskLine is one line: id, status, priority, title and dimmed details (due, project, assignee, progress).
func (c Context) TaskLine(t api.Task, idWidth int) string {
	glyph := statusGlyph[t.Status]
	if glyph == "" {
		glyph = "○"
	}
	if t.Status == "done" {
		glyph = c.Style.Green(glyph)
	}
	title := t.Title
	if t.Closed() {
		title = c.Style.Dim(title)
	}
	var meta []string
	due := c.Dates.Due(t)
	if c.TimeOnly {
		due = ""
		if t.DueAt != nil && t.DueHasTime {
			due = t.DueAt.In(c.Dates.Loc).Format("15:04")
		}
	}
	if due != "" {
		if c.Dates.Overdue(t) {
			meta = append(meta, c.Style.Red(due))
		} else {
			meta = append(meta, c.Style.Dim(due))
		}
	}
	if t.Recurrence != "" {
		meta = append(meta, c.Style.Dim("↻"))
	}
	if t.Project != nil {
		if p, ok := c.Projects[*t.Project]; ok {
			meta = append(meta, c.Style.Violet("#"+p.Name))
		}
	}
	if t.Assignee != nil && (c.Me == nil || t.Assignee.ID != c.Me.ID) {
		meta = append(meta, c.Style.Dim("@"+t.Assignee.Name))
	}
	if t.SubtaskCount > 0 {
		meta = append(meta, c.Style.Dim(fmt.Sprintf("%d/%d", t.SubtaskDone, t.SubtaskCount)))
	}
	line := fmt.Sprintf("%*d  %s %s  %s", idWidth, t.ID, glyph, c.priority(t.Priority), title)
	if len(meta) > 0 {
		line += "  " + strings.Join(meta, c.Style.Dim(" · "))
	}
	return line
}

// IDWidth is the column width for the widest id.
func IDWidth(tasks []api.Task) int {
	w := 1
	for _, t := range tasks {
		if n := len(fmt.Sprint(t.ID)); n > w {
			w = n
		}
	}
	return w
}

// Heading is a section title: "Overdue (2)".
func (c Context) Heading(title string, n int, danger bool) string {
	text := fmt.Sprintf("%s (%d)", title, n)
	if danger {
		return c.Style.Bold(c.Style.Red(text))
	}
	return c.Style.Bold(text)
}
