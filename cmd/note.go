package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note [text]",
	Short: "Append a timestamped note to the active task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNote(args[0])
	},
}

func init() {
	rootCmd.AddCommand(noteCmd)
}

func runNote(text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("Fatal: note text cannot be empty")
	}
	if err := ensureGitRepository(); err != nil {
		return err
	}
	if _, err := readConfig(); err != nil {
		return err
	}

	activePath := filepath.Join(".tracemesh", "active.md")
	if _, err := activeTaskID(activePath); err != nil {
		if errors.Is(err, errNoActiveTask) {
			return errors.New("Fatal: no active task found; start a task first")
		}
		return err
	}

	contents, err := readActiveTaskFile(activePath)
	if err != nil {
		if errors.Is(err, errNoActiveTask) {
			return errors.New("Fatal: no active task found; start a task first")
		}
		return fmt.Errorf("read %s: %w", activePath, err)
	}
	if !strings.Contains(string(contents), "\n## Implementation Log\n") {
		return fmt.Errorf("Fatal: active task %s is missing the ## Implementation Log header", activePath)
	}

	entry := fmt.Sprintf("- %s: %s\n", time.Now().Format(time.RFC3339), text)
	if err := appendActiveTaskFile(activePath, entry); err != nil {
		return err
	}
	return nil
}
