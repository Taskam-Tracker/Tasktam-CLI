package cmd

import (
	"testing"
	"time"
)

func TestParseDue(t *testing.T) {
	tehran, _ := time.LoadLocation("Asia/Tehran")
	now := time.Date(2026, 11, 11, 9, 30, 0, 0, tehran) // Wednesday
	day := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, tehran) }
	cases := []struct {
		in       string
		want     time.Time
		hasTime  bool
		cleared  bool
		wantsErr bool
	}{
		{in: "today", want: day(2026, 11, 11, 23, 59)},
		{in: "Tomorrow", want: day(2026, 11, 12, 23, 59)},
		{in: "+3d", want: day(2026, 11, 14, 23, 59)},
		{in: "fri", want: day(2026, 11, 13, 23, 59)},
		{in: "wednesday", want: day(2026, 11, 18, 23, 59)}, // today is Wednesday: next week's
		{in: "2026-12-01", want: day(2026, 12, 1, 23, 59)},
		{in: "2026-12-01 17:00", want: day(2026, 12, 1, 17, 0), hasTime: true},
		{in: "17:45", want: day(2026, 11, 11, 17, 45), hasTime: true},
		{in: "none", cleared: true},
		{in: "next blursday", wantsErr: true},
		{in: "+xd", wantsErr: true},
		{in: "25:99", wantsErr: true},
	}
	for _, c := range cases {
		got, hasTime, err := ParseDue(c.in, now)
		switch {
		case c.wantsErr:
			if err == nil {
				t.Errorf("%q: expected an error", c.in)
			}
		case err != nil:
			t.Errorf("%q: %v", c.in, err)
		case c.cleared:
			if got != nil {
				t.Errorf("%q: expected no deadline, got %v", c.in, got)
			}
		case got == nil || !got.Equal(c.want) || hasTime != c.hasTime:
			t.Errorf("%q = %v (time %v), want %v (time %v)", c.in, got, hasTime, c.want, c.hasTime)
		}
	}
}
