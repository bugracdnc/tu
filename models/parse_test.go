package models

import (
	"testing"
	"time"
)

// Monday, 12 October 2026 14:30
var parseNow = time.Date(2026, 10, 12, 14, 30, 0, 0, time.Local)

func local(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.Local)
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		in     string
		want   time.Time
		allDay bool
	}{
		{"2026-05-22", local(2026, 5, 22, 0, 0), true},
		{"22/05/2026", local(2026, 5, 22, 0, 0), true},
		{"22.05.2026", local(2026, 5, 22, 0, 0), true},
		{"2026-05-22 16:13", local(2026, 5, 22, 16, 13), false},
		{"2026-05-22  16:13:00", local(2026, 5, 22, 16, 13), false},
		{"22/05/2026 16:13", local(2026, 5, 22, 16, 13), false},
		{"today", local(2026, 10, 12, 0, 0), true},
		{"Tomorrow", local(2026, 10, 13, 0, 0), true},
		{"tomorrow 18:00", local(2026, 10, 13, 18, 0), false},
		{"friday", local(2026, 10, 16, 0, 0), true},
		{"next mon", local(2026, 10, 19, 0, 0), true},
		{"fri 9am", local(2026, 10, 16, 9, 0), false},
		{"fri 12pm", local(2026, 10, 16, 12, 0), false},
		{"18:00", local(2026, 10, 12, 18, 0), false},
		{"09:00", local(2026, 10, 13, 9, 0), false},
		{"+3d", local(2026, 10, 15, 0, 0), true},
		{"in 2 weeks", local(2026, 10, 26, 0, 0), true},
		{"1y 2mo", local(2027, 12, 12, 0, 0), true},
		{"3 days ago", local(2026, 10, 9, 0, 0), true},
		{"in 2 hours", local(2026, 10, 12, 16, 30), false},
		{"1h30m", local(2026, 10, 12, 16, 0), false},
		{"+1w 10:00", local(2026, 10, 19, 10, 0), false},
		{"2026-11-01 9pm", local(2026, 11, 1, 21, 0), false},
	}
	for _, tt := range tests {
		got, allDay, err := ParseDate(tt.in, parseNow)
		if err != nil {
			t.Errorf("ParseDate(%q): unexpected error %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) || allDay != tt.allDay {
			t.Errorf("ParseDate(%q) = %v (all day %t), want %v (all day %t)", tt.in, got, allDay, tt.want, tt.allDay)
		}
	}
}

func TestParseDate_Bad(t *testing.T) {
	for _, in := range []string{"", "2026-4-3", "2026-5-22 4:0", "2026-05-22 25:43:35", "exam friday", "3dx", "in 3 days foo", "13pm", "2026-13-01"} {
		if got, _, err := ParseDate(in, parseNow); err == nil {
			t.Errorf("ParseDate(%q) = %v, wanted error", in, got)
		}
	}
}

func TestParseRepeat(t *testing.T) {
	if r, err := ParseRepeat("Yearly"); err != nil || r != RepeatYearly {
		t.Errorf("got %q %v, want yearly", r, err)
	}
	if r, err := ParseRepeat("none"); err != nil || r != RepeatNone {
		t.Errorf("got %q %v, want none", r, err)
	}
	if _, err := ParseRepeat("hourly"); err == nil {
		t.Error("wanted error for hourly")
	}
}

func TestOccurrence(t *testing.T) {
	tests := []struct {
		name string
		d    TuDate
		want time.Time
	}{
		{"one-off in the past stays", TuDate{Date: local(2020, 1, 1, 0, 0)}, local(2020, 1, 1, 0, 0)},
		{"yearly birthday", TuDate{Date: local(1995, 5, 22, 0, 0), AllDay: true, Repeat: RepeatYearly}, local(2027, 5, 22, 0, 0)},
		{"yearly all day today stays today", TuDate{Date: local(1995, 10, 12, 0, 0), AllDay: true, Repeat: RepeatYearly}, local(2026, 10, 12, 0, 0)},
		{"yearly leap day clamps", TuDate{Date: local(2024, 2, 29, 0, 0), AllDay: true, Repeat: RepeatYearly}, local(2027, 2, 28, 0, 0)},
		{"monthly on the 31st clamps", TuDate{Date: local(2026, 1, 31, 0, 0), AllDay: true, Repeat: RepeatMonthly}, local(2026, 10, 31, 0, 0)},
		{"weekly", TuDate{Date: local(2026, 9, 1, 10, 0), Repeat: RepeatWeekly}, local(2026, 10, 13, 10, 0)},
		{"daily passed today", TuDate{Date: local(2026, 1, 1, 9, 30), Repeat: RepeatDaily}, local(2026, 10, 13, 9, 30)},
		{"daily later today", TuDate{Date: local(2026, 1, 1, 18, 0), Repeat: RepeatDaily}, local(2026, 10, 12, 18, 0)},
		{"future recurring stays", TuDate{Date: local(2030, 1, 1, 0, 0), Repeat: RepeatYearly}, local(2030, 1, 1, 0, 0)},
	}
	for _, tt := range tests {
		if got := tt.d.Occurrence(parseNow); !got.Equal(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsPast(t *testing.T) {
	tests := []struct {
		name string
		d    TuDate
		want bool
	}{
		{"all day today", TuDate{Date: local(2026, 10, 12, 0, 0), AllDay: true}, false},
		{"all day yesterday", TuDate{Date: local(2026, 10, 11, 0, 0), AllDay: true}, true},
		{"timed earlier today", TuDate{Date: local(2026, 10, 12, 9, 0)}, true},
		{"timed later today", TuDate{Date: local(2026, 10, 12, 18, 0)}, false},
		{"recurring never passes", TuDate{Date: local(2000, 1, 1, 0, 0), Repeat: RepeatYearly}, false},
	}
	for _, tt := range tests {
		if got := tt.d.IsPast(parseNow); got != tt.want {
			t.Errorf("%s: got %t, want %t", tt.name, got, tt.want)
		}
	}
}
