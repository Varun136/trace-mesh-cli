package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type listedTask struct {
	number   int
	id       string
	title    string
	path     string
	archived bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Tracemesh tasks and their titles",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList() error {
	if err := ensureGitRepository(); err != nil {
		return err
	}
	if _, err := readConfig(); err != nil {
		return err
	}

	var tasks []listedTask
	for _, directory := range []string{"tasks", "archive"} {
		root := filepath.Join(".tracemesh", directory)
		archived := directory == "archive"
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("inspect %s: %w", root, err)
		}

		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				return nil
			}
			match := taskFilePattern.FindStringSubmatch(entry.Name())
			if len(match) == 0 {
				return nil
			}
			contents, err := readTaskFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			title, err := taskTitle(contents)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			number, err := strconv.Atoi(match[1])
			if err != nil {
				return fmt.Errorf("parse task ID: %w", err)
			}
			tasks = append(tasks, listedTask{number: number, id: "TM-" + match[1], title: title, path: path, archived: archived})
			return nil
		})
		if err != nil {
			return fmt.Errorf("scan %s: %w", root, err)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].number == tasks[j].number {
			return tasks[i].path < tasks[j].path
		}
		return tasks[i].number < tasks[j].number
	})
	fmt.Println("Active tasks:")
	printTaskGroup(tasks, false)
	fmt.Println("\nArchived tasks:")
	printTaskGroup(tasks, true)
	return nil
}

func printTaskGroup(tasks []listedTask, archived bool) {
	for _, task := range tasks {
		if task.archived {
			if !archived {
				continue
			}
		} else if archived {
			continue
		}
		fmt.Printf("%s: %s\n", task.id, task.title)
	}
}

func taskTitle(contents []byte) (string, error) {
	firstLine := strings.SplitN(string(contents), "\n", 2)[0]
	if !strings.HasPrefix(firstLine, "# ") || strings.TrimSpace(strings.TrimPrefix(firstLine, "# ")) == "" {
		return "", fmt.Errorf("missing Markdown H1 title")
	}
	return strings.TrimPrefix(firstLine, "# "), nil
}
