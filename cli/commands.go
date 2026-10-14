package cli

import (
	"fmt"
	"os"
	"strings"
	"tu/db"
	"tu/gcal"

	"github.com/spf13/cobra"
)

var storage *db.Storage
var config *db.Config
var gstate *gcal.State

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

func Execute(_storage *db.Storage, _config *db.Config, _gstate *gcal.State) {
	storage = _storage
	config = _config
	gstate = _gstate
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
	var addLocal bool
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
tu add Standup 09:30 -r daily --local
tu track 2026-05-22 16:13:22`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := addToList(storage, args, addName, addRepeat, addLocal); err != nil {
				return err
			}
			autoPush(storage, config, gstate)
			return nil
		},
	}
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "name of the date (otherwise taken from the arguments)")
	addCmd.Flags().StringVarP(&addRepeat, "repeat", "r", "", "repeat daily, weekly, monthly or yearly")
	addCmd.Flags().BoolVarP(&addLocal, "local", "l", false, "never push this date to Google Calendar")

	var keepGoogle bool
	rmCmd := &cobra.Command{
		Use:     "rm <id|name>...",
		Aliases: []string{"remove", "delete", "del"},
		Short:   "Stop tracking dates",
		Long: `Stop tracking dates. A date can be referenced by (a prefix of) its ID, its name, or a unique part of its name.

Dates that were pushed to Google Calendar are deleted there too, unless --keep-google is given.
Dates pulled from Google are only hidden here and won't be pulled again.`,
		Example:           "tu rm 3f9a1c\ntu rm \"Summer holiday\"\ntu rm exam birthday",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeRefs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := removeFromList(storage, gstate, args, keepGoogle); err != nil {
				return err
			}
			autoPush(storage, config, gstate)
			return nil
		},
	}
	rmCmd.Flags().BoolVar(&keepGoogle, "keep-google", false, "keep the event in Google Calendar")

	var editOpts editOptions
	editCmd := &cobra.Command{
		Use:               "edit <id|name>",
		Aliases:           []string{"e"},
		Short:             "Change the name, date or repetition of a tracked date",
		Example:           "tu edit exam --date \"2026-06-01 10:00\"\ntu edit 3f9a1c --name \"Final exam\"\ntu edit birthday --repeat none",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRefs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := editDate(storage, gstate, args[0], editOpts, cmd.Flags().Changed); err != nil {
				return err
			}
			autoPush(storage, config, gstate)
			return nil
		},
	}
	editCmd.Flags().StringVarP(&editOpts.name, "name", "n", "", "new name")
	editCmd.Flags().StringVarP(&editOpts.date, "date", "d", "", "new date")
	editCmd.Flags().StringVarP(&editOpts.repeat, "repeat", "r", "", "repeat daily, weekly, monthly, yearly or none")
	editCmd.Flags().BoolVarP(&editOpts.localOnly, "local", "l", false, "keep out of Google Calendar (--local=false to push it)")

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
		Use:     "config [key] [value]",
		Short:   "Show or change settings",
		Long:    "Show or change settings.\n\nSettings:\n" + settingsHelp(),
		Example: "tu config\ntu config show-past true\ntu config google-calendars \"primary,Holidays in Turkey\"",
		Args:    cobra.MaximumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				var keys []string
				for _, s := range settings {
					keys = append(keys, s.key+"\t"+s.help)
				}
				return keys, cobra.ShellCompDirectiveNoFileComp
			}
			if s, err := findSetting(args[0]); err == nil && len(args) == 1 {
				return s.values, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return configure(config, args)
		},
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync with Google Calendar",
		Long: `Send new, changed and removed dates to Google Calendar and pull the upcoming
events from it (birthdays, holidays and anything else in your calendars).

Pulled events replace the ones from the previous sync, so they always mirror
what's upcoming in Google. Events that are already tracked here are not
pulled twice.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return syncGoogle(storage, config, gstate)
		},
	}

	googleCmd := &cobra.Command{
		Use:     "google",
		Aliases: []string{"gcal"},
		Short:   "Connect to Google Calendar",
		Long:    "Connect to Google Calendar, then use `tu sync` to push and pull dates.\n\n" + setupHelp,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return googleStatus(storage, config, gstate)
		},
	}

	var loginOpts loginOptions
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Allow tu to use your Google Calendar",
		Long: `Allow tu to use your Google Calendar.

A browser opens to sign in with Google. When it can't send you back to tu by
itself (e.g. you signed in on another device), copy the address the browser
ends up on and paste it into the terminal.

` + setupHelp,
		Example: "tu google login --client-file ~/Downloads/client_secret.json\ntu google login --no-browser",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return googleLogin(gstate, loginOpts)
		},
	}
	loginCmd.Flags().StringVar(&loginOpts.clientFile, "client-file", "", "OAuth client JSON downloaded from the Google Cloud console")
	loginCmd.Flags().StringVar(&loginOpts.clientID, "client-id", "", "OAuth client ID")
	loginCmd.Flags().StringVar(&loginOpts.clientSecret, "client-secret", "", "OAuth client secret")
	loginCmd.Flags().BoolVar(&loginOpts.noBrowser, "no-browser", false, "only print the address to open")

	googleCmd.AddCommand(
		loginCmd,
		&cobra.Command{
			Use:   "logout",
			Short: "Disconnect from Google Calendar",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return googleLogout(gstate)
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show the connection and sync state",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return googleStatus(storage, config, gstate)
			},
		},
		&cobra.Command{
			Use:     "calendars",
			Aliases: []string{"cals"},
			Short:   "List your calendars and which ones are synced",
			Args:    cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return googleCalendars(config, gstate)
			},
		},
	)

	rootCmd.AddCommand(listCmd, addCmd, rmCmd, editCmd, showCmd, nextCmd, cleanCmd, configCmd, syncCmd, googleCmd)
}

func settingsHelp() string {
	var b strings.Builder
	for _, s := range settings {
		fmt.Fprintf(&b, "  %-18s %s\n", s.key, s.help)
	}
	return strings.TrimRight(b.String(), "\n")
}
