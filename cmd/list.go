package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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
  ft list --not removed --not done     Exclude multiple statuses`,
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
	for _, ex := range excludes {
		if status == ex {
			return false
		}
	}

	if len(includes) == 0 {
		return true
	}

	for _, inc := range includes {
		if status == inc {
			return true
		}
	}
	return false
}

func RunList(w io.Writer, includes []string, excludes []string) error {
	if _, err := os.Stat("fts"); os.IsNotExist(err) {
		return fmt.Errorf("run `ft init` first")
	}

	sqlDB, err := db.Open("fts/ft.db")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer sqlDB.Close()

	rows, err := sqlDB.Query(`
		SELECT s.id, f.file_path, s.name,
			COALESCE(
				(SELECT status FROM statuses WHERE scenario_id = s.id ORDER BY changed_at DESC, id DESC LIMIT 1),
				'no-activity'
			) AS current_status
		FROM scenarios s
		JOIN files f ON s.file_id = f.id
		ORDER BY f.file_path, s.id
	`)
	if err != nil {
		return fmt.Errorf("querying scenarios: %w", err)
	}
	defer rows.Close()

	var results []listRow
	for rows.Next() {
		var r listRow
		var filePath string
		if err := rows.Scan(&r.id, &filePath, &r.name, &r.status); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}
		r.fileName = filepath.Base(filePath)

		if (len(includes) > 0 || len(excludes) > 0) && !matchesFilter(r.status, includes, excludes) {
			continue
		}

		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating rows: %w", err)
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
