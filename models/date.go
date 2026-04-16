package models

import (
	"time"

	"github.com/google/uuid"
)

type TuDate struct {
	UUID string    `json:"id"`
	Name string    `json:"name"`
	Date time.Time `json:"date"`
}

func NewTuDateFromString(dateStr string) (*TuDate, error) {
	if len(dateStr) > 19 {
		dateStr = dateStr[0:19]
	}

	var tmp TuDate
	tmp.UUID = uuid.New().String()

	layoutStr := "2006-01-02 15:04:05"
	if len(dateStr) < 19 {
		layoutStr = "2006-01-02"
	}

	parsedDate, err := time.ParseInLocation(layoutStr, dateStr, time.Local)
	if err != nil {
		return nil, err
	} else {
		tmp.Date = parsedDate
	}

	return &tmp, nil
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
