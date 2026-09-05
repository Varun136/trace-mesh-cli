package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:   "switch [task ID]",
	Short: "Make an active task the current branch's task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSwitch(args[0])
	},
}

func init() {
	rootCmd.AddCommand(switchCmd)
}

func runSwitch(taskID string) error {
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

	if !taskIDPattern.MatchString(taskID) {
		return fmt.Errorf("Error: invalid task ID %s.", taskID)
	}
	if !activeTaskExists(taskID) {
		if archivedTaskExists(taskID) {
			return fmt.Errorf("Error: Task %s is archived and cannot be activated.", taskID)
		}
		return fmt.Errorf("Error: Task %s does not exist.", taskID)
	}

	warnAboutOtherBranches(cfg, branch, taskID)

	current := cfg.Branches[branch]
	reordered := make([]string, 0, len(current)+1)
	for _, id := range current {
		if id != taskID {
			reordered = append(reordered, id)
		}
	}
	reordered = append(reordered, taskID)
	cfg.Branches[branch] = reordered

	if err := writeConfig(cfg); err != nil {
		return err
	}
	if err := activateTask(taskID); err != nil {
		return err
	}

	fmt.Printf("Switched active task to %s on branch %s.\n", taskID, branch)
	return nil
}

func activeTaskExists(taskID string) bool {
	return taskFileExists(filepath.Join(".tracemesh", "tasks", taskID+".md"))
}

func archivedTaskExists(taskID string) bool {
	return taskFileExists(filepath.Join(".tracemesh", "archive", taskID+".md"))
}

func taskFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func warnAboutOtherBranches(cfg *config, currentBranch, taskID string) {
	branches := make([]string, 0)
	for branch, taskIDs := range cfg.Branches {
		if branch == currentBranch {
			continue
		}
		for _, id := range taskIDs {
			if id == taskID {
				branches = append(branches, branch)
				break
			}
		}
	}
	sort.Strings(branches)
	for _, branch := range branches {
		fmt.Printf("Warning: Task %s is associated with branch %s; consider switching branches.\n", taskID, branch)
	}
}
