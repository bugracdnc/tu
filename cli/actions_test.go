package cli

import (
	"strings"
	"testing"
	"time"
	"tu/models"
)

// Monday, 12 October 2026 14:30
var testNow = time.Date(2026, 10, 12, 14, 30, 0, 0, time.Local)

func local(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.Local)
}

func TestSplitNameDate(t *testing.T) {
	tests := []struct {
		args []string
		name string
		want time.Time
	}{
		{[]string{"Summer holiday", "2027-07-01"}, "Summer holiday", local(2027, 7, 1, 0, 0)},
		{[]string{"Exam", "friday", "9am"}, "Exam", local(2026, 10, 16, 9, 0)},
		{[]string{"2026-05-22", "16:13:22"}, "", time.Date(2026, 5, 22, 16, 13, 22, 0, time.Local)},
		{[]string{"2026-05-22 16:13:22"}, "", time.Date(2026, 5, 22, 16, 13, 22, 0, time.Local)},
		{[]string{"Trip", "in", "3", "days"}, "Trip", local(2026, 10, 15, 0, 0)},
		{[]string{"Mom", "60", "22/05/2027"}, "Mom 60", local(2027, 5, 22, 0, 0)},
	}
	for _, tt := range tests {
		name, date, _, err := splitNameDate(tt.args, testNow)
		if err != nil {
			t.Errorf("%q: unexpected error %v", tt.args, err)
			continue
		}
		if name != tt.name || !date.Equal(tt.want) {
			t.Errorf("%q: got (%q, %v), want (%q, %v)", tt.args, name, date, tt.name, tt.want)
		}
	}

	if _, _, _, err := splitNameDate([]string{"just", "a", "name"}, testNow); err == nil {
		t.Error("wanted error when no date is given")
	}
}

func TestCalculateUntilDate(t *testing.T) {
	tests := []struct {
		d    models.TuDate
		want string
		past bool
	}{
		{models.TuDate{Date: local(2026, 10, 12, 0, 0), AllDay: true}, "  today", false},
		{models.TuDate{Date: local(2026, 10, 13, 0, 0), AllDay: true}, "  1 day", false},
		{models.TuDate{Date: local(2026, 10, 9, 0, 0), AllDay: true}, "  3 days", true},
		{models.TuDate{Date: local(2028, 1, 1, 0, 0), AllDay: true}, "  1 year", false},
		{models.TuDate{Date: local(2026, 10, 12, 17, 0)}, "  2 hours", false},
		{models.TuDate{Date: local(2026, 10, 12, 14, 45)}, " 15 minutes", false},
		{models.TuDate{Date: local(2026, 10, 2, 14, 30)}, " 10 days", true},
	}
	for _, tt := range tests {
		got, past := calculateUntilDate(tt.d, testNow)
		if got != tt.want || past != tt.past {
			t.Errorf("%v: got (%q, %t), want (%q, %t)", tt.d.Date, got, past, tt.want, tt.past)
		}
	}
}

func TestBreakdown(t *testing.T) {
	tests := []struct {
		to     time.Time
		allDay bool
		want   string
	}{
		{local(2027, 12, 15, 16, 45), false, "1 year, 2 months, 3 days, 2 hours, 15 minutes"},
		{local(2026, 10, 12, 14, 30), false, "less than a minute"},
		{local(2026, 10, 12, 0, 0), true, "today"},
		{local(2026, 11, 13, 0, 0), true, "1 month, 1 day"},
		{local(2026, 10, 9, 0, 0), true, "3 days"},
	}
	for _, tt := range tests {
		if got := breakdown(testNow, tt.to, tt.allDay); got != tt.want {
			t.Errorf("breakdown to %v: got %q, want %q", tt.to, got, tt.want)
		}
	}
}

func TestFilterDates(t *testing.T) {
	dates := []models.TuDate{
		{UUID: "later", Date: local(2027, 1, 1, 0, 0), AllDay: true},
		{UUID: "past", Date: local(2026, 1, 1, 0, 0), AllDay: true},
		{UUID: "soon", Date: local(2026, 10, 14, 0, 0), AllDay: true},
		{UUID: "birthday", Date: local(1995, 10, 20, 0, 0), AllDay: true, Repeat: models.RepeatYearly},
	}
	ids := func(ds []models.TuDate) string {
		var out []string
		for _, d := range ds {
			out = append(out, d.UUID)
		}
		return strings.Join(out, ",")
	}

	tests := []struct {
		name     string
		opts     listOptions
		showPast bool
		want     string
	}{
		{"default hides past", listOptions{}, false, "soon,birthday,later"},
		{"all", listOptions{all: true}, false, "past,soon,birthday,later"},
		{"config shows past", listOptions{}, true, "past,soon,birthday,later"},
		{"only past", listOptions{past: true}, false, "past"},
		{"within a week", listOptions{within: "1w"}, false, "soon"},
		{"within 2 weeks", listOptions{within: "2w"}, false, "soon,birthday"},
	}
	for _, tt := range tests {
		got, err := filterDates(dates, tt.opts, tt.showPast, testNow)
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if ids(got) != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, ids(got), tt.want)
		}
	}

	if _, err := filterDates(dates, listOptions{within: "soonish"}, false, testNow); err == nil {
		t.Error("wanted error for a bad --within span")
	}
}
