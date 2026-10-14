# tu - Time Until Tracker

A lightweight, terminal-based countdown tracker written in Go. `tu` allows you to save target dates and track the time remaining until they occur, entirely from the command line.

## Features

- **Minimalist & Fast:** Built as a compiled Go binary for instant execution.
- **Local Storage:** Dates are saved locally in a simple JSON file—no heavy database services required.
- **Clean Architecture:** Utilizes dependency injection for modularity, separating command-line parsing logic from data storage, making the codebase highly testable.

## Installation

Ensure you have [Go](https://golang.org/) installed on your system.

Clone the repository and build the binary:

```bash
git clone https://github.com/bugracdnc/tu.git
cd tu
go build -o tu main.go
```

To run it from anywhere, move the compiled tu binary to a directory in your `$PATH` (e.g., `~/.local/bin` or `/usr/local/bin`).

## Usage

```bash
# List upcoming dates (same as `tu list`)
tu

# Track new dates, everything before the date is the name
tu add "Summer holiday" 2027-07-01
tu add Exam friday 9am
tu add Birthday 22/05/1995 --repeat yearly
tu add Standup 09:30 -r daily
tu track "2026-12-31 23:59:59"

# Listing
tu list --all          # include dates that have passed
tu list --past         # only dates that have passed
tu list --within 2w    # only what's coming up in the next two weeks
tu list --json         # machine readable output

# Inspect, change and remove dates by ID prefix, name or part of a name
tu show birthday
tu edit exam --date "2026-10-20 10:00" --name "Final exam"
tu edit birthday --repeat none
tu rm 3f9a1c "Summer holiday"

# Compact output for status bars and shell prompts
tu next
tu next -c 3

# Remove every date that has passed
tu clean --dry-run
tu clean

# Always show passed dates in the default list
tu config show-past true
```

### Date formats

| Input | Meaning |
| --- | --- |
| `2026-05-22`, `22/05/2026`, `22.05.2026` | a whole day |
| `2026-05-22 16:00`, `22/05/2026 16:00:30` | an exact time |
| `today`, `tomorrow`, `friday`, `next mon` | a whole day (weekdays are always the upcoming one) |
| `tomorrow 18:00`, `fri 9am` | an exact time on that day |
| `18:00` | the next time the clock shows 18:00 |
| `+3d`, `in 2 weeks`, `1y 2mo`, `1h30m`, `3 days ago` | relative to now |

Whole-day dates count in calendar days, so they show as `today` all day long and only count as passed once the day is over.

### Recurring dates

Dates added with `--repeat daily|weekly|monthly|yearly` always show their next occurrence and never become passed. Monthly and yearly dates on days that don't exist in every month (the 31st, 29 February) fall on the last day of shorter months.

### Google Calendar

`tu` can push your dates to Google Calendar and pull upcoming events (birthdays, holidays, appointments) from it.

**One-time setup.** Google only lets apps in that have their own OAuth client, so create one (it's free):

1. In the [Google Cloud console](https://console.cloud.google.com/), create or pick a project.
2. *APIs & Services → Library*: enable the **Google Calendar API**.
3. *APIs & Services → OAuth consent screen*: choose **External** and add your own Google account under **Test users**.
   While the app is in *Testing*, Google expires the login after 7 days. To avoid logging in again every week, click **Publish app** on the same page. It doesn't need verification for personal use; Google just shows an "unverified app" warning you can click through.
4. *APIs & Services → Credentials → Create credentials → OAuth client ID*, type **Desktop app**, and download the JSON file.

```bash
tu google login --client-file ~/Downloads/client_secret_....json
```

A browser opens to sign in with Google, then redirects back to `tu`. If you sign in on another device, or the browser lands on a page it can't reach, copy that page's address and paste it into the terminal instead (`--no-browser` only prints the link). The client is remembered, so later logins are just `tu google login`.

```bash
tu sync                       # push local changes, pull upcoming events
tu google                     # connection and sync status
tu google calendars           # your calendars and which ones are synced
tu add Standup 09:30 -r daily --local   # never push this one
tu rm "Republic Day"          # hide a pulled event, it won't be pulled again
tu rm exam --keep-google      # stop tracking here, but leave it in Google
tu google logout
```

- **Pushing:** upcoming dates you add here are created in your primary calendar, either all-day or at their time (as one-hour events). Repeating dates repeat in Google too. Changes from `tu edit` and removals from `tu rm` are sent right away. If that fails (e.g. offline), the next `tu sync` sends them. Edits made in Google to dates created here are taken over on sync. `tu clean` never deletes anything in Google.
- **Pulling:** `tu sync` pulls the next 25 events within a year from every calendar shown in Google Calendar, including Birthdays and any holiday calendars you've added there. They're marked with their calendar's name and replaced on every sync. Edit or delete them in Google itself.
- **No duplicates:** events that came from `tu` aren't pulled back. An event with the same name at the same time as one tracked here is only shown once, and isn't pushed again if it's already in Google.

| Setting | Default |
| --- | --- |
| `google-calendar` | `primary`, the calendar new dates go to |
| `google-calendars` | the calendars shown in Google Calendar; comma-separated IDs or names to pull from |
| `google-count` | `25` upcoming events to pull |
| `google-days` | `365` days ahead to look |
| `google-auto-push` | `true`; send changes right after `add`/`edit`/`rm` |

To keep pulled events fresh, run `tu sync` from cron (or Termux's `termux-job-scheduler`).

### Storage

Dates are stored in `~/.config/tu/data.db` (or `$XDG_CONFIG_HOME/tu/data.db`), settings in `config.json` next to it, and the Google login and sync state in `google.json` (readable only by you). Set `TU_DATA` to use a different data file. Colors are turned off when output is piped, when `NO_COLOR` is set, or with `--no-color`.

Shell completion (including IDs of tracked dates) is available through `tu completion bash|zsh|fish`.

## Project Structure

The codebase is organized to keep responsibilities strictly separated:

- `main.go:` The entry point. It acts as the central router, initializing the storage manager and injecting it into the CLI application.

- `cli/:` Contains the command-line interface logic. It receives the storage dependencies and handles user input/output without needing to know how the underlying data is stored.

- `db/:` The storage manager. It handles the internal logic for reading from and writing to the local JSON file.

- `gcal/:` Google Calendar support: the OAuth login, a small client for the Calendar API, and the sync logic.

- `models/:` Defines the core data structures (like the tracked date format) used across the application to ensure consistency.
