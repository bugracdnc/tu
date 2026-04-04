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

tu uses standard CLI commands to manage your tracked dates.

```bash
# View available commands and help documentation
tu --help

# List all currently tracked dates and the time remaining
tu list

# Track new dates
tu track "2026-12-31 23:59:59"
```

## Project Structure

The codebase is organized to keep responsibilities strictly separated:

- `main.go:` The entry point. It acts as the central router, initializing the storage manager and injecting it into the CLI application.

- `cli/:` Contains the command-line interface logic. It receives the storage dependencies and handles user input/output without needing to know how the underlying data is stored.

- `db/:` The storage manager. It handles the internal logic for reading from and writing to the local JSON file.

- `models/:` Defines the core data structures (like the tracked date format) used across the application to ensure consistency.
