package cmd

import (
	_ "embed"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var agentInstructionsCmd = &cobra.Command{
	Use:   "agent-instructions",
	Short: "Print instructions for an AI agent using ft",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunAgentInstructions(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(agentInstructionsCmd)
}

func RunAgentInstructions(w io.Writer) error {
	_, err := fmt.Fprint(w, agentInstructionsText)
	return err
}

//go:embed agent_instructions.md
var agentInstructionsText string
