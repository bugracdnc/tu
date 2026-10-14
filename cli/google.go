package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
	"tu/db"
	"tu/gcal"
)

const setupHelp = `tu needs an OAuth client of your own to talk to Google Calendar:

  1. Open https://console.cloud.google.com/ and create (or pick) a project
  2. APIs & Services > Library: enable the "Google Calendar API"
  3. APIs & Services > OAuth consent screen: set it up as "External" and add
     your own Google account under "Test users"
     (or "Publish app", otherwise Google logs you out every 7 days)
  4. APIs & Services > Credentials > Create credentials > OAuth client ID,
     application type "Desktop app", then download the JSON file
  5. Run: tu google login --client-file ~/Downloads/client_secret_....json`

type loginOptions struct {
	clientFile, clientID, clientSecret string
	noBrowser                          bool
}

func googleLogin(gstate *gcal.State, opts loginOptions) error {
	switch {
	case opts.clientFile != "":
		id, secret, err := gcal.ReadClientFile(opts.clientFile)
		if err != nil {
			return err
		}
		gstate.ClientID, gstate.ClientSecret = id, secret
	case opts.clientID != "":
		gstate.ClientID, gstate.ClientSecret = opts.clientID, opts.clientSecret
	case gstate.ClientID == "":
		return errors.New(setupHelp)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := gstate.Login(ctx, !opts.noBrowser, os.Stdin, os.Stdout); err != nil {
		return err
	}
	if err := gstate.Save(); err != nil {
		return err
	}

	client, _ := gstate.Client()
	cals, err := client.Calendars(ctx)
	if err != nil {
		return fmt.Errorf("logged in, but reading calendars failed: %w", err)
	}
	for _, cal := range cals {
		if cal.Primary {
			gstate.Account = cal.ID
		}
	}
	fmt.Printf("\nConnected to Google Calendar as %s.\n", gstate.Account)
	fmt.Println("Run `tu sync` to send your dates there and pull upcoming events.")
	return gstate.Save()
}

func googleLogout(gstate *gcal.State) error {
	if !gstate.LoggedIn() {
		fmt.Println("Not connected to Google Calendar.")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gstate.Logout(ctx)
	if err := gstate.Save(); err != nil {
		return err
	}
	fmt.Println("Disconnected from Google Calendar. Dates stay here, run `tu google login` to connect again.")
	return nil
}

func googleStatus(storage *db.Storage, config *db.Config, gstate *gcal.State) error {
	row := func(label, value string) {
		fmt.Printf("%s %s\n", paint(colorGray, fmt.Sprintf("%-14s", label+":")), value)
	}
	if !gstate.LoggedIn() {
		row("Google", "not connected (tu google login)")
		return nil
	}
	row("Google", "connected as "+gstate.Account)
	if gstate.LastSync.IsZero() {
		row("Last sync", "never")
	} else {
		row("Last sync", gstate.LastSync.Format("02/01/2006 15:04"))
	}
	row("Push to", config.PushCalendar())
	row("Pull from", settingValue(config, "google-calendars"))
	row("Pull", fmt.Sprintf("next %d events within %d days", config.PullCount(), config.PullDays()))

	var pushed, waiting, pulled int
	for _, d := range storage.Read() {
		switch {
		case d.Pulled():
			pulled++
		case d.Linked():
			pushed++
		case !d.LocalOnly && !d.IsPast(now()):
			waiting++
		}
	}
	row("Dates", fmt.Sprintf("%d in Google, %d pulled, %d waiting to be pushed", pushed, pulled, waiting))
	if n := len(gstate.PendingDeletes); n > 0 {
		row("Deletions", fmt.Sprintf("%d waiting", n))
	}
	return nil
}

func settingValue(config *db.Config, key string) string {
	s, _ := findSetting(key)
	return s.get(config)
}

func googleCalendars(config *db.Config, gstate *gcal.State) error {
	client, err := gstate.Client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cals, err := client.Calendars(ctx)
	if err != nil {
		return err
	}

	pulls := func(cal gcal.Calendar) bool {
		if len(config.GoogleCalendars) == 0 {
			return cal.Selected || cal.Primary
		}
		for _, ref := range config.GoogleCalendars {
			if ref == cal.ID || strings.EqualFold(ref, cal.Name()) || (ref == "primary" && cal.Primary) {
				return true
			}
		}
		return false
	}
	target := config.PushCalendar()
	for _, cal := range cals {
		mark := " "
		if pulls(cal) {
			mark = "↓"
		}
		if cal.ID == target || strings.EqualFold(cal.Name(), target) || (target == "primary" && cal.Primary) {
			mark = "⇅"
		}
		access := ""
		if !cal.Writable() {
			access = paint(colorGray, " (read-only)")
		}
		fmt.Printf("%s %s%s\n  %s\n", paint(colorCyan, mark), paint(colorWhite, cal.Name()), access, paint(colorGray, cal.ID))
	}
	fmt.Println(paint(colorGray, "\n⇅ new dates are pushed here and its events pulled, ↓ events are pulled"))
	fmt.Println(paint(colorGray, "Change with: tu config google-calendar <id>, tu config google-calendars <id,id,...>"))
	return nil
}

func syncOptions(config *db.Config, pushOnly bool) gcal.Options {
	return gcal.Options{
		Target:    config.PushCalendar(),
		Calendars: config.GoogleCalendars,
		Count:     config.PullCount(),
		Days:      config.PullDays(),
		PushOnly:  pushOnly,
	}
}

// runSync syncs and saves whatever got done, also when it failed halfway.
func runSync(storage *db.Storage, config *db.Config, gstate *gcal.State, pushOnly bool, timeout time.Duration) (gcal.Report, error) {
	client, err := gstate.Client()
	if err != nil {
		return gcal.Report{}, err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dates, rep, syncErr := client.Sync(ctx, storage.Read(), syncOptions(config, pushOnly), now())
	if err := storage.ReplaceAll(dates); err != nil {
		return rep, err
	}
	if err := gstate.Save(); err != nil {
		return rep, err
	}
	return rep, syncErr
}

func syncGoogle(storage *db.Storage, config *db.Config, gstate *gcal.State) error {
	rep, err := runSync(storage, config, gstate, false, 2*time.Minute)
	if parts := reportParts(rep); len(parts) > 0 {
		fmt.Println("Google Calendar: " + strings.Join(parts, ", ") + ".")
	}
	if err != nil {
		return err
	}
	fmt.Println()
	return printList(storage, config, listOptions{})
}

func reportParts(rep gcal.Report) []string {
	var parts []string
	add := func(n int, what string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, what))
		}
	}
	add(rep.Created, "added")
	add(rep.Updated, "updated")
	add(rep.Linked, "matched existing")
	add(rep.Changed, "changed from Google")
	add(rep.Deleted, "deleted")
	add(rep.Pulled, "pulled")
	return parts
}

// autoPush sends local changes to Google right after a command made them.
// Failing is fine, everything left is sent on the next sync.
func autoPush(storage *db.Storage, config *db.Config, gstate *gcal.State) {
	if gstate == nil || !gstate.LoggedIn() || !config.AutoPush() {
		return
	}
	rep, err := runSync(storage, config, gstate, true, 15*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, paint(colorGray, "Could not reach Google Calendar ("+err.Error()+"), run `tu sync` to retry."))
		return
	}
	if parts := reportParts(rep); len(parts) > 0 {
		fmt.Println(paint(colorGray, "Google Calendar: "+strings.Join(parts, ", ")))
	}
}
