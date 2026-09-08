package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const tracemeshAgentPrompt = "# [TRACEMESH CONTEXT PROTOCOL]\n" +
	"You must use the Tracemesh task file as the source of truth for the current work.\n\n" +
	"1. BEFORE generating your first response, read `.tracemesh/active.md`.\n" +
	"2. Review the task title, ID, status, description, and implementation log before making changes.\n" +
	"3. Keep your work aligned with the active task description. Ask the user when requirements are unclear.\n" +
	"4. Record important implementation decisions, discoveries, and changes under the `## Implementation Log` header.\n" +
	"5. Add implementation-log entries as timestamped Markdown bullets using this format: `- YYYY-MM-DDTHH:MM:SS±HH:MM: Description`\n" +
	"6. Do NOT run terminal commands solely to record implementation details.\n" +
	"7. Do NOT overwrite, delete, or replace `.tracemesh/active.md`.\n" +
	"8. BEFORE finishing or handing control back to the user, read `.tracemesh/active.md` again and append one short sentence summarizing the major implementation changes under `## Implementation Log`.\n" +
	"9. Keep implementation-log entries concise and relevant. Do not duplicate the entire conversation or task file.\n"

const tracemeshHookBlock = `
# >>> tracemesh post-checkout hook >>>
if tm_bin=$(command -v tm); then
  "$tm_bin" sync || echo "Tracemesh warning: unable to synchronize task context" >&2
else
  echo "Tracemesh warning: tm is not on PATH; task context was not synchronized" >&2
fi
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

	if err := ensureTracemeshIgnored(); err != nil {
		return err
	}
	if err := scaffoldTracemesh(); err != nil {
		return err
	}

	detectedAgents, err := injectAgentRules()
	if err != nil {
		return err
	}
	if err := recordDetectedAgents(detectedAgents); err != nil {
		return err
	}

	if err := installPostCheckoutHook(); err != nil {
		return err
	}

	fmt.Println("Tracemesh initialized successfully.")
	return nil
}

func ensureGitRepository() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if output, err := cmd.CombinedOutput(); err != nil {
		if len(output) == 0 {
			return fmt.Errorf("Fatal: Git repository not found. Tracemesh requires a Git repository to function")
		}
		return fmt.Errorf("Fatal: Git repository not found. Tracemesh requires a Git repository to function: %s", strings.TrimSpace(string(output)))
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

func injectAgentRules() ([]string, error) {
	targets := detectedAgentTargets()
	if len(targets) == 0 {
		fmt.Println("No supported AI-agent instruction file was detected.")
		fmt.Println("Choose an option:")
		fmt.Println("  [A] Create AGENTS.md and add the Tracemesh instructions (default)")
		fmt.Println("  [B] Show the instructions so I can add them manually")
		fmt.Print("Choice [A/B]: ")

		choice, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.EqualFold(strings.TrimSpace(choice), "b") {
			fmt.Println("\nAdd the following instructions to your AI-agent instruction file:")
			printTracemeshPrompt()
			fmt.Println("Warning: AI-agent integration will not work until these instructions are added manually.")
			return nil, nil
		}

		if err := appendAgentPromptIfMissing("AGENTS.md"); err != nil {
			return nil, err
		}
		fmt.Println("Created AGENTS.md with the Tracemesh instructions.")
		return []string{"agents"}, nil
	}

	detected := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := appendAgentPromptIfMissing(target.Path); err != nil {
			return nil, err
		}
		detected = append(detected, target.Name)
		fmt.Printf("Auto-detected %s instructions in %s.\n", target.Name, target.Path)
	}

	return detected, nil
}

func detectedAgentTargets() []agentTarget {
	var targets []agentTarget
	for _, target := range []agentTarget{
		{Name: "cursor", Path: ".cursorrules"},
		{Name: "claude", Path: "CLAUDE.md"},
		{Name: "windsurf", Path: ".windsurfrules"},
		{Name: "copilot", Path: filepath.Join(".github", "copilot-instructions.md")},
		{Name: "cline", Path: ".clinerules"},
		{Name: "opencode", Path: "AGENTS.md"},
		{Name: "antigravity", Path: "GEMINI.md"},
	} {
		if info, err := os.Stat(target.Path); err == nil && !info.IsDir() {
			targets = append(targets, target)
		}
	}

	for _, directory := range []string{filepath.Join(".agents", "rule"), filepath.Join(".agents", "rules")} {
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			targets = append(targets, agentTarget{Name: "antigravity", Path: filepath.Join(directory, "tracemesh.md")})
			break
		}
	}
	return targets
}

func ensureTracemeshIgnored() error {
	const entry = ".tracemesh/"
	path := ".gitignore"
	contents, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	updated := string(contents)
	if len(updated) > 0 && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	updated += entry + "\n"
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("Added %s to %s.\n", entry, path)
	return nil
}

func repairConfiguredAgentRules(cfg *config) error {
	for _, name := range cfg.Agents {
		target := resolveAgentTarget(name)
		contents, err := os.ReadFile(target.Path)
		if err == nil && strings.Contains(string(contents), "[TRACEMESH CONTEXT PROTOCOL]") {
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read configured %s instructions %s: %w", name, target.Path, err)
		}
		if err := appendAgentPromptIfMissing(target.Path); err != nil {
			return err
		}
		fmt.Printf("Restored Tracemesh instructions for %s in %s.\n", name, target.Path)
	}
	return nil
}

func recordDetectedAgents(names []string) error {
	if len(names) == 0 {
		return nil
	}
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	for _, name := range names {
		cfg.addAgent(name)
	}
	return writeConfig(cfg)
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
	hookDir, err := gitPath("hooks")
	if err != nil {
		return err
	}
	hookPath := filepath.Join(hookDir, "post-checkout")

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

func gitPath(name string) (string, error) {
	output, err := exec.Command("git", "rev-parse", "--git-path", name).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve Git %s path: %w", name, err)
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("resolve Git %s path: empty path", name)
	}
	return filepath.FromSlash(path), nil
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
