package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"github.com/chriserin/ft/internal/db"
	"github.com/chriserin/ft/internal/ui"
	"github.com/spf13/cobra"
)

var notStatuses []string

var listCmd = &cobra.Command{
	Use:   "list [status...]",
	Short: "List all tracked scenarios",
	Long: `List all tracked scenarios. Filter by passing status names as arguments.
Use --not to exclude statuses.

Examples:
  ft list                              Show all scenarios
  ft list accepted                     Show only accepted scenarios
  ft list accepted ready               Show accepted and ready scenarios
  ft list --not removed                Show all except removed scenarios
  ft list ready --not no-activity      Show ready, excluding no-activity
  ft list --not removed --not done     Exclude multiple statuses
  ft list tested                       Show only scenarios with linked tests
  ft list --not tested                 Show only scenarios without linked tests
  ft list ready --not tested           Show ready scenarios missing tests`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunList(cmd.OutOrStdout(), args, notStatuses)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringArrayVar(&notStatuses, "not", nil, "exclude scenarios with this status (repeatable)")
}

type listRow struct {
	id       int64
	fileName string
	name     string
	status   string
}

func matchesFilter(status string, includes []string, excludes []string) bool {
	if slices.Contains(excludes, status) {
		return false
	}

	if len(includes) == 0 {
		return true
	}

	return slices.Contains(includes, status)
}

func extractVirtual(filters []string, name string) ([]string, bool) {
	var remaining []string
	found := false
	for _, f := range filters {
		if f == name {
			found = true
		} else {
			remaining = append(remaining, f)
		}
	}
	return remaining, found
}

func RunList(w io.Writer, includes []string, excludes []string) error {
	store, err := db.OpenProjectStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Extract virtual "tested" filter
	includes, requireTested := extractVirtual(includes, "tested")
	excludes, excludeTested := extractVirtual(excludes, "tested")

	rows, err := store.ListScenarios()
	if err != nil {
		return fmt.Errorf("querying scenarios: %w", err)
	}

	var results []listRow
	for _, row := range rows {
		r := listRow{
			id:       row.ID,
			fileName: filepath.Base(row.FilePath),
			name:     row.Name,
			status:   row.Status,
		}

		if (len(includes) > 0 || len(excludes) > 0) && !matchesFilter(r.status, includes, excludes) {
			continue
		}

		if requireTested && !store.IsTested(r.id) {
			continue
		}
		if excludeTested && store.IsTested(r.id) {
			continue
		}

		results = append(results, r)
	}

	if len(results) == 0 {
		return nil
	}

	// Compute column widths
	idWidth, fileWidth, nameWidth := 0, 0, 0
	for _, r := range results {
		tag := fmt.Sprintf("@ft:%d", r.id)
		if len(tag) > idWidth {
			idWidth = len(tag)
		}
		if len(r.fileName) > fileWidth {
			fileWidth = len(r.fileName)
		}
		if len(r.name) > nameWidth {
			nameWidth = len(r.name)
		}
	}

	for _, r := range results {
		ui.ListRow(w, r.id, r.fileName, r.name, r.status, idWidth, fileWidth, nameWidth)
	}

	return nil
}
