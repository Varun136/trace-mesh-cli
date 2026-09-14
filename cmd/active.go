package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// activeTargetFor returns the portable relative target recorded in
// .tracemesh/active.md for a task, e.g. "tasks/TM-001.md".
func activeTargetFor(id string) string {
	return filepath.ToSlash(filepath.Join("tasks", id+".md"))
}

// activateTask points .tracemesh/active.md at the given task.
func activateTask(id string) error {
	activePath := filepath.Join(".tracemesh", "active.md")
	if err := os.Remove(activePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", activePath, err)
	}
	target := activeTargetFor(id)
	if err := os.Symlink(target, activePath); err != nil {
		return fmt.Errorf("create %s symlink: %w", activePath, err)
	}
	return nil
}

// ensureActiveIsAvailable reports an error when an active task already exists.
func ensureActiveIsAvailable() error {
	path := filepath.Join(".tracemesh", "active.md")
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	return errors.New("Fatal: an active task already exists; close or archive it before starting another task")
}

// clearActiveTask removes the active task symlink.
func clearActiveTask() error {
	activePath := filepath.Join(".tracemesh", "active.md")
	if err := os.Remove(activePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", activePath, err)
	}
	return nil
}

// closeOnReturn closes file when the calling function returns, reporting a
// close error only if the function would otherwise succeed.
func closeOnReturn(file *os.File, err *error, what string) {
	if cerr := file.Close(); cerr != nil && *err == nil {
		*err = fmt.Errorf("close %s: %w", what, cerr)
	}
}

// activeTaskTarget returns the normalized relative target (e.g.
// "tasks/TM-001.md") recorded in active.md.
func activeTaskTarget(activePath string) (string, error) {
	info, err := os.Lstat(activePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", errNoActiveTask
	}
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", activePath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("Fatal: %s does not point to an active task", activePath)
	}
	target, err := os.Readlink(activePath)
	if err != nil {
		return "", fmt.Errorf("read %s link: %w", activePath, err)
	}
	target = filepath.ToSlash(filepath.Clean(target))
	rest, ok := strings.CutPrefix(target, "tasks/")
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return "", fmt.Errorf("Fatal: %s does not point to an active task", activePath)
	}
	return target, nil
}

// resolveActiveTaskPath maps an active.md symlink to the underlying task file.
func resolveActiveTaskPath(activePath string) (string, error) {
	target, err := activeTaskTarget(activePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(activePath), filepath.FromSlash(target)), nil
}

// activeTaskID extracts the TM-NNN task ID from an active.md symlink.
func activeTaskID(activePath string) (string, error) {
	target, err := activeTaskTarget(activePath)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(target, "tasks/")
	if !strings.HasSuffix(name, ".md") {
		return "", fmt.Errorf("Fatal: %s does not point to an active task", activePath)
	}
	taskID := strings.TrimSuffix(name, ".md")
	if !taskIDPattern.MatchString(taskID) {
		return "", fmt.Errorf("Fatal: invalid active task ID %s", taskID)
	}
	return taskID, nil
}

// readActiveTaskFile reads the task file currently selected by active.md.
func readActiveTaskFile(activePath string) ([]byte, error) {
	taskPath, err := resolveActiveTaskPath(activePath)
	if err != nil {
		return nil, err
	}
	contents, err := readTaskFile(taskPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("Fatal: active task symlink is broken")
		}
		return nil, err
	}
	return contents, nil
}

// appendActiveTaskFile appends an entry to the task file currently selected by
// active.md.
func appendActiveTaskFile(activePath string, entry string) (err error) {
	taskPath, err := resolveActiveTaskPath(activePath)
	if err != nil {
		return err
	}
	contents, err := readTaskFile(taskPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("Fatal: active task symlink is broken")
		}
		return fmt.Errorf("read %s: %w", taskPath, err)
	}
	if len(contents)+len(entry) > maxTaskFileSize {
		return fmt.Errorf("Fatal: note would make %s exceed maximum size of %d bytes", taskPath, maxTaskFileSize)
	}
	file, err := os.OpenFile(taskPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", taskPath, err)
	}
	defer closeOnReturn(file, &err, taskPath)
	if _, err = file.WriteString(entry); err != nil {
		return fmt.Errorf("append note to %s: %w", taskPath, err)
	}
	return nil
}
