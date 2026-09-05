package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current branch and active task",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus() error {
	if err := ensureGitRepository(); err != nil {
		return err
	}
	branch, err := currentBranch()
	if err != nil {
		return err
	}

	fmt.Printf("Branch: %s\n", branch)
	activePath := filepath.Join(".tracemesh", "active.md")
	taskID, err := activeTaskID(activePath)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("Active task: none")
		return nil
	}
	if err != nil {
		if _, statErr := os.Lstat(activePath); errors.Is(statErr, os.ErrNotExist) {
			fmt.Println("Active task: none")
			return nil
		}
		return err
	}

	contents, err := os.ReadFile(activePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", activePath, err)
	}
	title, err := taskTitle(contents)
	if err != nil {
		return fmt.Errorf("parse active task %s: %w", taskID, err)
	}
	fmt.Printf("Active task: %s - %s\n", taskID, title)
	return nil
}
