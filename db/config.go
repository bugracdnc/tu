package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds user preferences that change the default behaviour of tu.
type Config struct {
	ShowPast bool `json:"show_past"`

	// Google Calendar sync
	GoogleCalendar  string   `json:"google_calendar,omitempty"`
	GoogleCalendars []string `json:"google_calendars,omitempty"`
	GoogleCount     int      `json:"google_count,omitempty"`
	GoogleDays      int      `json:"google_days,omitempty"`
	GoogleAutoPush  *bool    `json:"google_auto_push,omitempty"`

	filepath string
}

func LoadConfig(path string) (*Config, error) {
	c := &Config{filepath: path}
	byt, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if len(byt) > 0 {
		if err := json.Unmarshal(byt, c); err != nil {
			return nil, fmt.Errorf("config file %s is not valid JSON: %w", path, err)
		}
	}
	return c, nil
}

func (c *Config) Save() error {
	byt, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.filepath), 0755); err != nil {
		return err
	}
	return os.WriteFile(c.filepath, byt, 0644)
}

// PushCalendar is the calendar dates created in tu are pushed to.
func (c *Config) PushCalendar() string {
	if c.GoogleCalendar == "" {
		return "primary"
	}
	return c.GoogleCalendar
}

// PullCount is how many upcoming Google events are pulled.
func (c *Config) PullCount() int {
	if c.GoogleCount <= 0 {
		return 25
	}
	return c.GoogleCount
}

// PullDays is how many days ahead Google events are pulled.
func (c *Config) PullDays() int {
	if c.GoogleDays <= 0 {
		return 365
	}
	return c.GoogleDays
}

// AutoPush reports whether changes are sent to Google right away.
func (c *Config) AutoPush() bool {
	return c.GoogleAutoPush == nil || *c.GoogleAutoPush
}
