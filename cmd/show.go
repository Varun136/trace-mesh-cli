package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current active task",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runShow()
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow() error {
	if err := ensureGitRepository(); err != nil {
		return err
	}
	activePath := filepath.Join(".tracemesh", "active.md")
	if _, err := activeTaskID(activePath); err != nil {
		return err
	}
	contents, err := readTaskFile(activePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("Fatal: active task symlink is broken")
		}
		return fmt.Errorf("read %s: %w", activePath, err)
	}
	fmt.Print(string(contents))
	return nil
}
