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

	store, err := db.OpenProjectStore()
	if err != nil {
		return err
	}
	defer store.Close()

	if !store.ScenarioExists(id) {
		return fmt.Errorf("scenario %d not found", id)
	}

	links, err := store.TestLinks(id)
	if err != nil {
		return fmt.Errorf("querying test links: %w", err)
	}

	for _, l := range links {
		name, err := testFuncName(l.FilePath, l.LineNumber)
		if err != nil {
			return fmt.Errorf("reading %s: %w", l.FilePath, err)
		}
		if name != "" {
			fmt.Fprintf(w, "  %s:%d %s\n", l.FilePath, l.LineNumber, name)
		} else {
			fmt.Fprintf(w, "  %s:%d\n", l.FilePath, l.LineNumber)
		}
	}

	return nil
}

func testFuncName(filePath string, commentLine int) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
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
			return name, nil
		}
		return "", nil
	}
	return "", scanner.Err()
}
