package gcal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"slices"
	"strings"
	"testing"
	"time"
	"tu/models"
)

// Monday, 12 October 2026 14:30
var testNow = time.Date(2026, 10, 12, 14, 30, 0, 0, time.Local)

func local(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.Local)
}

// fakeGoogle is a tiny in-memory stand-in for the Calendar API.
type fakeGoogle struct {
	cals    []Calendar
	events  map[string][]map[string]any // calendar -> events
	nextID  int
	inserts int
	patches int
	deletes []string
}

func (f *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if r.URL.Path == "/users/me/calendarList" {
		json.NewEncoder(w).Encode(map[string]any{"items": f.cals})
		return
	}
	// /calendars/{cal}/events[/{id}]
	cal := parts[1]
	switch {
	case r.Method == http.MethodGet:
		json.NewEncoder(w).Encode(map[string]any{"items": f.events[cal]})
	case r.Method == http.MethodPost:
		var ev map[string]any
		json.NewDecoder(r.Body).Decode(&ev)
		f.nextID++
		ev["id"] = fmt.Sprintf("new%d", f.nextID)
		f.events[cal] = append(f.events[cal], ev)
		f.inserts++
		json.NewEncoder(w).Encode(ev)
	case r.Method == http.MethodPatch:
		id := path.Base(r.URL.Path)
		for _, ev := range f.events[cal] {
			if ev["id"] == id {
				json.NewDecoder(r.Body).Decode(&ev)
				f.patches++
				w.Write([]byte("{}"))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":{"message":"Not Found"}}`))
	case r.Method == http.MethodDelete:
		f.deletes = append(f.deletes, path.Base(r.URL.Path))
		w.WriteHeader(http.StatusNoContent)
	}
}

func allDayEvent(id, name, date string) map[string]any {
	return map[string]any{"id": id, "summary": name, "start": map[string]any{"date": date}}
}

func timedEvent(id, name string, at time.Time) map[string]any {
	return map[string]any{"id": id, "summary": name, "start": map[string]any{"dateTime": at.Format(time.RFC3339)}}
}

func setup(t *testing.T) (*fakeGoogle, *Client) {
	f := &fakeGoogle{
		cals: []Calendar{
			{ID: "me@example.com", Summary: "me@example.com", AccessRole: "owner", Primary: true, Selected: true, TimeZone: "Europe/Istanbul"},
			{ID: "holidays", Summary: "Holidays in Turkey", AccessRole: "reader", Selected: true},
			{ID: "hidden", Summary: "Hidden", AccessRole: "reader"},
		},
		events: map[string][]map[string]any{},
	}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	old := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = old })

	state := &State{
		ClientID: "id",
		Token:    &Token{AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour)},
		path:     t.TempDir() + "/google.json",
	}
	c, err := state.Client()
	if err != nil {
		t.Fatal(err)
	}
	return f, c
}

var opts = Options{Target: "primary", Count: 10, Days: 365}

func names(dates []models.TuDate) string {
	var out []string
	for _, d := range dates {
		out = append(out, d.Name)
	}
	slices.Sort(out)
	return strings.Join(out, ",")
}

func TestSyncPushAndPull(t *testing.T) {
	f, c := setup(t)
	f.events["holidays"] = []map[string]any{
		allDayEvent("h1", "Republic Day", "2026-10-29"),
		// Already tracked locally, must not show up twice.
		allDayEvent("h2", "New Year's Day", "2027-01-01"),
	}
	f.events["hidden"] = []map[string]any{allDayEvent("x", "Not shown", "2026-10-20")}
	f.events["me@example.com"] = []map[string]any{
		timedEvent("m1", "Dentist", local(2026, 10, 15, 10, 0)),
		{"id": "wl", "summary": "Home", "eventType": "workingLocation", "start": map[string]any{"date": "2026-10-13"}},
		allDayEvent("p1", "Last week", "2026-10-05"),
	}

	dates := []models.TuDate{
		{UUID: "u1", Name: "Exam", Date: local(2026, 10, 16, 9, 0)},
		{UUID: "u2", Name: "new year's day", Date: local(2027, 1, 1, 0, 0), AllDay: true},
		{UUID: "u3", Name: "Standup", Date: local(2026, 10, 13, 9, 30), Repeat: models.RepeatDaily, LocalOnly: true},
		{UUID: "u4", Name: "Old", Date: local(2026, 1, 1, 0, 0), AllDay: true},
	}

	out, rep, err := c.Sync(context.Background(), dates, opts, testNow)
	if err != nil {
		t.Fatal(err)
	}

	// Only Exam is pushed: New Year is already in Google (holidays), the
	// others are local only or past.
	if f.inserts != 1 || rep.Created != 1 {
		t.Errorf("wanted 1 insert, got %d (report %+v)", f.inserts, rep)
	}
	if got := names(out); got != "Dentist,Exam,Old,Republic Day,Standup,new year's day" {
		t.Errorf("got dates %s", got)
	}
	for _, d := range out {
		if d.Name == "Republic Day" && (!d.Pulled() || d.Google.CalendarName != "Holidays in Turkey" || !d.AllDay) {
			t.Errorf("bad pulled date %+v", d)
		}
		if d.Name == "Exam" && (!d.Linked() || d.Pulled()) {
			t.Errorf("Exam should be linked: %+v", d)
		}
	}

	// A second sync changes nothing and keeps the IDs of pulled dates.
	ids := map[string]string{}
	for _, d := range out {
		ids[d.Name] = d.UUID
	}
	again, rep, err := c.Sync(context.Background(), out, opts, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if f.inserts != 1 || f.patches != 0 || rep.Pulled != 2 {
		t.Errorf("second sync should be a no-op, got %d inserts, %d patches, report %+v", f.inserts, f.patches, rep)
	}
	for _, d := range again {
		if ids[d.Name] != d.UUID {
			t.Errorf("%s changed ID from %s to %s", d.Name, ids[d.Name], d.UUID)
		}
	}
}

func TestSyncEditsAndDeletes(t *testing.T) {
	f, c := setup(t)
	dates := []models.TuDate{{UUID: "u1", Name: "Exam", Date: local(2026, 10, 16, 9, 0)}}
	dates, _, err := c.Sync(context.Background(), dates, opts, testNow)
	if err != nil {
		t.Fatal(err)
	}

	// Changed here: patched in Google.
	dates[0].Date = local(2026, 10, 17, 9, 0)
	dates, _, err = c.Sync(context.Background(), dates, opts, testNow)
	if err != nil || f.patches != 1 {
		t.Fatalf("wanted a patch, got %d (%v)", f.patches, err)
	}
	start := f.events["me@example.com"][0]["start"].(map[string]any)
	if start["dateTime"] != local(2026, 10, 17, 9, 0).Format(time.RFC3339) || start["timeZone"] != "Europe/Istanbul" {
		t.Errorf("patched start is %v", start)
	}

	// Changed in Google: taken over here.
	f.events["me@example.com"][0]["summary"] = "Final exam"
	dates, rep, err := c.Sync(context.Background(), dates, opts, testNow)
	if err != nil || rep.Changed != 1 || dates[0].Name != "Final exam" || f.patches != 1 {
		t.Errorf("wanted the name from Google, got %q (report %+v, %d patches, %v)", dates[0].Name, rep, f.patches, err)
	}

	// Removed here: deleted in Google.
	c.state.Forget(dates[0].Google.ID, dates[0].Google.Calendar)
	f.events["me@example.com"] = nil
	if _, _, err := c.Sync(context.Background(), nil, opts, testNow); err != nil || !slices.Equal(f.deletes, []string{"new1"}) {
		t.Errorf("wanted new1 deleted, got %v (%v)", f.deletes, err)
	}
	if len(c.state.PendingDeletes) != 0 {
		t.Error("pending deletions not cleared")
	}
}

func TestSyncAdoptsExistingEvents(t *testing.T) {
	f, c := setup(t)
	// Created by tu before the local data was lost, then the same date was added again.
	ev := timedEvent("e1", "Exam", local(2026, 10, 16, 9, 0))
	f.events["me@example.com"] = []map[string]any{ev}

	dates := []models.TuDate{{UUID: "u1", Name: "exam", Date: local(2026, 10, 16, 9, 0)}}
	out, rep, err := c.Sync(context.Background(), dates, opts, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if f.inserts != 0 || rep.Linked != 1 || len(out) != 1 || out[0].Google.ID != "e1" {
		t.Errorf("wanted the event adopted, got %d inserts, %+v", f.inserts, out)
	}
}

func TestSyncIgnoredAndCount(t *testing.T) {
	f, c := setup(t)
	f.events["holidays"] = []map[string]any{
		allDayEvent("h1", "A", "2026-10-20"),
		allDayEvent("h2", "B", "2026-10-21"),
		allDayEvent("h3", "C", "2026-10-22"),
		allDayEvent("h4", "D", "2026-10-23"),
	}
	c.state.Ignore("h2")
	o := opts
	o.Count = 2
	out, _, err := c.Sync(context.Background(), nil, o, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(out); got != "A,C" {
		t.Errorf("got %s, want A,C", got)
	}
}

func TestRecurrence(t *testing.T) {
	tests := []struct {
		d    models.TuDate
		want string
	}{
		{models.TuDate{Date: local(2026, 1, 31, 0, 0), Repeat: models.RepeatMonthly}, "RRULE:FREQ=MONTHLY;BYMONTHDAY=28,29,30,31;BYSETPOS=-1"},
		{models.TuDate{Date: local(2026, 1, 15, 0, 0), Repeat: models.RepeatMonthly}, "RRULE:FREQ=MONTHLY"},
		{models.TuDate{Date: local(1996, 2, 29, 0, 0), Repeat: models.RepeatYearly}, "RRULE:FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=28,29;BYSETPOS=-1"},
		{models.TuDate{Date: local(1995, 5, 22, 0, 0), Repeat: models.RepeatYearly}, "RRULE:FREQ=YEARLY"},
		{models.TuDate{Date: local(2026, 5, 22, 0, 0)}, ""},
	}
	for _, tt := range tests {
		if got := strings.Join(recurrence(tt.d), ""); got != tt.want {
			t.Errorf("%v %s: got %q, want %q", tt.d.Date, tt.d.Repeat, got, tt.want)
		}
	}
}

func TestEventBodyAllDay(t *testing.T) {
	b := eventBody(models.TuDate{UUID: "u", Name: "Trip", Date: local(2026, 12, 31, 0, 0), AllDay: true}, "Europe/Istanbul", true)
	start, end := b["start"].(map[string]any), b["end"].(map[string]any)
	if start["date"] != "2026-12-31" || end["date"] != "2027-01-01" || start["dateTime"] != nil {
		t.Errorf("bad all-day times %v %v", start, end)
	}
}
