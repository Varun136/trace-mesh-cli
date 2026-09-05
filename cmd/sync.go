package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize the active task with the current Git branch",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync()
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync() error {
	if err := ensureGitRepository(); err != nil {
		return err
	}
	branch, err := currentBranch()
	if err != nil {
		return err
	}
	cfg, err := readConfig()
	if err != nil {
		return err
	}

	configured := cfg.Branches[branch]
	valid := make([]string, 0, len(configured))
	for _, id := range configured {
		taskPath := filepath.Join(".tracemesh", "tasks", id+".md")
		if _, err := os.Stat(taskPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Printf("Warning: Task %s not found, removed from branch context\n", id)
				continue
			}
			return fmt.Errorf("inspect task %s: %w", id, err)
		}
		valid = append(valid, id)
	}

	if len(valid) != len(configured) {
		cfg.Branches[branch] = valid
		if err := writeConfig(cfg); err != nil {
			return err
		}
	}

	activePath := filepath.Join(".tracemesh", "active.md")
	if err := os.Remove(activePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", activePath, err)
	}
	if len(valid) == 0 {
		return nil
	}

	target := filepath.ToSlash(filepath.Join("tasks", valid[len(valid)-1]+".md"))
	if err := os.Symlink(target, activePath); err != nil {
		return fmt.Errorf("create %s symlink: %w", activePath, err)
	}
	return nil
}
