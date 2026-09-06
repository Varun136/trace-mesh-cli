package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var tmBinary string

func TestMain(m *testing.M) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(thisFile))
	binary, err := os.CreateTemp("", "tracemesh-test-*")
	if err != nil {
		panic(err)
	}
	binary.Close()
	os.Remove(binary.Name())
	cmd := exec.Command("go", "build", "-o", binary.Name(), "./cmd/tm")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("build tm: %v\n%s", err, output))
	}
	tmBinary = binary.Name()
	code := m.Run()
	os.Remove(tmBinary)
	os.Exit(code)
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runIn(t, dir, "git", "init", "-q")
	runIn(t, dir, "git", "config", "user.email", "tests@example.com")
	runIn(t, dir, "git", "config", "user.name", "Tracemesh Tests")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test repository\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runIn(t, dir, "git", "add", "README.md")
	runIn(t, dir, "git", "commit", "-qm", "initial commit")
	return dir
}

func runIn(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %s %s\nerror: %v\nstdout:\n%s\nstderr:\n%s", name, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String() + stderr.String()
}

func tm(t *testing.T, dir, input string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(tmBinary, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatalf("run tm: %v", err)
		}
	}
	return stdout.String() + stderr.String(), code
}

func tmStreams(t *testing.T, dir, input string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(tmBinary, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatalf("run tm: %v", err)
		}
	}
	return stdout.String(), stderr.String(), code
}

func mustTM(t *testing.T, dir, input string, args ...string) string {
	t.Helper()
	output, code := tm(t, dir, input, args...)
	if code != 0 {
		t.Fatalf("tm %s failed with exit code %d:\n%s", strings.Join(args, " "), code, output)
	}
	return output
}

func assertContains(t *testing.T, text string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(text, value) {
			t.Fatalf("expected output to contain %q\nactual:\n%s", value, text)
		}
	}
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(contents)
}

func TestInitCreatesStateAndCheckoutHook(t *testing.T) {
	dir := newRepo(t)
	output := mustTM(t, dir, "", "init")
	assertContains(t, output, "Tracemesh initialized successfully.")
	for _, name := range []string{".tracemesh/config.json", ".tracemesh/tasks", ".tracemesh/archive", ".git/hooks/post-checkout"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
	hook := readFile(t, dir, ".git/hooks/post-checkout")
	assertContains(t, hook, "sync", "tracemesh post-checkout hook")
	info, _ := os.Stat(filepath.Join(dir, ".git/hooks/post-checkout"))
	if info.Mode()&0111 == 0 {
		t.Fatal("post-checkout hook is not executable")
	}
}

func TestPromptWritesProtocolToStdout(t *testing.T) {
	dir := newRepo(t)
	stdout, stderr, code := tmStreams(t, dir, "", "tm-prompt")
	if code != 0 {
		t.Fatalf("tm-prompt failed with exit code %d: %s", code, stderr)
	}
	assertContains(t, stdout, "TRACEMESH CONTEXT PROTOCOL")
	if stderr != "" {
		t.Fatalf("tm-prompt wrote unexpected stderr: %q", stderr)
	}
}

func TestInitRequiresGitRepository(t *testing.T) {
	dir := t.TempDir()
	output, code := tm(t, dir, "", "init")
	if code == 0 {
		t.Fatal("init unexpectedly succeeded outside a Git repository")
	}
	assertContains(t, output, "Git repository not found", "requires a Git repository")
}

func TestAddDetectsAndAddsAgentRules(t *testing.T) {
	dir := newRepo(t)
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project rules\n"), 0644)
	mustTM(t, dir, "", "init")
	assertContains(t, readFile(t, dir, "CLAUDE.md"), "TRACEMESH CONTEXT PROTOCOL")
	output := mustTM(t, dir, "", "add", "cursor")
	assertContains(t, output, "Added Tracemesh instructions for cursor", ".cursorrules")
	if strings.Count(readFile(t, dir, "CLAUDE.md"), "TRACEMESH CONTEXT PROTOCOL") != 1 {
		t.Fatal("init duplicated an existing Tracemesh protocol")
	}
	mustTM(t, dir, "", "add", "cursor")
	if strings.Count(readFile(t, dir, ".cursorrules"), "TRACEMESH CONTEXT PROTOCOL") != 1 {
		t.Fatal("add duplicated the Tracemesh protocol")
	}
}

func TestInitSupportsGitWorktrees(t *testing.T) {
	dir := newRepo(t)
	worktree := filepath.Join(t.TempDir(), "worktree")
	runIn(t, dir, "git", "worktree", "add", "-q", "-b", "worktree-branch", worktree)
	output := mustTM(t, worktree, "", "init")
	assertContains(t, output, "Tracemesh initialized successfully.")
}

func TestAddRepairsDeletedConfiguredAgentFile(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	mustTM(t, dir, "", "add", "cursor")
	if err := os.Remove(filepath.Join(dir, ".cursorrules")); err != nil {
		t.Fatal(err)
	}
	mustTM(t, dir, "", "add", "cursor")
	assertContains(t, readFile(t, dir, ".cursorrules"), "TRACEMESH CONTEXT PROTOCOL")
}

func TestAddFallbackAndRequiresInit(t *testing.T) {
	dir := newRepo(t)
	output, code := tm(t, dir, "", "add", "unknown-agent")
	if code == 0 {
		t.Fatal("add unexpectedly succeeded before init")
	}
	assertContains(t, output, "config.json not found", "tm init")
	mustTM(t, dir, "", "init")
	output = mustTM(t, dir, "", "add", "unknown-agent")
	assertContains(t, output, "AGENTS.md")
	assertContains(t, readFile(t, dir, "AGENTS.md"), "TRACEMESH CONTEXT PROTOCOL")
}

func TestInvalidInputsAndDetachedHeadAreRejected(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	if output, code := tm(t, dir, "", "start", "   "); code == 0 {
		t.Fatalf("blank title unexpectedly succeeded: %s", output)
	}
	mustTM(t, dir, "", "start", "Task")
	if output, code := tm(t, dir, "", "note", "   "); code == 0 {
		t.Fatalf("blank note unexpectedly succeeded: %s", output)
	}
	runIn(t, dir, "git", "checkout", "-q", "--detach", "HEAD")
	output, code := tm(t, dir, "", "status")
	if code == 0 {
		t.Fatalf("status unexpectedly succeeded in detached HEAD: %s", output)
	}
	assertContains(t, output, "detached HEAD is not supported")
}

func TestMalformedConfigurationIsRejected(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	if err := os.WriteFile(filepath.Join(dir, ".tracemesh/config.json"), []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	output, code := tm(t, dir, "", "list")
	if code == 0 {
		t.Fatal("list unexpectedly accepted malformed configuration")
	}
	assertContains(t, output, "parse .tracemesh/config.json")
}

func TestSwitchReportsArchivedTasksClearly(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	mustTM(t, dir, "", "start", "Archive me")
	mustTM(t, dir, "", "finish")
	output, code := tm(t, dir, "", "switch", "TM-001")
	if code == 0 {
		t.Fatal("switch unexpectedly activated an archived task")
	}
	assertContains(t, output, "is archived and cannot be activated")
}

func TestStartCreatesDescriptionAndActiveTask(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	output := mustTM(t, dir, "Detailed implementation plan\n", "start", "Implement feature")
	assertContains(t, output, "Started TM-001", "on branch")
	task := readFile(t, dir, ".tracemesh/tasks/TM-001.md")
	assertContains(t, task, "# Implement feature", "**ID:** TM-001", "**Status:** In Progress", "Detailed implementation plan", "## Implementation Log")
	active, err := os.Readlink(filepath.Join(dir, ".tracemesh/active.md"))
	if err != nil || filepath.ToSlash(active) != "tasks/TM-001.md" {
		t.Fatalf("unexpected active task link %q: %v", active, err)
	}
}

func TestStartUsesTitleWhenDescriptionIsBlankAndPreventsSecondActiveTask(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	mustTM(t, dir, "\n", "start", "First task")
	output, code := tm(t, dir, "", "start", "Second task")
	if code == 0 {
		t.Fatal("second task unexpectedly started")
	}
	assertContains(t, output, "active task already exists")
	assertContains(t, readFile(t, dir, ".tracemesh/tasks/TM-001.md"), "## Description\nFirst task")
}

func TestStatusShowAndNote(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	mustTM(t, dir, "", "start", "Track work")
	status := mustTM(t, dir, "", "status")
	assertContains(t, status, "Branch: master", "Active task: TM-001 - Track work")
	show := mustTM(t, dir, "", "show")
	assertContains(t, show, "# Track work", "## Implementation Log")
	mustTM(t, dir, "", "note", "Chose the safer implementation")
	assertContains(t, readFile(t, dir, ".tracemesh/tasks/TM-001.md"), "Chose the safer implementation")
}

func TestListAndHistoryShowActiveAndArchivedTasks(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	mustTM(t, dir, "", "start", "First")
	mustTM(t, dir, "", "finish")
	mustTM(t, dir, "", "start", "Second")
	list := mustTM(t, dir, "", "list")
	assertContains(t, list, "Active tasks:", "TM-002: Second", "Archived tasks:", "TM-001: First")
	history := mustTM(t, dir, "", "history")
	assertContains(t, history, "History for branch master", "TM-002: Second")
	if strings.Contains(history, "TM-001: First") {
		t.Fatal("finished task should be removed from the branch's active history")
	}
}

func TestSwitchChangesActiveTaskAndWarnsOnOtherBranch(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	mustTM(t, dir, "", "start", "Main task")
	runIn(t, dir, "git", "checkout", "-q", "-b", "feature")
	output := mustTM(t, dir, "", "switch", "TM-001")
	assertContains(t, output, "Warning: Task TM-001 is associated with branch master", "Switched active task to TM-001")
	active, _ := os.Readlink(filepath.Join(dir, ".tracemesh/active.md"))
	if filepath.ToSlash(active) != "tasks/TM-001.md" {
		t.Fatalf("switch selected %q", active)
	}
}

func TestSyncSelectsLatestBranchTaskAndClearsWhenNone(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	mustTM(t, dir, "", "start", "Main task")
	runIn(t, dir, "git", "checkout", "-q", "-b", "feature")
	mustTM(t, dir, "", "sync")
	if _, err := os.Lstat(filepath.Join(dir, ".tracemesh/active.md")); !os.IsNotExist(err) {
		t.Fatal("sync should clear active task on an unconfigured branch")
	}
	runIn(t, dir, "git", "checkout", "-q", "master")
	mustTM(t, dir, "", "sync")
	active, err := os.Readlink(filepath.Join(dir, ".tracemesh/active.md"))
	if err != nil || filepath.ToSlash(active) != "tasks/TM-001.md" {
		t.Fatalf("sync selected %q: %v", active, err)
	}
}

func TestFinishArchivesAndClearsActiveTask(t *testing.T) {
	dir := newRepo(t)
	mustTM(t, dir, "", "init")
	mustTM(t, dir, "", "start", "Complete me")
	output := mustTM(t, dir, "", "finish")
	assertContains(t, output, "Finished TM-001 on branch")
	if _, err := os.Stat(filepath.Join(dir, ".tracemesh/archive/TM-001.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".tracemesh/active.md")); !os.IsNotExist(err) {
		t.Fatal("finish did not clear active task")
	}
}

func TestPromptPrintsAgentProtocol(t *testing.T) {
	dir := newRepo(t)
	output := mustTM(t, dir, "", "tm-prompt")
	assertContains(t, output, "TRACEMESH CONTEXT PROTOCOL", ".tracemesh/active.md", "Implementation Log")
}

func TestCommandsRequireValidContext(t *testing.T) {
	dir := newRepo(t)
	for _, args := range [][]string{{"list"}, {"history"}, {"show"}, {"finish"}, {"note", "x"}} {
		if output, code := tm(t, dir, "", args...); code == 0 {
			t.Fatalf("%s unexpectedly succeeded before init: %s", args[0], output)
		}
	}
	mustTM(t, dir, "", "init")
	for _, args := range [][]string{{"show"}, {"finish"}, {"note", "x"}} {
		if output, code := tm(t, dir, "", args...); code == 0 {
			t.Fatalf("%s unexpectedly succeeded without active task: %s", args[0], output)
		}
	}
}
