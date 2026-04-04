package models

import (
	"errors"
	"time"
)

type TuDate struct {
	Date time.Time
}

func NewTuDateFromString(dateStr string) (*TuDate, error) {
	var tmp TuDate

	if len(dateStr) < 19 {
		return nil, errors.New("Date string too short")
	}

	if parsedDate, err := time.Parse("2006-01-02 15:04:05", dateStr[0:19]); err != nil {
		return nil, err
	} else {
		tmp.Date = parsedDate
	}

	return &tmp, nil
}

func NewTuDateFromDate(date time.Time) *TuDate {
	var tmp TuDate
	tmp.Date = date

	return &tmp
}
