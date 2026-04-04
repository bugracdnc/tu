package models

import (
	"testing"
	"time"
)

var exampleDateString = "2026-05-22 16:00:00"
var exampleTimeString = "16:00:00"

// When given a valid string, the function should parse the date using built in methods
// and return a valid object
func TestNewTuDateFromString(t *testing.T) {
	date, err := time.Parse("2006-01-02 15:04:05", exampleDateString)
	tuDate, err := NewTuDateFromString(date.String())

	if err != nil {
		t.Error(err.Error())
	}

	if date.Compare(tuDate.Date) != 0 {
		t.Errorf("got %s want %s", tuDate.Date, date)
	}
}

// When given a proper time string (with zeros in date section)
// the function should return a date with empty date and valid time
func TestNewTuDateFromTimeString(t *testing.T) {
	time, err := time.Parse("15:04:05", exampleTimeString)
	tuDate, err := NewTuDateFromString(time.String())

	if err != nil {
		t.Error(err.Error())
	}

	if tuDate != nil {
		if time.Compare(tuDate.Date) != 0 {
			t.Errorf("got %s want %s", tuDate.Date, time)
		}
	}
}

// When given a short bad string, the function should throw an error
func TestNewTuDateFromString_BadString(t *testing.T) {
	var exampleBadString = "2026-5-22 4:0"

	tuDate, err := NewTuDateFromString(exampleBadString)

	if err == nil {
		t.Error("NewTuDateFromString should return error, instead it returned: ", tuDate)
	}
}

// When given a date object, the constructor function should just create an object and copy the date
// object into it, should not cause any errors
func TestNewTuDateFromDate(t *testing.T) {
	dateNow := time.Now()
	tudate := NewTuDateFromDate(dateNow)

	if dateNow != tudate.Date {
		t.Errorf("got %v want %v", tudate.Date, dateNow)
	}
}
