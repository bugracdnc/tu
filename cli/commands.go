package cli

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var date string

var rootCmd = &cobra.Command{
	Use:   "tu",
	Short: "Keep track of time until an event",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
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
				fmt.Println(args)
			},
		},
		{
			Use:     "list",
			Short:   "Show the list",
			Example: "tu list",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println(args)
			},
		},
	}

	for _, Command := range Commands {
		rootCmd.AddCommand(&Command)
	}

}
