package cli

import (
	"fmt"
	"log"
	"math"
	"time"
	"tu/db"
	"tu/models"
)

func calculateUntilDate(date time.Time) string {
	until := time.Until(date)
	days := int(until.Hours() / 24)
	hours := int(until.Hours()) - (days * 24)
	minutes := math.Ceil(until.Hours() - float64(days*24))

	return fmt.Sprintf("%d days, %d hours, %v minutes", days, hours, minutes)
}

func formatDateString(date time.Time) string {
	return date.Format("2006-01-02 15:04:05")
}

func printList(db db.Storage) {
	maxlen := 0
	for _, tuDate := range db.Read() {
		if len(tuDate.Name) > maxlen {
			maxlen = len(tuDate.Name)
		}
	}

	for _, tuDate := range db.Read() {
		fmtStr := fmt.Sprintf("%%%ds in %%s (%%s)\n", maxlen-3)
		log.Printf(fmtStr, tuDate.Name, calculateUntilDate(tuDate.Date), formatDateString(tuDate.Date))
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
