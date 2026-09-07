package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show task history for the current Git branch",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHistory()
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
}

func runHistory() error {
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

	taskIDs := cfg.Branches[branch]
	fmt.Printf("History for branch %s:\n", branch)
	if len(taskIDs) == 0 {
		fmt.Println("(no tasks)")
		return nil
	}

	currentID := taskIDs[len(taskIDs)-1]
	for i := len(taskIDs) - 1; i >= 0; i-- {
		id := taskIDs[i]
		title, found, err := historyTaskTitle(id)
		if err != nil {
			return err
		}
		if !found {
			fmt.Printf("  %s: [task file not found]\n", id)
			continue
		}
		marker := " "
		if id == currentID {
			marker = "*"
		}
		fmt.Printf("%s %s: %s\n", marker, id, title)
	}
	return nil
}

func historyTaskTitle(id string) (string, bool, error) {
	for _, root := range []string{
		filepath.Join(".tracemesh", "tasks"),
		filepath.Join(".tracemesh", "archive"),
	} {
		var foundTitle string
		found := false
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if entry.IsDir() || entry.Name() != id+".md" {
				return nil
			}
			contents, err := readTaskFile(path)
			if err != nil {
				return err
			}
			title, err := taskTitle(contents)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			foundTitle = strings.TrimSpace(title)
			found = true
			return nil
		})
		if err != nil {
			return "", false, fmt.Errorf("scan task history: %w", err)
		}
		if found {
			return foundTitle, true, nil
		}
	}
	return "", false, nil
}
