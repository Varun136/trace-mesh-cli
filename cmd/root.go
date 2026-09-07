package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	version  string
	commit   string
	date     string
)

var rootCmd = &cobra.Command{
	Use:           "tm",
	Short:         "tm is the Tracemesh CLI",
	Long:          "tm is the low-friction command line interface for Tracemesh workflows.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .tracemesh/config.json)")
	rootCmd.Version = buildVersion()
}

func buildVersion() string {
	v := version
	if v == "" {
		v = "dev"
	}
	if commit != "" {
		v += " (" + commit + ")"
	}
	if date != "" {
		v += " built " + date
	}
	return v
}
