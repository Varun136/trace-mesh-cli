package cmd

import (
	"errors"
	"fmt"
	"os"
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
	info, err := os.Lstat(activePath)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("Fatal: no active task found; start a task first")
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", activePath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("Fatal: %s is not an OS symlink; text pointer fallback is unsupported", activePath)
	}

	contents, err := readTaskFile(activePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", activePath, err)
	}
	if !strings.Contains(string(contents), "\n## Implementation Log\n") {
		return fmt.Errorf("Fatal: active task %s is missing the ## Implementation Log header", activePath)
	}

	entry := fmt.Sprintf("- %s: %s\n", time.Now().Format(time.RFC3339), text)
	if len(contents)+len(entry) > maxTaskFileSize {
		return fmt.Errorf("Fatal: note would make %s exceed maximum size of %d bytes", activePath, maxTaskFileSize)
	}

	file, err := os.OpenFile(activePath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", activePath, err)
	}
	defer file.Close()

	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("append note to %s: %w", activePath, err)
	}
	return nil
}
