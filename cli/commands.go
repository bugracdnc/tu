package cli

import (
	"log"
	"strings"
	"tu/db"

	"github.com/spf13/cobra"
)

var storage *db.Storage
var nameFlag string = ""

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
			Aliases: []string{"add"},
			Example: "tu track 2026-05-22\ntu track 2026-05-22 16:13:22",
			Run: func(cmd *cobra.Command, args []string) {
				if err := addToList(storage, strings.Join(args, " "), nameFlag); err != nil {
					log.Println("Error: ", err.Error())
				}
			},
		},
		{
			Use:     "list",
			Short:   "Show the list",
			Aliases: []string{"ls","l"},
			Example: "tu list",
			Run: func(cmd *cobra.Command, args []string) {
				printList(*storage)
			},
		},
		{
			Use:     "edit",
			Short:   "Edit an item in the list",
			Aliases: []string{"e"},
			Example: "tu edit 0",
			Run: func(cmd *cobra.Command, args []string) {
				// TODO: Implement edit functionality
			},
		},
		{
			Use:     "remove",
			Short:   "Remove an item from the list",
			Aliases: []string{"rm"},
			Example: "tu remove 0",
			Run: func(cmd *cobra.Command, args []string) {
				// TODO: Implement remove functionality
			},
		},
	}

	Commands[0].Flags().StringVarP(&nameFlag, "name", "n", "", "Name of the event")

	for _, Command := range Commands {
		
		rootCmd.AddCommand(&Command)
	}

}
