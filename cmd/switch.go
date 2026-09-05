package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/spf13/cobra"
)

var switchTaskIDPattern = regexp.MustCompile(`^TM-\d+$`)

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

	if !switchTaskIDPattern.MatchString(taskID) || !activeTaskExists(taskID) {
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
	info, err := os.Stat(filepath.Join(".tracemesh", "tasks", taskID+".md"))
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
