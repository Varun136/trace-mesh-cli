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

type agentTarget struct {
	Name string
	Path string
}

var supportedAgentTargets = map[string]string{
	"cursor":      ".cursorrules",
	"claude":      "CLAUDE.md",
	"windsurf":    ".windsurfrules",
	"copilot":     filepath.Join(".github", "copilot-instructions.md"),
	"cline":       ".clinerules",
	"opencode":    "AGENTS.md",
	"antigravity": "GEMINI.md (or .agents/rule[s]/tracemesh.md)",
}

var addCmd = &cobra.Command{
	Use:   "add [agent_name]",
	Short: "Opt a coding agent into Tracemesh",
	Long:  "Opt a coding agent into Tracemesh by appending the context protocol to its rule file.\n\nSupported agents:\n" + supportedAgentsHelp(),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAdd(args[0])
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(agentName string) error {
	if err := ensureGitRepository(); err != nil {
		return err
	}

	cfg, err := readConfig()
	if err != nil {
		return err
	}

	target := resolveAgentTarget(agentName)
	for _, existing := range cfg.Agents {
		if existing == target.Name {
			fmt.Printf("Tracemesh instructions for %s are already added.\n", target.Name)
			return nil
		}
	}
	if err := appendAgentPromptIfMissing(target.Path); err != nil {
		return err
	}

	cfg.ensureDefaults()
	cfg.addAgent(target.Name)

	if err := writeConfig(cfg); err != nil {
		return err
	}

	fmt.Printf("Added Tracemesh instructions for %s in %s.\n", target.Name, target.Path)
	return nil
}

func resolveAgentTarget(agentName string) agentTarget {
	normalized := strings.ToLower(strings.TrimSpace(agentName))
	if normalized == "antigravity" {
		if info, err := os.Stat("GEMINI.md"); err == nil && !info.IsDir() {
			return agentTarget{Name: normalized, Path: "GEMINI.md"}
		}
		for _, directory := range []string{filepath.Join(".agents", "rule"), filepath.Join(".agents", "rules")} {
			if info, err := os.Stat(directory); err == nil && info.IsDir() {
				return agentTarget{Name: normalized, Path: filepath.Join(directory, "tracemesh.md")}
			}
		}
		return agentTarget{Name: normalized, Path: "GEMINI.md"}
	}
	if path, ok := supportedAgentTargets[normalized]; ok {
		return agentTarget{Name: normalized, Path: path}
	}
	return agentTarget{Name: "agents", Path: "AGENTS.md"}
}

func supportedAgentsHelp() string {
	names := make([]string, 0, len(supportedAgentTargets)+1)
	for name := range supportedAgentTargets {
		names = append(names, name)
	}
	sort.Strings(names)

	var builder strings.Builder
	for _, name := range names {
		builder.WriteString(fmt.Sprintf("  %-12s %s\n", name, supportedAgentTargets[name]))
	}
	builder.WriteString(fmt.Sprintf("  %-12s %s", "agents", "AGENTS.md (fallback for all other agents)"))
	return builder.String()
}

func readConfig() (*config, error) {
	configPath := filepath.Join(".tracemesh", "config.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Fatal: .tracemesh/config.json not found. Run `tm init` first")
		}
		return nil, fmt.Errorf("read %s: %w", configPath, err)
	}

	var cfg config
	if err := json.Unmarshal(contents, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}
	cfg.ensureDefaults()
	return &cfg, nil
}

func writeConfig(cfg *config) error {
	configPath := filepath.Join(".tracemesh", "config.json")
	contents, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(configPath, contents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}
