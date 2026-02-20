package cmd

import (
	"fmt"
	"io"
	"os"
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

	// Query previous status before inserting
	var prevStatus string
	err = sqlDB.QueryRow(`SELECT status FROM statuses WHERE scenario_id = ? ORDER BY changed_at DESC, id DESC LIMIT 1`, id).Scan(&prevStatus)
	if err != nil {
		prevStatus = ""
	}

	_, err = sqlDB.Exec(`INSERT INTO statuses (scenario_id, status) VALUES (?, ?)`, id, status)
	if err != nil {
		return fmt.Errorf("inserting status: %w", err)
	}

	ui.StatusConfirm(w, id, prevStatus, status)
	return nil
}

func RunStatusReport(w io.Writer) error {
	if _, err := os.Stat("fts"); os.IsNotExist(err) {
		return fmt.Errorf("run `ft init` first")
	}

	sqlDB, err := db.Open("fts/ft.db")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer sqlDB.Close()

	var count int
	err = sqlDB.QueryRow(`SELECT COUNT(*) FROM scenarios`).Scan(&count)
	if err != nil {
		return fmt.Errorf("counting scenarios: %w", err)
	}

	fmt.Fprintf(w, "Scenarios: %d\n", count)

	if count == 0 {
		return nil
	}

	rows, err := sqlDB.Query(`
		SELECT COALESCE(
			(SELECT status FROM statuses WHERE scenario_id = s.id ORDER BY changed_at DESC, id DESC LIMIT 1),
			'no-activity'
		) AS current_status, COUNT(*) AS cnt
		FROM scenarios s
		GROUP BY current_status
		ORDER BY CASE WHEN current_status = 'no-activity' THEN 1 ELSE 0 END, cnt DESC
	`)
	if err != nil {
		return fmt.Errorf("querying status counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var cnt int
		if err := rows.Scan(&status, &cnt); err != nil {
			return fmt.Errorf("scanning status row: %w", err)
		}
		if cnt > 0 {
			fmt.Fprintf(w, "  %s: %d\n", status, cnt)
		}
	}

	return rows.Err()
}
