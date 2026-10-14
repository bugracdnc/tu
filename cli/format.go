package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
	"tu/models"
)

const (
	Day  = 24 * time.Hour
	Year = 365 * Day
)

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorGray   = "\033[0;90m"
	colorWhite  = "\033[0;37m"
	colorYellow = "\033[0;33m"
	colorCyan   = "\033[0;36m"
)

var useColor = true

// setupColor disables colors when asked to, when NO_COLOR is set, or when
// stdout is not a terminal (piped into another program or a file).
func setupColor(disabled bool) {
	if disabled || os.Getenv("NO_COLOR") != "" {
		useColor = false
		return
	}
	if fi, err := os.Stdout.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		useColor = false
	}
}

func paint(color, s string) string {
	if !useColor {
		return s
	}
	return color + s + colorReset
}

// Helper function for quick formatting and pluralization
func formatUnit(val int64, unit string) string {
	if val == 1 {
		return fmt.Sprintf("  1 %s", unit) // Singular
	}
	return fmt.Sprintf("%3d %ss", val, unit) // Plural
}

// calculateUntilDate returns the distance to the date's relevant occurrence,
// showing only the largest unit, and whether that occurrence has passed.
func calculateUntilDate(tuDate models.TuDate, now time.Time) (string, bool) {
	occ := tuDate.Occurrence(now)

	// All-day dates count in calendar days, so tomorrow is always "1 day".
	if tuDate.AllDay {
		days := calendarDays(now, occ)
		if days == 0 {
			return "  today", false
		}
		past := days < 0
		if past {
			days = -days
		}
		if years := days / 365; years > 0 {
			return formatUnit(years, "year"), past
		}
		return formatUnit(days, "day"), past
	}

	d := occ.Sub(now)
	past := d <= 0
	if past {
		d = -d
	}

	// Check from largest to smallest.
	// The first one > 0 is our biggest operand, so we return immediately.
	if years := d / Year; years > 0 {
		return formatUnit(int64(years), "year"), past
	}
	if days := d / Day; days > 0 {
		return formatUnit(int64(days), "day"), past
	}
	if hours := d / time.Hour; hours > 0 {
		return formatUnit(int64(hours), "hour"), past
	}
	if minutes := d / time.Minute; minutes > 0 {
		return formatUnit(int64(minutes), "minute"), past
	}

	// If we get here, it's less than a minute away
	return formatUnit(int64(d/time.Second), "second"), past
}

// calendarDays counts the day boundaries between a and b, ignoring DST shifts.
func calendarDays(a, b time.Time) int64 {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	ua := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	ub := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	return int64(ub.Sub(ua) / Day)
}

// breakdown spells out the full distance between two moments, e.g.
// "1 year, 2 months, 3 days, 4 hours".
func breakdown(from, to time.Time, allDay bool) string {
	if to.Before(from) {
		from, to = to, from
	}
	if allDay {
		from, to = models.StartOfDay(from), models.StartOfDay(to)
	}

	years := 0
	for !from.AddDate(years+1, 0, 0).After(to) {
		years++
	}
	from = from.AddDate(years, 0, 0)

	months := 0
	for !from.AddDate(0, months+1, 0).After(to) {
		months++
	}
	from = from.AddDate(0, months, 0)

	days := int(calendarDays(from, to))
	if from.AddDate(0, 0, days).After(to) {
		days--
	}
	rest := to.Sub(from.AddDate(0, 0, days))

	parts := []string{}
	add := func(n int, unit string) {
		if n == 1 {
			parts = append(parts, "1 "+unit)
		} else if n > 1 {
			parts = append(parts, fmt.Sprintf("%d %ss", n, unit))
		}
	}
	add(years, "year")
	add(months, "month")
	add(days, "day")
	if !allDay {
		add(int(rest/time.Hour), "hour")
		add(int(rest%time.Hour/time.Minute), "minute")
	}

	if len(parts) == 0 {
		if allDay {
			return "today"
		}
		return "less than a minute"
	}
	return strings.Join(parts, ", ")
}

func formatDateString(date time.Time, allDay bool) string {
	if allDay {
		return date.Format("02/01/2006")
	}
	return date.Format("02/01/2006 15:04")
}

func formatDateLong(date time.Time, allDay bool) string {
	if allDay {
		return date.Format("Monday, 02 January 2006") + " (all day)"
	}
	return date.Format("Monday, 02 January 2006 15:04")
}

// formatLine renders a date as a single list entry.
func formatLine(tuDate models.TuDate, now time.Time) string {
	untilStr, past := calculateUntilDate(tuDate, now)
	occ := tuDate.Occurrence(now)
	id := paint(colorGray, tuDate.ShortID())

	if past {
		return fmt.Sprintf("* %s %s %s %s [%s] %s",
			id,
			paint(colorGray, "[Passed]"),
			paint(colorWhite, untilStr),
			paint(colorGray, "for"),
			paint(colorWhite, tuDate.DisplayName()),
			paint(colorGray, "(since "+formatDateString(occ, tuDate.AllDay)+")"),
		)
	}

	untilColor := colorWhite
	if occ.Sub(now) < Day {
		untilColor = colorYellow + colorBold
	}
	to := "to"
	if strings.TrimSpace(untilStr) == "today" {
		to = "is"
	}
	line := fmt.Sprintf("* %s %s %s [%s] %s",
		id,
		paint(untilColor, untilStr),
		paint(colorGray, to),
		paint(colorWhite, tuDate.DisplayName()),
		paint(colorGray, "(at "+formatDateString(occ, tuDate.AllDay)+")"),
	)
	if tuDate.Repeat != models.RepeatNone {
		line += " " + paint(colorCyan, "↻ "+string(tuDate.Repeat))
	}
	if tuDate.Pulled() {
		line += " " + paint(colorGray, "· "+tuDate.Google.CalendarName)
	}
	return line
}
