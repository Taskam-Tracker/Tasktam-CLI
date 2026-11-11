package render

import (
	"testing"
	"time"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/api"
)

func TestToJalali(t *testing.T) {
	cases := []struct{ g, j [3]int }{
		{[3]int{2026, 11, 11}, [3]int{1405, 8, 20}}, // the web app shows "20 Aban 1405"
		{[3]int{2025, 3, 21}, [3]int{1404, 1, 1}},   // Nowruz
		{[3]int{2024, 3, 20}, [3]int{1403, 1, 1}},
		{[3]int{2024, 3, 19}, [3]int{1402, 12, 29}},
		{[3]int{2025, 3, 20}, [3]int{1403, 12, 30}}, // 1403 is a leap year
		{[3]int{2000, 1, 1}, [3]int{1378, 10, 11}},
	}
	for _, c := range cases {
		y, m, d := ToJalali(c.g[0], c.g[1], c.g[2])
		if [3]int{y, m, d} != c.j {
			t.Errorf("ToJalali(%v) = %d-%d-%d, want %v", c.g, y, m, d, c.j)
		}
	}
}

func TestDaysFromTodayAcrossDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata")
	}
	// Clocks spring forward on 2026-03-08: that day has 23 hours.
	d := Dates{Loc: ny, Now: time.Date(2026, 3, 7, 23, 30, 0, 0, ny)}
	if got := d.DaysFromToday(time.Date(2026, 3, 8, 23, 0, 0, 0, ny)); got != 1 {
		t.Errorf("tomorrow across DST = %d, want 1", got)
	}
	if got := d.DaysFromToday(time.Date(2026, 3, 9, 0, 30, 0, 0, ny)); got != 2 {
		t.Errorf("two days across DST = %d, want 2", got)
	}
}

func TestDayLabelsAndJalaliDates(t *testing.T) {
	tehran, _ := time.LoadLocation("Asia/Tehran")
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, tehran) // a Wednesday
	d := Dates{Loc: tehran, Now: now}
	for offset, want := range map[int]string{0: "today", 1: "tomorrow", -1: "yesterday", 2: "Friday"} {
		if got := d.Day(now.AddDate(0, 0, offset)); got != want {
			t.Errorf("Day(+%d) = %q, want %q", offset, got, want)
		}
	}
	if got := d.Day(now.AddDate(0, 0, 9)); got != "Nov 20" {
		t.Errorf("Gregorian date = %q", got)
	}
	d.Jalali = true
	if got := d.Day(now.AddDate(0, 0, 9)); got != "29 Aban" {
		t.Errorf("Jalali date = %q", got)
	}
	if got := d.Long(now); got != "Wednesday, 20 Aban 1405" {
		t.Errorf("Jalali long = %q", got)
	}
}

func TestOverdue(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	d := Dates{Loc: time.UTC, Now: now}
	at := func(t time.Time) *time.Time { return &t }
	cases := []struct {
		task api.Task
		want bool
	}{
		{api.Task{DueAt: at(now.Add(-time.Hour)), DueHasTime: true, Status: "todo"}, true},
		{api.Task{DueAt: at(now.Add(time.Hour)), DueHasTime: true, Status: "todo"}, false},
		{api.Task{DueAt: at(time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)), Status: "todo"}, false}, // whole day today
		{api.Task{DueAt: at(now.AddDate(0, 0, -1)), Status: "todo"}, true},
		{api.Task{DueAt: at(now.AddDate(0, 0, -1)), Status: "done"}, false},
		{api.Task{Status: "todo"}, false},
	}
	for i, c := range cases {
		if got := d.Overdue(c.task); got != c.want {
			t.Errorf("case %d: Overdue = %v, want %v", i, got, c.want)
		}
	}
}

func TestTaskLineWithoutColour(t *testing.T) {
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	due := now.Add(8 * time.Hour)
	project := 3
	c := Context{
		Dates:    Dates{Loc: time.UTC, Now: now},
		Me:       &api.User{ID: 1},
		Projects: map[int]api.Project{3: {ID: 3, Name: "Work"}},
	}
	task := api.Task{ID: 42, Title: "Ship it", Status: "todo", Priority: "p1", DueAt: &due, DueHasTime: true,
		Project: &project, Assignee: &api.Person{ID: 2, Name: "Sara"}, SubtaskCount: 3, SubtaskDone: 1}
	want := "42  ○ P1  Ship it  today 17:00 · #Work · @Sara · 1/3"
	if got := c.TaskLine(task, 2); got != want {
		t.Errorf("TaskLine =\n  %q\nwant\n  %q", got, want)
	}
	c.TimeOnly = true
	if got := c.TaskLine(task, 2); got != "42  ○ P1  Ship it  17:00 · #Work · @Sara · 1/3" {
		t.Errorf("TimeOnly TaskLine = %q", got)
	}
}
