package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chriserin/ft/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ft in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunInit(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func RunInit(w io.Writer) error {
	// fts/ directory
	ftsExists := db.DataDirExists()
	if err := db.EnsureDataDir(); err != nil {
		return fmt.Errorf("creating fts directory: %w", err)
	}
	if ftsExists {
		fmt.Fprintln(w, "fts/ already exists")
	} else {
		fmt.Fprintln(w, "fts/ created")
	}

	// database
	dbExists := db.FileExists()
	store, err := db.OpenProjectStore()
	if err != nil {
		return err
	}
	store.Close()
	if dbExists {
		fmt.Fprintln(w, "fts/ft.db already exists")
	} else {
		fmt.Fprintln(w, "fts/ft.db created")
	}

	// statuses file — git-tracked, unlike ft.db, so never added to .gitignore
	statusesExists := db.StatusesFileExists()
	if err := db.EnsureStatusesFile(); err != nil {
		return fmt.Errorf("creating statuses file: %w", err)
	}
	if statusesExists {
		fmt.Fprintln(w, "fts/statuses.csv already exists")
	} else {
		fmt.Fprintln(w, "fts/statuses.csv created")
	}

	// gitignore
	msgs, err := ensureGitignore()
	if err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}
	for _, msg := range msgs {
		fmt.Fprintln(w, msg)
	}

	return nil
}

func ensureGitignore() ([]string, error) {
	entry := db.Path()

	data, err := os.ReadFile(".gitignore")
	if os.IsNotExist(err) {
		if err := os.WriteFile(".gitignore", []byte(entry+"\n"), 0o644); err != nil {
			return nil, err
		}
		return []string{".gitignore created", "fts/ft.db added to .gitignore"}, nil
	}
	if err != nil {
		return nil, err
	}

	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		if strings.TrimSpace(line) == entry {
			return []string{"fts/ft.db already in .gitignore"}, nil
		}
	}

	content := string(data)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"

	if err := os.WriteFile(".gitignore", []byte(content), 0o644); err != nil {
		return nil, err
	}
	return []string{"fts/ft.db added to .gitignore"}, nil
}
