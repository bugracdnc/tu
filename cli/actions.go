package cli

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"tu/db"
	"tu/gcal"
	"tu/models"
)

// now is a variable so tests can pin the clock.
var now = time.Now

type listOptions struct {
	all    bool
	past   bool
	within string
	json   bool
}

func sortTuDates(at time.Time) func(a, b models.TuDate) int {
	return func(a, b models.TuDate) int {
		return a.Occurrence(at).Compare(b.Occurrence(at))
	}
}

// filterDates applies the list options and returns the dates sorted by their
// relevant occurrence.
func filterDates(dates []models.TuDate, opts listOptions, showPast bool, at time.Time) ([]models.TuDate, error) {
	var limit time.Time
	if opts.within != "" {
		off, err := models.ParseOffset(opts.within)
		if err != nil {
			return nil, err
		}
		limit = off.AddTo(at)
	}

	includePast := opts.all || opts.past || showPast
	filtered := slices.DeleteFunc(slices.Clone(dates), func(d models.TuDate) bool {
		past := d.IsPast(at)
		if opts.past && !past {
			return true
		}
		if past && !includePast {
			return true
		}
		if !limit.IsZero() && !past && d.Occurrence(at).After(limit) {
			return true
		}
		return false
	})
	slices.SortFunc(filtered, sortTuDates(at))
	return filtered, nil
}

type jsonDate struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Date         time.Time `json:"date"`
	AllDay       bool      `json:"all_day"`
	Repeat       string    `json:"repeat,omitempty"`
	Calendar     string    `json:"calendar,omitempty"`
	Past         bool      `json:"past"`
	SecondsUntil int64     `json:"seconds_until"`
}

func printList(storage *db.Storage, config *db.Config, opts listOptions) error {
	at := now()
	dates, err := filterDates(storage.Read(), opts, config.ShowPast, at)
	if err != nil {
		return err
	}

	if opts.json {
		out := make([]jsonDate, len(dates))
		for i, d := range dates {
			occ := d.Occurrence(at)
			out[i] = jsonDate{
				ID:           d.UUID,
				Name:         d.Name,
				Date:         occ,
				AllDay:       d.AllDay,
				Repeat:       string(d.Repeat),
				Calendar:     d.Google.CalendarName,
				Past:         d.IsPast(at),
				SecondsUntil: int64(occ.Sub(at) / time.Second),
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if len(dates) == 0 {
		switch {
		case len(storage.Read()) == 0:
			fmt.Println("Nothing tracked yet. Add something with: tu add \"Name\" 2026-12-31")
		case opts.past:
			fmt.Println("No passed dates.")
		default:
			fmt.Println("No upcoming dates. Use --all to include passed ones.")
		}
		return nil
	}

	for _, tuDate := range dates {
		fmt.Println(formatLine(tuDate, at))
	}

	if hidden := countPast(storage.Read(), at); hidden > 0 && !opts.all && !opts.past && !config.ShowPast {
		fmt.Println(paint(colorGray, fmt.Sprintf("  (%d passed hidden, use --all to show)", hidden)))
	}
	return nil
}

func countPast(dates []models.TuDate, at time.Time) int {
	n := 0
	for _, d := range dates {
		if d.IsPast(at) {
			n++
		}
	}
	return n
}

// splitNameDate separates the event name from the date expression. The date is
// the longest trailing run of arguments that parses, so both
// `tu add Exam friday 9am` and `tu add 2026-05-22` work.
func splitNameDate(args []string, at time.Time) (string, time.Time, bool, error) {
	var lastErr error
	for k := range args {
		date, allDay, err := models.ParseDate(strings.Join(args[k:], " "), at)
		if err == nil {
			return strings.Join(args[:k], " "), date, allDay, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no date given")
	}
	return "", time.Time{}, false, lastErr
}

func addToList(storage *db.Storage, args []string, name string, repeat string, localOnly bool) error {
	at := now()

	var date time.Time
	var allDay bool
	var err error
	if name != "" {
		date, allDay, err = models.ParseDate(strings.Join(args, " "), at)
	} else {
		name, date, allDay, err = splitNameDate(args, at)
	}
	if err != nil {
		return err
	}

	rep, err := models.ParseRepeat(repeat)
	if err != nil {
		return err
	}

	tuDate := models.NewTuDateFromDateWithName(date, strings.TrimSpace(name))
	tuDate.AllDay = allDay
	tuDate.Repeat = rep
	tuDate.LocalOnly = localOnly

	if err := storage.Add(*tuDate); err != nil {
		return err
	}
	fmt.Println("Tracking:")
	fmt.Println(formatLine(*tuDate, at))
	return nil
}

// removeFromList removes dates. Pushed ones are deleted from Google as well
// unless keepGoogle is set, pulled ones are not pulled again.
func removeFromList(storage *db.Storage, gstate *gcal.State, refs []string, keepGoogle bool) error {
	ids := []string{}
	removed := []models.TuDate{}
	for _, ref := range refs {
		d, err := storage.Find(ref)
		if err != nil {
			return err
		}
		if !slices.Contains(ids, d.UUID) {
			ids = append(ids, d.UUID)
			removed = append(removed, d)
		}
	}
	for _, d := range removed {
		switch {
		case d.Pulled():
			gstate.Ignore(cmp.Or(d.Google.Series, d.Google.ID))
		case d.Linked() && keepGoogle:
			gstate.Ignore(d.Google.ID)
		case d.Linked():
			gstate.Forget(d.Google.ID, d.Google.Calendar)
		}
	}
	if err := gstate.Save(); err != nil {
		return err
	}
	if _, err := storage.Remove(ids...); err != nil {
		return err
	}
	for _, d := range removed {
		fmt.Printf("Removed %s (%s)\n", d.DisplayName(), d.ShortID())
	}
	return nil
}

type editOptions struct {
	name      string
	date      string
	repeat    string
	localOnly bool
}

func editDate(storage *db.Storage, gstate *gcal.State, ref string, opts editOptions, changed func(string) bool) error {
	d, err := storage.Find(ref)
	if err != nil {
		return err
	}
	if !changed("name") && !changed("date") && !changed("repeat") && !changed("local") {
		return errors.New("nothing to change, use --name, --date, --repeat or --local")
	}
	if d.Pulled() {
		return fmt.Errorf("%s comes from Google Calendar (%s), change it there", d.DisplayName(), d.Google.CalendarName)
	}

	if changed("name") {
		d.Name = strings.TrimSpace(opts.name)
	}
	if changed("date") {
		date, allDay, err := models.ParseDate(opts.date, now())
		if err != nil {
			return err
		}
		d.Date, d.AllDay = date, allDay
	}
	if changed("repeat") {
		if d.Repeat, err = models.ParseRepeat(opts.repeat); err != nil {
			return err
		}
	}

	if changed("local") {
		d.LocalOnly = opts.localOnly
		if d.LocalOnly && d.Linked() {
			gstate.Forget(d.Google.ID, d.Google.Calendar)
			d.Google = models.GoogleLink{}
			if err := gstate.Save(); err != nil {
				return err
			}
		}
	}

	if err := storage.Update(d); err != nil {
		return err
	}
	fmt.Println("Updated:")
	fmt.Println(formatLine(d, now()))
	return nil
}

func showDate(storage *db.Storage, ref string) error {
	d, err := storage.Find(ref)
	if err != nil {
		return err
	}
	at := now()
	occ := d.Occurrence(at)

	row := func(label, value string) {
		fmt.Printf("%s %s\n", paint(colorGray, fmt.Sprintf("%-10s", label+":")), value)
	}
	row("Name", paint(colorWhite, d.DisplayName()))
	row("ID", d.UUID)
	switch {
	case d.Pulled():
		row("Calendar", d.Google.CalendarName+" (Google)")
	case d.Linked():
		row("Google", "in "+d.Google.Calendar)
	case d.LocalOnly:
		row("Google", "local only")
	}
	if d.Repeat != models.RepeatNone {
		row("Repeats", string(d.Repeat)+", since "+formatDateString(d.Date, d.AllDay))
		row("Next", formatDateLong(occ, d.AllDay))
	} else {
		row("Date", formatDateLong(occ, d.AllDay))
	}

	span := breakdown(at, occ, d.AllDay)
	days := calendarDays(at, occ)
	if !d.AllDay {
		days = int64(occ.Sub(at) / Day)
	}
	if d.IsPast(at) {
		row("Passed", span+" ago")
	} else {
		row("Left", paint(colorWhite, span))
	}
	if days < -1 || days > 1 {
		row("Days", fmt.Sprintf("%d", max(days, -days)))
	}
	return nil
}

// printNext prints the soonest upcoming dates in a compact form that suits
// status bars and shell prompts.
func printNext(storage *db.Storage, count int) error {
	at := now()
	dates, err := filterDates(storage.Read(), listOptions{}, false, at)
	if err != nil {
		return err
	}
	if len(dates) == 0 {
		return errors.New("no upcoming dates")
	}
	for _, d := range dates[:min(count, len(dates))] {
		untilStr, _ := calculateUntilDate(d, at)
		fmt.Printf("%s: %s\n", d.DisplayName(), strings.TrimSpace(untilStr))
	}
	return nil
}

func cleanPast(storage *db.Storage, dryRun bool) error {
	at := now()
	var past []models.TuDate
	for _, d := range storage.Read() {
		if d.IsPast(at) {
			past = append(past, d)
		}
	}
	if len(past) == 0 {
		fmt.Println("No passed dates to clean.")
		return nil
	}
	slices.SortFunc(past, sortTuDates(at))

	ids := make([]string, len(past))
	for i, d := range past {
		ids[i] = d.UUID
		fmt.Println(formatLine(d, at))
	}
	if dryRun {
		fmt.Printf("Would remove %d passed date(s).\n", len(past))
		return nil
	}
	n, err := storage.Remove(ids...)
	if err != nil {
		return err
	}
	fmt.Printf("Removed %d passed date(s).\n", n)
	return nil
}

type setting struct {
	key, help string
	values    []string
	get       func(*db.Config) string
	set       func(*db.Config, string) error
}

var settings = []setting{
	{
		key: "show-past", help: "include passed dates in the default list (true/false)",
		values: []string{"true", "false"},
		get:    func(c *db.Config) string { return fmt.Sprint(c.ShowPast) },
		set: func(c *db.Config, v string) (err error) {
			c.ShowPast, err = parseBool(v)
			return err
		},
	},
	{
		key: "google-calendar", help: "Google calendar new dates are added to (ID or name, default primary)",
		get: func(c *db.Config) string { return c.PushCalendar() },
		set: func(c *db.Config, v string) error {
			c.GoogleCalendar = strings.TrimSpace(v)
			return nil
		},
	},
	{
		key: "google-calendars", help: "comma separated Google calendars to pull from (default: the ones shown in Google Calendar)",
		get: func(c *db.Config) string {
			if len(c.GoogleCalendars) == 0 {
				return "(shown in Google Calendar)"
			}
			return strings.Join(c.GoogleCalendars, ",")
		},
		set: func(c *db.Config, v string) error {
			c.GoogleCalendars = nil
			for _, cal := range strings.Split(v, ",") {
				if cal = strings.TrimSpace(cal); cal != "" {
					c.GoogleCalendars = append(c.GoogleCalendars, cal)
				}
			}
			return nil
		},
	},
	{
		key: "google-count", help: "how many upcoming Google events to pull (default 25)",
		get: func(c *db.Config) string { return fmt.Sprint(c.PullCount()) },
		set: func(c *db.Config, v string) (err error) {
			c.GoogleCount, err = parsePositive(v)
			return err
		},
	},
	{
		key: "google-days", help: "how many days ahead to pull Google events (default 365)",
		get: func(c *db.Config) string { return fmt.Sprint(c.PullDays()) },
		set: func(c *db.Config, v string) (err error) {
			c.GoogleDays, err = parsePositive(v)
			return err
		},
	},
	{
		key: "google-auto-push", help: "send added, changed and removed dates to Google right away (true/false)",
		values: []string{"true", "false"},
		get:    func(c *db.Config) string { return fmt.Sprint(c.AutoPush()) },
		set: func(c *db.Config, v string) error {
			b, err := parseBool(v)
			c.GoogleAutoPush = &b
			return err
		},
	},
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(v) {
	case "true", "on", "yes", "1":
		return true, nil
	case "false", "off", "no", "0":
		return false, nil
	}
	return false, fmt.Errorf("expected true or false, got %q", v)
}

func parsePositive(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("expected a positive number, got %q", v)
	}
	return n, nil
}

func findSetting(key string) (setting, error) {
	i := slices.IndexFunc(settings, func(s setting) bool { return s.key == key })
	if i < 0 {
		keys := make([]string, len(settings))
		for i, s := range settings {
			keys[i] = s.key
		}
		return setting{}, fmt.Errorf("unknown setting %q (available: %s)", key, strings.Join(keys, ", "))
	}
	return settings[i], nil
}

func configure(config *db.Config, args []string) error {
	if len(args) == 0 {
		for _, s := range settings {
			fmt.Printf("%s = %s\n", s.key, s.get(config))
		}
		return nil
	}

	s, err := findSetting(args[0])
	if err != nil {
		return err
	}
	if len(args) == 1 {
		fmt.Println(s.get(config))
		return nil
	}
	if err := s.set(config, args[1]); err != nil {
		return fmt.Errorf("%s: %w", s.key, err)
	}
	if err := config.Save(); err != nil {
		return err
	}
	fmt.Printf("%s = %s\n", s.key, s.get(config))
	return nil
}
