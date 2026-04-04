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

func TestNewTuDateFromStringWithName(t *testing.T) {
	date, err := time.Parse("2006-01-02 15:04:05", exampleDateString)
	name := "testing environment"
	tuDate, err := NewTuDateFromStringWithName(date.String(), name)

	if err != nil {
		t.Error(err.Error())
	}

	if date.Compare(tuDate.Date) != 0 {
		t.Errorf("got %s want %s", tuDate.Date, date)
	}
	if tuDate.Name != name {
		t.Errorf("got name as %s, want %s", tuDate.Name, name)
	}
}

func TestNewTuDateFromStringWithName_BadString(t *testing.T) {
	var exampleBadString = "2026-5-22 4:0"
	name := "testing environment"
	tuDate, err := NewTuDateFromStringWithName(exampleBadString, name)

	if tuDate != nil && err == nil {
		t.Errorf("got %v, wanted nil", tuDate)
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
func TestNewTuDateFromString_BadShortString(t *testing.T) {
	var exampleBadString = "2026-5-22 4:0"

	tuDate, err := NewTuDateFromString(exampleBadString)

	if err == nil {
		t.Error("NewTuDateFromString should return error, instead it returned: ", tuDate)
	}
}

func TestNewTuDateFromString_BadDateString(t *testing.T) {
	var exampleBadString = "2026-05-22 25:43:35"

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

func TestNewTuDateFromDateWithName(t *testing.T) {
	dateNow := time.Now()
	name := "testing environment"
	tuDate := NewTuDateFromDateWithName(dateNow, name)

	if dateNow != tuDate.Date {
		t.Errorf("got %v want %v", tuDate.Date, dateNow)
	}
	if tuDate.Name != name {
		t.Errorf("got name as %s, want %s", tuDate.Name, name)
	}
}
