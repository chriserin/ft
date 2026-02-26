package cmd

import (
	"bufio"
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
		cmd.SilenceUsage = true
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

	for rows.Next() {
		var filePath string
		var lineNumber int
		if err := rows.Scan(&filePath, &lineNumber); err != nil {
			continue
		}
		name := testFuncName(filePath, lineNumber)
		if name != "" {
			fmt.Fprintf(w, "  %s:%d %s\n", filePath, lineNumber, name)
		} else {
			fmt.Fprintf(w, "  %s:%d\n", filePath, lineNumber)
		}
	}

	return nil
}

func testFuncName(filePath string, commentLine int) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= commentLine {
			continue
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "func Test") {
			name := strings.TrimPrefix(line, "func ")
			if idx := strings.IndexByte(name, '('); idx != -1 {
				name = name[:idx]
			}
			return name
		}
		return ""
	}
	return ""
}
