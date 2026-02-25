package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/chriserin/ft/internal/db"
	"github.com/spf13/cobra"
)

var testsCmd = &cobra.Command{
	Use:   "tests <id>",
	Short: "List test files linked to a scenario",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunTests(cmd.OutOrStdout(), args[0])
	},
}

func init() {
	rootCmd.AddCommand(testsCmd)
}

func RunTests(w io.Writer, rawID string) error {
	rawID = strings.TrimPrefix(rawID, "@ft:")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid scenario ID: %s", rawID)
	}

	if _, err := os.Stat("fts"); os.IsNotExist(err) {
		return fmt.Errorf("run `ft init` first")
	}

	sqlDB, err := db.Open("fts/ft.db")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer sqlDB.Close()

	var existingID int64
	err = sqlDB.QueryRow(`SELECT id FROM scenarios WHERE id = ?`, id).Scan(&existingID)
	if err != nil {
		return fmt.Errorf("scenario %d not found", id)
	}

	rows, err := sqlDB.Query(`SELECT file_path, line_number FROM test_links WHERE scenario_id = ? ORDER BY file_path, line_number`, id)
	if err != nil {
		return fmt.Errorf("querying test links: %w", err)
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var filePath string
		var lineNumber int
		if err := rows.Scan(&filePath, &lineNumber); err != nil {
			continue
		}
		fmt.Fprintf(w, "  %s:%d\n", filePath, lineNumber)
		found = true
	}

	if !found {
		fmt.Fprintf(w, "no linked tests for @ft:%d\n", id)
	}

	return nil
}
