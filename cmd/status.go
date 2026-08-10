package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/chriserin/ft/internal/db"
	"github.com/chriserin/ft/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [<id> <status>]",
	Short: "Show project status or update a scenario's status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return RunStatusReport(cmd.OutOrStdout())
		}
		if len(args) < 2 {
			return fmt.Errorf("usage: ft status <id> <status>")
		}
		return RunStatusUpdate(cmd.OutOrStdout(), args[0], strings.Join(args[1:], " "))
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func RunStatusUpdate(w io.Writer, rawID, status string) error {
	rawID = strings.TrimPrefix(rawID, "@ft:")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid scenario ID: %s", rawID)
	}

	store, err := db.OpenProjectStore()
	if err != nil {
		return err
	}
	defer store.Close()

	if !store.ScenarioExists(id) {
		return fmt.Errorf("scenario %d not found", id)
	}

	// Query previous status before inserting
	prevStatus, err := store.CurrentStatus(id)
	if err != nil {
		prevStatus = ""
	}

	if err := store.InsertStatus(id, status); err != nil {
		return fmt.Errorf("inserting status: %w", err)
	}

	ui.StatusConfirm(w, id, prevStatus, status)
	return nil
}

func RunStatusReport(w io.Writer) error {
	store, err := db.OpenProjectStore()
	if err != nil {
		return err
	}
	defer store.Close()

	count, err := store.CountScenarios()
	if err != nil {
		return fmt.Errorf("counting scenarios: %w", err)
	}

	fmt.Fprintf(w, "Scenarios: %d\n", count)

	if count == 0 {
		return nil
	}

	counts, err := store.StatusCounts()
	if err != nil {
		return fmt.Errorf("querying status counts: %w", err)
	}

	for _, c := range counts {
		if c.Count > 0 {
			fmt.Fprintf(w, "  %s: %d\n", c.Status, c.Count)
		}
	}

	return nil
}
