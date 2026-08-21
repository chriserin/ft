package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const Version = "0.1.2"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ft version",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunVersion(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func RunVersion(w io.Writer) error {
	fmt.Fprintln(w, Version)
	return nil
}
