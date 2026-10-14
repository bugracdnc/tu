package gcal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var ErrNotLoggedIn = errors.New("not connected to Google Calendar, run: tu google login")

var apiBase = "https://www.googleapis.com/calendar/v3"

// APIError is an error answer from the Calendar API.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("google calendar: %s (%d)", e.Message, e.Status)
}

// gone reports whether the error means the event doesn't exist (anymore).
func gone(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.Status == http.StatusNotFound || apiErr.Status == http.StatusGone)
}

type Calendar struct {
	ID              string `json:"id"`
	Summary         string `json:"summary"`
	SummaryOverride string `json:"summaryOverride"`
	TimeZone        string `json:"timeZone"`
	AccessRole      string `json:"accessRole"`
	Selected        bool   `json:"selected"`
	Primary         bool   `json:"primary"`
}

func (c Calendar) Name() string {
	if c.SummaryOverride != "" {
		return c.SummaryOverride
	}
	return c.Summary
}

func (c Calendar) Writable() bool {
	return c.AccessRole == "owner" || c.AccessRole == "writer"
}

type EventTime struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type Event struct {
	ID               string     `json:"id"`
	Summary          string     `json:"summary"`
	Status           string     `json:"status"`
	EventType        string     `json:"eventType"`
	Start            *EventTime `json:"start"`
	RecurringEventID string     `json:"recurringEventId"`
	Recurrence       []string   `json:"recurrence"`

	ExtendedProperties *struct {
		Private map[string]string `json:"private"`
	} `json:"extendedProperties"`
}

type Client struct {
	state *State
	http  *http.Client
}

func (s *State) Client() (*Client, error) {
	if !s.LoggedIn() {
		return nil, ErrNotLoggedIn
	}
	return &Client{state: s, http: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	u := apiBase + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	for attempt := 0; ; attempt++ {
		token, err := c.state.accessToken(ctx, attempt > 0)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		// An access token can be revoked before it expires, retry once with a fresh one.
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			continue
		}
		if resp.StatusCode >= 300 {
			var e struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			json.Unmarshal(data, &e)
			if e.Error.Message == "" {
				e.Error.Message = resp.Status
			}
			return &APIError{Status: resp.StatusCode, Message: e.Error.Message}
		}
		if out != nil && len(data) > 0 {
			return json.Unmarshal(data, out)
		}
		return nil
	}
}

func (c *Client) Calendars(ctx context.Context) ([]Calendar, error) {
	var all []Calendar
	q := url.Values{"maxResults": {"250"}}
	for {
		var page struct {
			Items         []Calendar `json:"items"`
			NextPageToken string     `json:"nextPageToken"`
		}
		if err := c.do(ctx, http.MethodGet, "/users/me/calendarList", q, nil, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Items...)
		if page.NextPageToken == "" {
			return all, nil
		}
		q.Set("pageToken", page.NextPageToken)
	}
}

// Events lists the occurrences that haven't ended before `from` and start
// before `to`, with recurring events expanded, earliest first.
func (c *Client) Events(ctx context.Context, calendar string, from, to time.Time, limit int) ([]Event, error) {
	var page struct {
		Items []Event `json:"items"`
	}
	q := url.Values{
		"timeMin":      {from.Format(time.RFC3339)},
		"timeMax":      {to.Format(time.RFC3339)},
		"singleEvents": {"true"},
		"orderBy":      {"startTime"},
		"maxResults":   {fmt.Sprint(limit)},
	}
	err := c.do(ctx, http.MethodGet, "/calendars/"+url.PathEscape(calendar)+"/events", q, nil, &page)
	return page.Items, err
}

func (c *Client) Insert(ctx context.Context, calendar string, body map[string]any) (Event, error) {
	var ev Event
	err := c.do(ctx, http.MethodPost, "/calendars/"+url.PathEscape(calendar)+"/events", nil, body, &ev)
	return ev, err
}

func (c *Client) Patch(ctx context.Context, calendar, id string, body map[string]any) error {
	return c.do(ctx, http.MethodPatch, "/calendars/"+url.PathEscape(calendar)+"/events/"+url.PathEscape(id), nil, body, nil)
}

// Delete removes an event, an event that is already gone counts as deleted.
func (c *Client) Delete(ctx context.Context, calendar, id string) error {
	err := c.do(ctx, http.MethodDelete, "/calendars/"+url.PathEscape(calendar)+"/events/"+url.PathEscape(id), nil, nil, nil)
	if gone(err) {
		return nil
	}
	return err
}
