package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const tracemeshAgentPrompt = "# [TRACEMESH CONTEXT PROTOCOL]\n" +
	"You must maintain situational awareness by reading local context.\n" +
	"1. ALWAYS read `.tracemesh/active.md` before generating your first response. This file contains the true state of the user's task.\n" +
	"2. Do NOT run terminal commands to log decisions. \n" +
	"3. BEFORE you finish your work or hand back control to the user, you MUST open `.tracemesh/active.md` natively in your editor and append a 1-sentence summary of your major code changes under the `## Decision Log` header.\n"

const tracemeshHookBlock = `
# >>> tracemesh post-checkout hook >>>
tm sync > /dev/null 2>&1 &
# <<< tracemesh post-checkout hook <<<
`

type config struct {
	Branches        map[string][]string `json:"branches"`
	Agents          []string            `json:"agents"`
	SupportedAgents map[string]string   `json:"supported_agents"`
}

func (cfg *config) ensureDefaults() {
	if cfg.Branches == nil {
		cfg.Branches = map[string][]string{}
	}
	if cfg.Agents == nil {
		cfg.Agents = []string{}
	}
	if cfg.SupportedAgents == nil {
		cfg.SupportedAgents = defaultSupportedAgents()
	}
}

func (cfg *config) addAgent(agent string) {
	for _, existing := range cfg.Agents {
		if existing == agent {
			return
		}
	}
	cfg.Agents = append(cfg.Agents, agent)
	sort.Strings(cfg.Agents)
}

func defaultSupportedAgents() map[string]string {
	agents := make(map[string]string, len(supportedAgentTargets)+1)
	for name, path := range supportedAgentTargets {
		agents[name] = path
	}
	agents["agents"] = "AGENTS.md"
	return agents
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tracemesh in the current Git repository",
	Long:  "Initialize Tracemesh by creating local state, agent rules, and a Git post-checkout hook.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	if err := ensureGitRepository(); err != nil {
		return err
	}

	if err := scaffoldTracemesh(); err != nil {
		return err
	}

	if err := injectAgentRules(); err != nil {
		return err
	}

	if err := installPostCheckoutHook(); err != nil {
		return err
	}

	fmt.Println("Tracemesh initialized successfully.")
	return nil
}

func ensureGitRepository() error {
	info, err := os.Stat(".git")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("Fatal: .git directory not found. Tracemesh requires a Git repository to function.")
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("Fatal: .git directory not found. Tracemesh requires a Git repository to function.")
	}
	return nil
}

func scaffoldTracemesh() error {
	for _, dir := range []string{".tracemesh/tasks", ".tracemesh/archive"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	configPath := filepath.Join(".tracemesh", "config.json")
	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	defaultConfig := config{Branches: map[string][]string{}}
	defaultConfig.ensureDefaults()
	contents, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}
	contents = append(contents, '\n')

	if err := os.WriteFile(configPath, contents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}

	return nil
}

func injectAgentRules() error {
	agentFiles := []string{
		".cursorrules",
		"CLAUDE.md",
		".windsurfrules",
		filepath.Join(".github", "copilot-instructions.md"),
		".clinerules",
		"AGENTS.md",
		filepath.Join(".agents", "rules"),
	}

	targets := existingFiles(agentFiles)
	if len(targets) == 0 {
		targets = []string{".cursorrules"}
	}

	for _, target := range targets {
		if err := appendAgentPromptIfMissing(target); err != nil {
			return err
		}
	}

	return nil
}

func existingFiles(paths []string) []string {
	var existing []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			existing = append(existing, path)
		}
	}
	return existing
}

func appendAgentPromptIfMissing(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read %s: %w", path, err)
		}
	} else if strings.Contains(string(contents), "[TRACEMESH CONTEXT PROTOCOL]") {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return fmt.Errorf("create parent for %s: %w", path, err)
	}

	prefix := ""
	if len(contents) > 0 && !strings.HasSuffix(string(contents), "\n") {
		prefix = "\n"
	}
	if len(contents) > 0 {
		prefix += "\n"
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	if _, err := file.WriteString(prefix + tracemeshAgentPrompt); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func installPostCheckoutHook() error {
	hookPath := filepath.Join(".git", "hooks", "post-checkout")

	contents, err := os.ReadFile(hookPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", hookPath, err)
	}

	if strings.Contains(string(contents), "tracemesh post-checkout hook") {
		return chmodExecutable(hookPath)
	}

	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return fmt.Errorf("create hook directory: %w", err)
	}

	prefix := ""
	if errors.Is(err, os.ErrNotExist) {
		prefix = "#!/usr/bin/env bash\n"
	} else if len(contents) > 0 && !strings.HasSuffix(string(contents), "\n") {
		prefix = "\n"
	}

	file, err := os.OpenFile(hookPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o755)
	if err != nil {
		return fmt.Errorf("open %s: %w", hookPath, err)
	}
	defer file.Close()

	if _, err := file.WriteString(prefix + tracemeshHookBlock); err != nil {
		return fmt.Errorf("write %s: %w", hookPath, err)
	}

	return chmodExecutable(hookPath)
}

func chmodExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	mode := info.Mode() | 0o111
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}
