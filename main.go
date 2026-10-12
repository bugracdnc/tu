package main

import (
	"fmt"
	"os"
	"path/filepath"
	"tu/cli"
	"tu/db"
)

func main() {
	// TU_DATA points at an alternative data file, otherwise use ~/.config/tu/data.db
	jsonFilePath := os.Getenv("TU_DATA")
	if jsonFilePath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: Cannot find user config directory")
			os.Exit(1)
		}
		jsonFilePath = filepath.Join(configDir, "tu", "data.db")
	}

	TuDateStorage := db.NewStorage(jsonFilePath)
	if err := TuDateStorage.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "Error: Cannot read storage:", err)
		os.Exit(1)
	}

	config, err := db.LoadConfig(filepath.Join(filepath.Dir(jsonFilePath), "config.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: Cannot read config:", err)
		os.Exit(1)
	}

	cli.Execute(TuDateStorage, config)
}
