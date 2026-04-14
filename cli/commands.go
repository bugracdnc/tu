package cli

import (
	"log"
	"strings"
	"tu/db"

	"github.com/spf13/cobra"
)

var storage *db.Storage

var rootCmd = &cobra.Command{
	Use:   "tu",
	Short: "Keep track of time until an event",
	Run: func(cmd *cobra.Command, args []string) {
		printList(*storage)
	},
}

func Execute(_storage *db.Storage) {
	storage = _storage
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	Commands := []cobra.Command{
		{
			Use:     "track",
			Short:   "Track a date",
			Example: "tu track 2026-05-22\ntu track 2026-05-22 16:13:22",
			Run: func(cmd *cobra.Command, args []string) {
				if err := addToList(storage, strings.Join(args, " ")); err != nil {
					log.Println("Error: ", err.Error())
				}
			},
		},
		{
			Use:     "list",
			Short:   "Show the list",
			Example: "tu list",
			Run: func(cmd *cobra.Command, args []string) {
				printList(*storage)
			},
		},
	}

	for _, Command := range Commands {
		rootCmd.AddCommand(&Command)
	}

}
