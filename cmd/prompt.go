package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var promptCmd = &cobra.Command{
	Use:   "tm-prompt",
	Short: "Show the Tracemesh prompt for AI-agent instruction files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		printTracemeshPrompt()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(promptCmd)
}

func printTracemeshPrompt() {
	fmt.Print(tracemeshAgentPrompt)
}
