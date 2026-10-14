package gcal

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"tu/models"

	"github.com/google/uuid"
)

type Options struct {
	// Target is the calendar dates created in tu are pushed to.
	Target string
	// Calendars to pull from, by ID or name. Empty means every calendar that
	// is shown in Google Calendar.
	Calendars []string
	// Count caps how many upcoming events are pulled, Days how far ahead.
	Count int
	Days  int
	// PushOnly sends local changes without looking at what's in Google.
	PushOnly bool
}

type Report struct {
	Created, Updated, Linked, Changed, Deleted, Pulled int
}

// Event types that aren't events worth counting down to.
var skippedTypes = []string{"workingLocation", "focusTime", "outOfOffice"}

// tuKey is the private extended property holding the ID of the tu date an
// event was created from.
const tuKey = "tu"

type remoteEvent struct {
	Event
	cal    Calendar
	date   time.Time
	allDay bool
}

// Sync pushes local changes to Google and replaces the pulled dates with the
// upcoming events from Google. The returned dates are valid even when an error
// is returned, as some changes may have been made already, and should be
// saved together with the state.
func (c *Client) Sync(ctx context.Context, dates []models.TuDate, opts Options, now time.Time) ([]models.TuDate, Report, error) {
	dates = slices.Clone(dates)
	var rep Report

	// Deletions go first, so removed events can't be pulled back in.
	for len(c.state.PendingDeletes) > 0 {
		del := c.state.PendingDeletes[0]
		if err := c.Delete(ctx, del.Calendar, del.ID); err != nil {
			return dates, rep, err
		}
		c.state.PendingDeletes = c.state.PendingDeletes[1:]
		rep.Deleted++
	}

	cals, err := c.Calendars(ctx)
	if err != nil {
		return dates, rep, err
	}
	for _, cal := range cals {
		if cal.Primary {
			c.state.Account = cal.ID
		}
	}
	target, err := findCalendar(cals, opts.Target)
	if err != nil {
		return dates, rep, err
	}
	if !target.Writable() {
		return dates, rep, fmt.Errorf("calendar %q is read-only, pick another with: tu config google-calendar <id>", target.Name())
	}

	var pull []Calendar
	var remote []remoteEvent
	if !opts.PushOnly {
		if pull, err = pullCalendars(cals, opts.Calendars); err != nil {
			return dates, rep, err
		}
		fetch := pull
		if !slices.ContainsFunc(fetch, func(c Calendar) bool { return c.ID == target.ID }) {
			fetch = append(slices.Clone(fetch), target)
		}
		until := now.AddDate(0, 0, opts.Days)
		for _, cal := range fetch {
			events, err := c.Events(ctx, cal.ID, now, until, 250)
			if err != nil {
				return dates, rep, fmt.Errorf("reading %s: %w", cal.Name(), err)
			}
			for _, ev := range events {
				date, allDay, ok := eventStart(ev, now.Location())
				if ok && ev.Status != "cancelled" {
					remote = append(remote, remoteEvent{ev, cal, date, allDay})
				}
			}
		}
		slices.SortStableFunc(remote, func(a, b remoteEvent) int { return a.date.Compare(b.date) })
	}

	timeZone := func(calendar string) string {
		for _, cal := range cals {
			if cal.ID == calendar {
				return cal.TimeZone
			}
		}
		return target.TimeZone
	}

	linked := map[string]bool{}
	for _, d := range dates {
		if d.Linked() && !d.Pulled() {
			linked[d.Google.ID] = true
		}
	}

	for i := range dates {
		d := &dates[i]
		if d.LocalOnly || d.Pulled() {
			continue
		}

		if !d.Linked() {
			if d.IsPast(now) {
				continue
			}
			if m := findMatch(*d, remote, linked, now); m != nil {
				// Already in Google. Take it over when it's an event tu can
				// manage, otherwise leave it alone instead of duplicating it.
				id := m.ID
				if m.RecurringEventID != "" {
					id = m.RecurringEventID
				}
				fromTu := m.tuID() == d.UUID
				sameKind := (d.Repeat != models.RepeatNone) == (m.RecurringEventID != "")
				if m.cal.Writable() && (fromTu || (m.cal.ID == target.ID && sameKind)) {
					d.Google = models.GoogleLink{ID: id, Calendar: m.cal.ID, Hash: d.ContentHash()}
					linked[id] = true
					rep.Linked++
				}
				continue
			}
			ev, err := c.Insert(ctx, target.ID, eventBody(*d, target.TimeZone, true))
			if err != nil {
				return dates, rep, err
			}
			d.Google = models.GoogleLink{ID: ev.ID, Calendar: target.ID, Hash: d.ContentHash()}
			linked[ev.ID] = true
			rep.Created++
			continue
		}

		hash := d.ContentHash()
		if hash != d.Google.Hash {
			err := c.Patch(ctx, d.Google.Calendar, d.Google.ID, eventBody(*d, timeZone(d.Google.Calendar), false))
			if gone(err) {
				// Deleted in Google, but changed here since: bring it back.
				var ev Event
				if ev, err = c.Insert(ctx, d.Google.Calendar, eventBody(*d, timeZone(d.Google.Calendar), true)); err == nil {
					d.Google.ID = ev.ID
					linked[ev.ID] = true
				}
			}
			if err != nil {
				return dates, rep, err
			}
			d.Google.Hash = hash
			rep.Updated++
			continue
		}

		// Unchanged here, so take over changes made in Google. Only for one-off
		// events, recurring ones are only seen as single occurrences.
		if d.Repeat == models.RepeatNone {
			i := slices.IndexFunc(remote, func(r remoteEvent) bool { return r.ID == d.Google.ID })
			if i >= 0 {
				r := remote[i]
				theirs := models.TuDate{Name: strings.TrimSpace(r.Summary), Date: r.date, AllDay: r.allDay}
				if theirs.ContentHash() != hash {
					d.Name, d.Date, d.AllDay = theirs.Name, theirs.Date, theirs.AllDay
					d.Google.Hash = d.ContentHash()
					rep.Changed++
				}
			}
		}
	}

	if opts.PushOnly {
		return dates, rep, nil
	}

	// Pulling: every sync replaces the pulled dates with what's upcoming now.
	oldIDs := map[string]string{}
	var out []models.TuDate
	for _, d := range dates {
		if d.Pulled() {
			oldIDs[d.Google.Calendar+"/"+d.Google.ID] = d.UUID
		} else {
			out = append(out, d)
		}
	}
	for _, r := range remote {
		if rep.Pulled >= opts.Count {
			break
		}
		if !slices.ContainsFunc(pull, func(c Calendar) bool { return c.ID == r.cal.ID }) ||
			linked[r.ID] || linked[r.RecurringEventID] ||
			slices.Contains(c.state.Ignored, r.ID) || slices.Contains(c.state.Ignored, r.RecurringEventID) ||
			slices.Contains(skippedTypes, r.EventType) {
			continue
		}
		d := models.TuDate{Name: strings.TrimSpace(r.Summary), Date: r.date, AllDay: r.allDay}
		if d.IsPast(now) || slices.ContainsFunc(out, func(o models.TuDate) bool { return sameDate(o, d, now) }) {
			continue
		}
		d.UUID = oldIDs[r.cal.ID+"/"+r.ID]
		if d.UUID == "" {
			d.UUID = uuid.NewString()
		}
		d.Google = models.GoogleLink{
			ID:           r.ID,
			Calendar:     r.cal.ID,
			CalendarName: r.cal.Name(),
			Series:       r.RecurringEventID,
			Pulled:       true,
		}
		out = append(out, d)
		rep.Pulled++
	}

	c.state.LastSync = now
	return out, rep, nil
}

func (r remoteEvent) tuID() string {
	if r.ExtendedProperties == nil {
		return ""
	}
	return r.ExtendedProperties.Private[tuKey]
}

// findMatch looks for the Google event a local date corresponds to: one
// created from it, or one with the same name at the same time.
func findMatch(d models.TuDate, remote []remoteEvent, linked map[string]bool, now time.Time) *remoteEvent {
	for i, r := range remote {
		if r.tuID() == d.UUID {
			return &remote[i]
		}
	}
	for i, r := range remote {
		if linked[r.ID] || linked[r.RecurringEventID] {
			continue
		}
		if sameDate(d, models.TuDate{Name: r.Summary, Date: r.date, AllDay: r.allDay}, now) {
			return &remote[i]
		}
	}
	return nil
}

// sameDate reports whether two dates look like the same event: equal names
// (ignoring case) at the same moment, or on the same day for all-day dates.
func sameDate(a, b models.TuDate, now time.Time) bool {
	if a.AllDay != b.AllDay || !strings.EqualFold(strings.TrimSpace(a.Name), strings.TrimSpace(b.Name)) {
		return false
	}
	ao, bo := a.Occurrence(now), b.Occurrence(now)
	if a.AllDay {
		return ao.Format(time.DateOnly) == bo.Format(time.DateOnly)
	}
	return ao.Equal(bo)
}

func findCalendar(cals []Calendar, ref string) (Calendar, error) {
	if ref == "" {
		ref = "primary"
	}
	for _, cal := range cals {
		if cal.ID == ref || (ref == "primary" && cal.Primary) {
			return cal, nil
		}
	}
	for _, cal := range cals {
		if strings.EqualFold(cal.Name(), ref) {
			return cal, nil
		}
	}
	return Calendar{}, fmt.Errorf("no calendar %q, see: tu google calendars", ref)
}

func pullCalendars(cals []Calendar, refs []string) ([]Calendar, error) {
	if len(refs) == 0 {
		var out []Calendar
		for _, cal := range cals {
			if cal.Selected || cal.Primary {
				out = append(out, cal)
			}
		}
		return out, nil
	}
	var out []Calendar
	for _, ref := range refs {
		cal, err := findCalendar(cals, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, cal)
	}
	return out, nil
}

// eventStart reads when an event starts, all-day events start at local midnight.
func eventStart(ev Event, loc *time.Location) (time.Time, bool, bool) {
	if ev.Start == nil {
		return time.Time{}, false, false
	}
	if ev.Start.Date != "" {
		t, err := time.ParseInLocation(time.DateOnly, ev.Start.Date, loc)
		return t, true, err == nil
	}
	t, err := time.Parse(time.RFC3339, ev.Start.DateTime)
	return t.In(loc), false, err == nil
}

// eventBody builds the event fields tu manages. Timed dates become one hour
// events. Fields that don't apply are sent as null so a patch clears them.
func eventBody(d models.TuDate, timeZone string, create bool) map[string]any {
	var tz any
	if timeZone != "" {
		tz = timeZone
	}
	var start, end map[string]any
	if d.AllDay {
		start = map[string]any{"date": d.Date.Format(time.DateOnly), "dateTime": nil, "timeZone": nil}
		end = map[string]any{"date": d.Date.AddDate(0, 0, 1).Format(time.DateOnly), "dateTime": nil, "timeZone": nil}
	} else {
		start = map[string]any{"dateTime": d.Date.Format(time.RFC3339), "date": nil, "timeZone": tz}
		end = map[string]any{"dateTime": d.Date.Add(time.Hour).Format(time.RFC3339), "date": nil, "timeZone": tz}
	}
	body := map[string]any{
		"summary":    d.DisplayName(),
		"start":      start,
		"end":        end,
		"recurrence": recurrence(d),
	}
	if create {
		body["extendedProperties"] = map[string]any{"private": map[string]string{tuKey: d.UUID}}
		if d.AllDay {
			// Like birthdays, whole-day dates shouldn't show as busy.
			body["transparency"] = "transparent"
		}
	}
	return body
}

// recurrence turns a repeat into an RRULE. Like tu, dates on days that
// don't exist in every month fall on the last day of shorter months.
func recurrence(d models.TuDate) []string {
	day := d.Date.Day()
	switch d.Repeat {
	case models.RepeatDaily:
		return []string{"RRULE:FREQ=DAILY"}
	case models.RepeatWeekly:
		return []string{"RRULE:FREQ=WEEKLY"}
	case models.RepeatMonthly:
		if day > 28 {
			return []string{"RRULE:FREQ=MONTHLY;BYMONTHDAY=" + dayRange(28, day) + ";BYSETPOS=-1"}
		}
		return []string{"RRULE:FREQ=MONTHLY"}
	case models.RepeatYearly:
		if d.Date.Month() == time.February && day == 29 {
			return []string{"RRULE:FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=28,29;BYSETPOS=-1"}
		}
		return []string{"RRULE:FREQ=YEARLY"}
	}
	return []string{}
}

func dayRange(from, to int) string {
	var days []string
	for d := from; d <= to; d++ {
		days = append(days, fmt.Sprint(d))
	}
	return strings.Join(days, ",")
}
