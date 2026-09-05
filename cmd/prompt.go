package cmd

import "github.com/spf13/cobra"

var promptCmd = &cobra.Command{
	Use:   "tm-prompt",
	Short: "Show the Tracemesh prompt for AI-agent instruction files",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		printTracemeshPrompt()
	},
}

func init() {
	rootCmd.AddCommand(promptCmd)
}

func printTracemeshPrompt() {
	println(tracemeshAgentPrompt)
}
