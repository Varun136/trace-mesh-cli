package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var taskIDPattern = regexp.MustCompile(`^TM-(\d+)$`)
var taskFilePattern = regexp.MustCompile(`^TM-(\d+)(?:\.md)?$`)

const taskTemplate = "# %s\n\n**ID:** %s\n**Date:** %s\n**Status:** In Progress\n\n## Description\n%s\n\n## Implementation Log\n"

var startCmd = &cobra.Command{
	Use:   "start [task title]",
	Short: "Create and activate a task for the current Git branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStart(args[0])
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(title string) error {
	return withConfigLock(func() error {
		return runStartLocked(title)
	})
}

func runStartLocked(title string) error {
	if strings.TrimSpace(title) == "" {
		return errors.New("Fatal: task title cannot be empty")
	}
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
	if err := ensureActiveIsAvailable(); err != nil {
		return err
	}

	description, err := promptTaskDescription(title)
	if err != nil {
		return err
	}

	next, err := nextTaskID()
	if err != nil {
		return err
	}
	taskPath := filepath.Join(".tracemesh", "tasks", next+".md")
	contents := fmt.Sprintf(taskTemplate, title, next, time.Now().Format("2006-01-02"), description)
	if err := writeNewFile(taskPath, []byte(contents)); err != nil {
		return err
	}

	cfg.Branches[branch] = append(cfg.Branches[branch], next)
	if err := writeConfigUnlocked(cfg); err != nil {
		return err
	}
	if err := activateTask(next); err != nil {
		return err
	}

	fmt.Printf("Started %s on branch %s.\n", next, branch)
	return nil
}

func promptTaskDescription(title string) (string, error) {
	fmt.Print("Description (press Enter to use the task title): ")
	description, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read task description: %w", err)
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return title, nil
	}
	return description, nil
}

func currentBranch() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Fatal: unable to read the current Git branch: %w", err)
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" || branch == "HEAD" {
		return "", errors.New("Fatal: detached HEAD is not supported; checkout a local branch first")
	}
	return branch, nil
}

func ensureActiveIsAvailable() error {
	path := filepath.Join(".tracemesh", "active.md")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("Fatal: an active task already exists; close or archive it before starting another task")
	}
	return errors.New("Fatal: an active task already exists but is not a valid symlink; close or archive it before starting another task")
}

func nextTaskID() (string, error) {
	max := 0
	root := filepath.Join(".tracemesh")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		match := taskFilePattern.FindStringSubmatch(entry.Name())
		if len(match) == 0 {
			return nil
		}
		n, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("parse task ID in %s: %w", path, err)
		}
		if n > max {
			max = n
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan existing tasks: %w", err)
	}
	return fmt.Sprintf("TM-%03d", max+1), nil
}

func writeNewFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func activateTask(id string) error {
	activePath := filepath.Join(".tracemesh", "active.md")
	if err := os.Remove(activePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", activePath, err)
	}
	target := filepath.ToSlash(filepath.Join("tasks", id+".md"))
	if err := os.Symlink(target, activePath); err != nil {
		return fmt.Errorf("create %s symlink: %w", activePath, err)
	}
	return nil
}
