package models

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Layouts carrying their own time zone (e.g. the output of time.Time.String).
var zonedLayouts = []string{
	"2006-01-02 15:04:05.999999999 -0700 MST",
	time.RFC3339,
}

var timedLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"02/01/2006 15:04:05",
	"02/01/2006 15:04",
	"02.01.2006 15:04:05",
	"02.01.2006 15:04",
}

var dateLayouts = []string{
	"2006-01-02",
	"02/01/2006",
	"02.01.2006",
}

var (
	clockRe    = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?$`)
	clockAmPm  = regexp.MustCompile(`^(\d{1,2})(?::(\d{2}))?\s*(am|pm)$`)
	offsetPart = regexp.MustCompile(`(\d+)\s*([a-z]+)`)
)

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

// ParseDate parses an absolute or relative date expression relative to `now`.
// It reports whether the result refers to a whole day rather than a moment.
//
// Supported forms include:
//
//	2026-05-22, 22/05/2026, 22.05.2026        (all day)
//	2026-05-22 16:00, 22/05/2026 16:00:30      (exact time)
//	today, tomorrow, yesterday, friday, next fri (all day)
//	tomorrow 18:00, fri 9am                    (exact time)
//	18:00                                      (next time the clock shows 18:00)
//	+3d, in 2 weeks, 1y 2mo, 3 days ago        (relative)
func ParseDate(input string, now time.Time) (time.Time, bool, error) {
	s := strings.Join(strings.Fields(input), " ")
	if s == "" {
		return time.Time{}, false, errors.New("no date given")
	}

	for _, layout := range zonedLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.In(time.Local), false, nil
		}
	}
	for _, layout := range timedLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, false, nil
		}
	}
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true, nil
		}
	}

	lower := strings.ToLower(s)

	// Split off a trailing clock time such as "18:00" or "9am".
	dayPart, clock, hasClock := lower, time.Duration(0), false
	if i := strings.LastIndex(lower, " "); i >= 0 {
		if c, ok := parseClock(lower[i+1:]); ok {
			dayPart, clock, hasClock = lower[:i], c, true
		}
	}
	if !hasClock {
		if c, ok := parseClock(lower); ok {
			dayPart, clock, hasClock = "", c, true
		}
	}

	// Set the wall clock rather than adding a duration, which would be off by
	// an hour on days when daylight saving time changes.
	at := func(day time.Time) time.Time {
		y, m, d := day.Date()
		return time.Date(y, m, d, int(clock/time.Hour), int(clock%time.Hour/time.Minute), int(clock%time.Minute/time.Second), 0, day.Location())
	}

	if dayPart == "" {
		t := at(now)
		if !t.After(now) {
			t = at(now.AddDate(0, 0, 1))
		}
		return t, false, nil
	}

	if day, ok := parseDayWord(dayPart, now); ok {
		if hasClock {
			return at(day), false, nil
		}
		return StartOfDay(day), true, nil
	}

	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, dayPart, time.Local); err == nil && hasClock {
			return at(t), false, nil
		}
	}

	if off, err := ParseOffset(dayPart); err == nil {
		if hasClock {
			return at(off.AddTo(now)), false, nil
		}
		if off.Clock == 0 {
			return StartOfDay(off.AddTo(now)), true, nil
		}
		return off.AddTo(now), false, nil
	}

	return time.Time{}, false, fmt.Errorf("cannot understand date %q", input)
}

func parseClock(s string) (time.Duration, bool) {
	var h, m, sec int
	if g := clockRe.FindStringSubmatch(s); g != nil {
		h, _ = strconv.Atoi(g[1])
		m, _ = strconv.Atoi(g[2])
		if g[3] != "" {
			sec, _ = strconv.Atoi(g[3])
		}
	} else if g := clockAmPm.FindStringSubmatch(s); g != nil {
		h, _ = strconv.Atoi(g[1])
		if g[2] != "" {
			m, _ = strconv.Atoi(g[2])
		}
		if h < 1 || h > 12 {
			return 0, false
		}
		h %= 12
		if g[3] == "pm" {
			h += 12
		}
	} else {
		return 0, false
	}
	if h > 23 || m > 59 || sec > 59 {
		return 0, false
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, true
}

func parseDayWord(s string, now time.Time) (time.Time, bool) {
	switch s {
	case "today", "now":
		return now, true
	case "tomorrow", "tmrw":
		return now.AddDate(0, 0, 1), true
	case "yesterday":
		return now.AddDate(0, 0, -1), true
	}

	s = strings.TrimPrefix(s, "next ")
	if wd, ok := weekdays[s]; ok {
		// Always the upcoming one; "friday" said on a Friday means next week.
		diff := (int(wd)-int(now.Weekday())+6)%7 + 1
		return now.AddDate(0, 0, diff), true
	}
	return time.Time{}, false
}

// Offset is a calendar-aware span such as "1y 2mo 3d 4h".
type Offset struct {
	Years, Months, Days int
	Clock               time.Duration
}

// AddTo applies the offset to t.
func (o Offset) AddTo(t time.Time) time.Time {
	return t.AddDate(o.Years, o.Months, o.Days).Add(o.Clock)
}

// ParseOffset parses spans like "3d", "+2w", "in 1 month", "1h30m" or
// "3 days ago". A leading "-" or trailing "ago" makes the offset negative.
func ParseOffset(input string) (Offset, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	sign := 1
	switch {
	case strings.HasPrefix(s, "in "):
		s = s[3:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	case strings.HasPrefix(s, "-"):
		s, sign = s[1:], -1
	}
	if rest, ok := strings.CutSuffix(s, " ago"); ok {
		s, sign = rest, -sign
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, ",", " "), " and ", " ")

	var off Offset
	matches := offsetPart.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return off, fmt.Errorf("cannot understand span %q", input)
	}

	// Only whitespace may sit between parts, so "3d foo" fails.
	pos := 0
	for _, m := range matches {
		if strings.TrimSpace(s[pos:m[0]]) != "" {
			return off, fmt.Errorf("cannot understand span %q", input)
		}
		pos = m[1]

		n, err := strconv.Atoi(s[m[2]:m[3]])
		if err != nil {
			return off, err
		}
		n *= sign
		switch s[m[4]:m[5]] {
		case "s", "sec", "secs", "second", "seconds":
			off.Clock += time.Duration(n) * time.Second
		case "m", "min", "mins", "minute", "minutes":
			off.Clock += time.Duration(n) * time.Minute
		case "h", "hr", "hrs", "hour", "hours":
			off.Clock += time.Duration(n) * time.Hour
		case "d", "day", "days":
			off.Days += n
		case "w", "wk", "wks", "week", "weeks":
			off.Days += 7 * n
		case "mo", "mon", "mos", "month", "months":
			off.Months += n
		case "y", "yr", "yrs", "year", "years":
			off.Years += n
		default:
			return off, fmt.Errorf("unknown unit %q in %q", s[m[4]:m[5]], input)
		}
	}
	if strings.TrimSpace(s[pos:]) != "" {
		return off, fmt.Errorf("cannot understand span %q", input)
	}
	return off, nil
}
