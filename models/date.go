package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Repeat describes how often a tracked date recurs.
type Repeat string

const (
	RepeatNone    Repeat = ""
	RepeatDaily   Repeat = "daily"
	RepeatWeekly  Repeat = "weekly"
	RepeatMonthly Repeat = "monthly"
	RepeatYearly  Repeat = "yearly"
)

var Repeats = []Repeat{RepeatDaily, RepeatWeekly, RepeatMonthly, RepeatYearly}

func ParseRepeat(s string) (Repeat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none", "never", "no":
		return RepeatNone, nil
	case "d", "day", "daily":
		return RepeatDaily, nil
	case "w", "week", "weekly":
		return RepeatWeekly, nil
	case "m", "month", "monthly":
		return RepeatMonthly, nil
	case "y", "year", "yearly", "annually":
		return RepeatYearly, nil
	}
	return RepeatNone, fmt.Errorf("unknown repeat %q (use daily, weekly, monthly, yearly or none)", s)
}

type TuDate struct {
	UUID   string    `json:"id"`
	Name   string    `json:"name"`
	Date   time.Time `json:"date"`
	AllDay bool      `json:"all_day,omitempty"`
	Repeat Repeat    `json:"repeat,omitempty"`

	// LocalOnly dates are never pushed to Google Calendar.
	LocalOnly bool       `json:"local_only,omitempty"`
	Google    GoogleLink `json:"google,omitzero"`
}

// GoogleLink ties a date to an event in Google Calendar. Dates created in tu
// are pushed and tu stays in charge of them; pulled dates mirror an event that
// lives in Google and are replaced on every sync.
type GoogleLink struct {
	ID           string `json:"id"`
	Calendar     string `json:"calendar"`
	CalendarName string `json:"calendar_name,omitempty"`
	// Series is the recurring event a pulled occurrence belongs to.
	Series string `json:"series,omitempty"`
	Pulled bool   `json:"pulled,omitempty"`
	// Hash of the content last sent to or received from Google, used to
	// notice which side changed since.
	Hash string `json:"hash,omitempty"`
}

func (t TuDate) Linked() bool { return t.Google.ID != "" }
func (t TuDate) Pulled() bool { return t.Google.Pulled }

// ContentHash summarises the fields that are kept in sync with Google.
func (t TuDate) ContentHash() string {
	date := t.Date.UTC().Format(time.RFC3339)
	if t.AllDay {
		date = t.Date.Format(time.DateOnly)
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{t.Name, date, fmt.Sprint(t.AllDay), string(t.Repeat)}, "\x00")))
	return hex.EncodeToString(sum[:8])
}

// ShortID is the prefix of the UUID shown to users for referencing a date.
func (t TuDate) ShortID() string {
	if len(t.UUID) < 6 {
		return t.UUID
	}
	return t.UUID[:6]
}

// DisplayName returns the name, or a placeholder for unnamed dates.
func (t TuDate) DisplayName() string {
	if t.Name == "" {
		return "unnamed"
	}
	return t.Name
}

// Occurrence returns the date that is relevant at `now`. For one-off dates
// this is the stored date. For recurring dates it's the next occurrence that
// hasn't passed yet (for all-day dates, today still counts as not passed).
func (t TuDate) Occurrence(now time.Time) time.Time {
	if t.Repeat == RepeatNone {
		return t.Date
	}

	cutoff := now
	if t.AllDay {
		cutoff = StartOfDay(now)
	}
	if !t.Date.Before(cutoff) {
		return t.Date
	}

	// Always step from the original date so that, e.g., a monthly date on the
	// 31st clamps to the 30th in short months without drifting afterwards.
	for n := 1; ; n++ {
		var next time.Time
		switch t.Repeat {
		case RepeatDaily:
			next = t.Date.AddDate(0, 0, n)
		case RepeatWeekly:
			next = t.Date.AddDate(0, 0, 7*n)
		case RepeatMonthly:
			next = addMonthsClamped(t.Date, n)
		case RepeatYearly:
			next = addMonthsClamped(t.Date, 12*n)
		default:
			return t.Date
		}
		if !next.Before(cutoff) {
			return next
		}
	}
}

// IsPast reports whether the date has passed at `now`. Recurring dates never
// pass. All-day dates are only past once their day is over.
func (t TuDate) IsPast(now time.Time) bool {
	if t.Repeat != RepeatNone {
		return false
	}
	if t.AllDay {
		return t.Date.Before(StartOfDay(now))
	}
	return !t.Date.After(now)
}

func StartOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(months), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(d, last)-1)
}

func NewTuDateFromString(dateStr string) (*TuDate, error) {
	parsed, allDay, err := ParseDate(dateStr, time.Now())
	if err != nil {
		return nil, err
	}

	tmp := NewTuDateFromDate(parsed)
	tmp.AllDay = allDay
	return tmp, nil
}

func NewTuDateFromDate(date time.Time) *TuDate {
	var tmp TuDate
	tmp.UUID = uuid.NewString()
	tmp.Date = date

	return &tmp
}

func NewTuDateFromStringWithName(dateStr string, name string) (*TuDate, error) {
	if tmp, err := NewTuDateFromString(dateStr); err == nil {
		tmp.Name = name
		return tmp, nil
	} else {
		return nil, err
	}

}

func NewTuDateFromDateWithName(date time.Time, name string) *TuDate {
	tmp := NewTuDateFromDate(date)
	tmp.Name = name

	return tmp
}
