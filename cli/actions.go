package cli

import (
	"fmt"
	"log"
	"slices"
	"time"
	"tu/db"
	"tu/models"
)

const (
	Day  = 24 * time.Hour
	Year = 365 * Day
)

func calculateUntilDate(date time.Time) (string, int) {
	mark := 1
	d := time.Until(date)

	// Catch dates that have already passed
	if d <= 0 {
		d = time.Since(date)
		mark = -1
	}

	// Helper function for quick formatting and pluralization
	format := func(val time.Duration, unit string) string {
		if val == 1 {
			return fmt.Sprintf("1 %s", unit) // Singular
		}
		return fmt.Sprintf("%d %ss", val, unit) // Plural
	}

	// Check from largest to smallest.
	// The first one > 0 is our biggest operand, so we return immediately.
	if years := d / Year; years > 0 {
		return format(years, "year"), mark
	}
	if days := d / Day; days > 0 {
		return format(days, "day"), mark
	}
	if hours := d / time.Hour; hours > 0 {
		return format(hours, "hour"), mark
	}
	if minutes := d / time.Minute; minutes > 0 {
		return format(minutes, "minute"), mark
	}

	// If we get here, it's less than a minute away
	return format(d/time.Second, "second"), mark
}

func formatDateString(date time.Time) string {
	return date.Format("02/01/2006")
}

func sortTuDates(a, b models.TuDate) int {
	if a.Date.Before(b.Date) {
		return -1
	} else if b.Date.Before(a.Date) {
		return 1
	}
	return 0
}

func printList(db db.Storage) {
	tuDates := db.Read()
	slices.SortFunc(tuDates, sortTuDates)

	for _, tuDate := range tuDates {
		untilStr, toOrSince := calculateUntilDate(tuDate.Date)
		if toOrSince < 0 {
			fmt.Printf("* \033[0;90m[Passed] \033[0;37m%s\033[0;90m \033[0;90mfor [\033[0;37m%s\033[0;90m] (since %s)\n", untilStr, tuDate.Name, formatDateString(tuDate.Date))
		} else {
			fmt.Printf("* \033[0;37m%s \033[0;90mto [\033[0;37m%s\033[0;90m] (at %s)\n", untilStr, tuDate.Name, formatDateString(tuDate.Date))
		}
	}
}

func addToList(db *db.Storage, arg string) error {
	tudate, err := models.NewTuDateFromString(arg)
	if err != nil {
		return err
	}
	if err := db.Add(*tudate); err == nil {
		log.Println("New date successfully added")
		return nil
	} else {
		return err
	}

}
