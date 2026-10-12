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

### Storage

Dates are stored in `~/.config/tu/data.db` (or `$XDG_CONFIG_HOME/tu/data.db`), settings in `config.json` next to it. Set `TU_DATA` to use a different data file. Colors are turned off when output is piped, when `NO_COLOR` is set, or with `--no-color`.

Shell completion (including IDs of tracked dates) is available through `tu completion bash|zsh|fish`.

## Project Structure

The codebase is organized to keep responsibilities strictly separated:

- `main.go:` The entry point. It acts as the central router, initializing the storage manager and injecting it into the CLI application.

- `cli/:` Contains the command-line interface logic. It receives the storage dependencies and handles user input/output without needing to know how the underlying data is stored.

- `db/:` The storage manager. It handles the internal logic for reading from and writing to the local JSON file.

- `models/:` Defines the core data structures (like the tracked date format) used across the application to ensure consistency.
