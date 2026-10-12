package cli

import (
	"fmt"
	"os"
	"strings"
	"tu/db"

	"github.com/spf13/cobra"
)

var storage *db.Storage
var config *db.Config

var noColor bool

var rootCmd = &cobra.Command{
	Use:   "tu",
	Short: "Keep track of time until an event",
	Long: `tu keeps track of the time until (and since) the dates you care about.

Running tu without a command lists your upcoming dates.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupColor(noColor)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return printList(storage, config, rootListOpts)
	},
}

func Execute(_storage *db.Storage, _config *db.Config) {
	storage = _storage
	config = _config
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

var rootListOpts, listOpts listOptions

func addListFlags(cmd *cobra.Command, opts *listOptions) {
	cmd.Flags().BoolVarP(&opts.all, "all", "a", false, "include dates that have passed")
	cmd.Flags().BoolVarP(&opts.past, "past", "p", false, "show only dates that have passed")
	cmd.Flags().StringVarP(&opts.within, "within", "w", "", "only show upcoming dates within a span, e.g. 2w or 3mo")
	cmd.Flags().BoolVar(&opts.json, "json", false, "print as JSON")
}

// completeRefs suggests short IDs of tracked dates, described by their names.
func completeRefs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	for _, d := range storage.Read() {
		if strings.HasPrefix(d.ShortID(), toComplete) {
			out = append(out, d.ShortID()+"\t"+d.DisplayName())
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	addListFlags(rootCmd, &rootListOpts)

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "Show the list",
		Example: "tu list\ntu list --all\ntu list --past\ntu list --within 2w",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printList(storage, config, listOpts)
		},
	}
	addListFlags(listCmd, &listOpts)

	var addName, addRepeat string
	addCmd := &cobra.Command{
		Use:     "add [name] <date>",
		Aliases: []string{"track", "new"},
		Short:   "Track a date",
		Long: `Track a date. Everything before the date is used as its name.

Dates can be written as:
  2026-05-22, 22/05/2026, 22.05.2026        a whole day
  2026-05-22 16:00, 22/05/2026 16:00:30      an exact time
  today, tomorrow, friday, next mon          a whole day
  tomorrow 18:00, fri 9am                    an exact time
  18:00                                      the next time the clock shows 18:00
  +3d, in 2 weeks, 1y 2mo, 3 days ago        relative to now`,
		Example: `tu add "Summer holiday" 2026-07-01
tu add Exam friday 9am
tu add Birthday 22/05/1995 --repeat yearly
tu add Standup 09:30 -r daily
tu track 2026-05-22 16:13:22`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return addToList(storage, args, addName, addRepeat)
		},
	}
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "name of the date (otherwise taken from the arguments)")
	addCmd.Flags().StringVarP(&addRepeat, "repeat", "r", "", "repeat daily, weekly, monthly or yearly")

	rmCmd := &cobra.Command{
		Use:               "rm <id|name>...",
		Aliases:           []string{"remove", "delete", "del"},
		Short:             "Stop tracking dates",
		Long:              "Stop tracking dates. A date can be referenced by (a prefix of) its ID, its name, or a unique part of its name.",
		Example:           "tu rm 3f9a1c\ntu rm \"Summer holiday\"\ntu rm exam birthday",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeRefs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeFromList(storage, args)
		},
	}

	var editOpts editOptions
	editCmd := &cobra.Command{
		Use:               "edit <id|name>",
		Aliases:           []string{"e"},
		Short:             "Change the name, date or repetition of a tracked date",
		Example:           "tu edit exam --date \"2026-06-01 10:00\"\ntu edit 3f9a1c --name \"Final exam\"\ntu edit birthday --repeat none",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRefs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return editDate(storage, args[0], editOpts, cmd.Flags().Changed)
		},
	}
	editCmd.Flags().StringVarP(&editOpts.name, "name", "n", "", "new name")
	editCmd.Flags().StringVarP(&editOpts.date, "date", "d", "", "new date")
	editCmd.Flags().StringVarP(&editOpts.repeat, "repeat", "r", "", "repeat daily, weekly, monthly, yearly or none")

	showCmd := &cobra.Command{
		Use:               "show <id|name>",
		Aliases:           []string{"info"},
		Short:             "Show the full time left for a date",
		Example:           "tu show birthday",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRefs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showDate(storage, args[0])
		},
	}

	var nextCount int
	nextCmd := &cobra.Command{
		Use:     "next",
		Short:   "Print the next upcoming date(s) in a compact form",
		Long:    "Print the next upcoming date(s) in a compact form, handy for status bars and shell prompts.",
		Example: "tu next\ntu next -c 3",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printNext(storage, nextCount)
		},
	}
	nextCmd.Flags().IntVarP(&nextCount, "count", "c", 1, "how many dates to print")

	var dryRun bool
	cleanCmd := &cobra.Command{
		Use:     "clean",
		Aliases: []string{"prune"},
		Short:   "Remove all dates that have passed",
		Example: "tu clean --dry-run\ntu clean",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanPast(storage, dryRun)
		},
	}
	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "only show what would be removed")

	configCmd := &cobra.Command{
		Use:   "config [key] [value]",
		Short: "Show or change settings",
		Long: `Show or change settings.

Settings:
  show-past   include passed dates in the default list (true/false)`,
		Example: "tu config\ntu config show-past true",
		Args:    cobra.MaximumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return configKeys, cobra.ShellCompDirectiveNoFileComp
			}
			return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return configure(config, args)
		},
	}

	rootCmd.AddCommand(listCmd, addCmd, rmCmd, editCmd, showCmd, nextCmd, cleanCmd, configCmd)
}
