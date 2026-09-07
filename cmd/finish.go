package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var errNoActiveTask = errors.New("Fatal: no active task found")

var finishCmd = &cobra.Command{
	Use:   "finish",
	Short: "Archive the current active task",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFinish()
	},
}

func init() {
	rootCmd.AddCommand(finishCmd)
}

func runFinish() error {
	return withConfigLock(runFinishLocked)
}

func runFinishLocked() error {
	if err := ensureGitRepository(); err != nil {
		return err
	}
	branch, err := currentBranch()
	if err != nil {
		return err
	}
	cfg, err := readConfigUnlocked()
	if err != nil {
		return err
	}

	activePath := filepath.Join(".tracemesh", "active.md")
	taskID, err := activeTaskID(activePath)
	if err != nil {
		return err
	}
	taskPath := filepath.Join(".tracemesh", "tasks", taskID+".md")
	archivePath := filepath.Join(".tracemesh", "archive", taskID+".md")
	if _, err := os.Stat(taskPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("Fatal: active task symlink is broken")
		}
		return fmt.Errorf("inspect active task %s: %w", taskID, err)
	}
	if _, err := os.Stat(archivePath); err == nil {
		return fmt.Errorf("Fatal: archived task file already exists: %s", archivePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect archive destination: %w", err)
	}

	if err := os.Rename(taskPath, archivePath); err != nil {
		return fmt.Errorf("archive task %s: %w", taskID, err)
	}

	remaining := make([]string, 0, len(cfg.Branches[branch]))
	for _, id := range cfg.Branches[branch] {
		if id != taskID {
			remaining = append(remaining, id)
		}
	}
	cfg.Branches[branch] = remaining
	if err := writeConfigUnlocked(cfg); err != nil {
		return err
	}
	if err := os.Remove(activePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear %s: %w", activePath, err)
	}

	fmt.Printf("Finished %s on branch %s.\n", taskID, branch)
	return nil
}

func activeTaskID(activePath string) (string, error) {
	info, err := os.Lstat(activePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", errNoActiveTask
	}
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", activePath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("Fatal: %s is not an OS symlink", activePath)
	}

	target, err := os.Readlink(activePath)
	if err != nil {
		return "", fmt.Errorf("read %s symlink: %w", activePath, err)
	}
	target = filepath.ToSlash(filepath.Clean(target))
	if !strings.HasPrefix(target, "tasks/") || strings.Count(strings.TrimPrefix(target, "tasks/"), "/") != 0 {
		return "", fmt.Errorf("Fatal: %s does not point to an active task", activePath)
	}

	name := filepath.Base(target)
	if !strings.HasSuffix(name, ".md") {
		return "", fmt.Errorf("Fatal: %s does not point to an active task", activePath)
	}
	taskID := strings.TrimSuffix(name, ".md")
	if !taskIDPattern.MatchString(taskID) {
		return "", fmt.Errorf("Fatal: invalid active task ID %s", taskID)
	}
	return taskID, nil
}
