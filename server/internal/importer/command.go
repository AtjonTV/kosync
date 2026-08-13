//
// File:        internal/importer/command.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package importer

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/config"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// NewCommand builds the "import-legacy" command.
func NewCommand(app core.App, conf *config.Config) *cobra.Command {
	options := Options{}

	command := &cobra.Command{
		Use:   "import-legacy",
		Short: "Imports the data of a legacy KOsync 1 SQLite database",
		Long: "Imports users, documents and their history from a legacy KOsync 1 database.\n\n" +
			"Legacy users become KOReader credentials. Because a credential has to belong to an\n" +
			"account, one account per legacy user is created unless --owner-email is given.\n" +
			"The generated passwords are printed once and cannot be recovered afterwards.",
		SilenceUsage: true,
		RunE: func(command *cobra.Command, args []string) error {
			// Only "serve" applies the migrations on its own, and an import into
			// a fresh data directory has to find the collections in place.
			if err := app.RunAllMigrations(); err != nil {
				return fmt.Errorf("failed to prepare the database: %w", err)
			}

			report, err := Run(app, options)
			if err != nil {
				return err
			}

			printReport(command, report, options)

			if options.DryRun {
				return nil
			}

			// The import queued a statistics recomputation for every day it
			// touched. Running it here means the dashboard is correct the
			// moment the server starts.
			handled, err := analytics.NewWorker(app, conf).DrainAll()
			if err != nil {
				return fmt.Errorf("import finished, but computing the statistics failed: %w", err)
			}
			command.Printf("Computed %d days of reading statistics.\n", handled)

			return nil
		},
	}

	command.Flags().StringVar(&options.File, "file", "./kosync.db", "path to the legacy KOsync database")
	command.Flags().StringVar(&options.OwnerEmail, "owner-email", "",
		"attach every imported credential and document to this existing account instead of creating one account per legacy user")
	command.Flags().BoolVar(&options.IncludeDeleted, "include-deleted", false, "also import rows the legacy server marked as deleted")
	command.Flags().BoolVar(&options.DryRun, "dry-run", false, "report what would be imported without writing anything")

	return command
}

// maxSkippedLines is how many distinct reasons are listed before the rest is
// summarised. A legacy database can hold thousands of history entries of
// deleted documents, and printing one line each helps nobody.
const maxSkippedLines = 15

// printSkipped lists what the import passed over, collapsing repetitions.
func printSkipped(command *cobra.Command, skipped []string) {
	if len(skipped) == 0 {
		return
	}

	counts := map[string]int{}
	order := []string{}
	for _, entry := range skipped {
		if _, seen := counts[entry]; !seen {
			order = append(order, entry)
		}
		counts[entry]++
	}

	sort.SliceStable(order, func(i, j int) bool {
		return counts[order[i]] > counts[order[j]]
	})

	command.Printf("\nSkipped %d entries:\n", len(skipped))
	for i, entry := range order {
		if i == maxSkippedLines {
			command.Printf("  ... and %d more kinds of entry\n", len(order)-maxSkippedLines)
			break
		}
		if counts[entry] > 1 {
			command.Printf("  - %s (%d times)\n", entry, counts[entry])
		} else {
			command.Printf("  - %s\n", entry)
		}
	}
}

// printReport writes the outcome of an import to the command output.
func printReport(command *cobra.Command, report *Report, options Options) {
	if options.DryRun {
		command.Println("Dry run, nothing was written.")
	}

	command.Printf("Credentials: %d\nDocuments:   %d\nHistory:     %d\nAccounts:    %d\n",
		report.Credentials, report.Documents, report.History, len(report.Accounts))

	printSkipped(command, report.Skipped)

	if len(report.Accounts) == 0 {
		return
	}

	command.Println("\nAccounts created for the legacy users.")
	command.Println("Write these down now, the passwords are not stored anywhere and cannot be shown again.")
	command.Println("Every user should sign in, change their address to a real one and set their own password.")
	command.Println()

	writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "LEGACY USER\tEMAIL\tPASSWORD")
	for _, account := range report.Accounts {
		fmt.Fprintf(writer, "%s\t%s\t%s\n", account.LegacyUsername, account.Email, account.Password)
	}
	_ = writer.Flush()
}
