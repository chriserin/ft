package cmd

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/chriserin/ft/internal/db"
	"github.com/chriserin/ft/internal/ui"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Scan fts/ for .ft files and register new ones",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunSync(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func RunSync(w io.Writer) error {
	if _, err := os.Stat("fts"); os.IsNotExist(err) {
		return fmt.Errorf("run `ft init` first")
	}

	sqlDB, err := db.Open("fts/ft.db")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer sqlDB.Close()

	matches, err := filepath.Glob("fts/*.ft")
	if err != nil {
		return fmt.Errorf("scanning fts/: %w", err)
	}
	sort.Strings(matches)

	count := 0
	for _, path := range matches {
		var id int
		err := sqlDB.QueryRow(`SELECT id FROM files WHERE file_path = ?`, path).Scan(&id)
		if err == sql.ErrNoRows {
			_, err = sqlDB.Exec(`INSERT INTO files (file_path) VALUES (?)`, path)
			if err != nil {
				return fmt.Errorf("inserting %s: %w", path, err)
			}
			ui.NewLine(w, path)
		} else if err != nil {
			return fmt.Errorf("querying %s: %w", path, err)
		} else {
			ui.TrkLine(w, path)
		}
		count++
	}

	ui.SummaryLine(w, count)
	return nil
}
