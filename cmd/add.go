package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	maxConfigSize   = 1 << 20 // 1 MB
	maxTaskFileSize = 1 << 20 // 1 MB
	lockTimeout     = 5 * time.Second
	lockRetryDelay  = 100 * time.Millisecond
	lockRetries     = 50
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
	return withConfigLock(func() error {
		return runAddLocked(agentName)
	})
}

func runAddLocked(agentName string) error {
	if err := ensureGitRepository(); err != nil {
		return err
	}

	cfg, err := readConfigUnlocked()
	if err != nil {
		return err
	}

	target := resolveAgentTarget(agentName)
	alreadyAdded := false
	for _, existing := range cfg.Agents {
		if existing == target.Name {
			alreadyAdded = true
			break
		}
	}
	// Always inspect the file: configuration can outlive a deleted instruction file.
	if err := appendAgentPromptIfMissing(target.Path); err != nil {
		return err
	}
	if alreadyAdded {
		fmt.Printf("Tracemesh instructions for %s are already added in %s.\n", target.Name, target.Path)
		return nil
	}

	cfg.ensureDefaults()
	cfg.addAgent(target.Name)

	if err := writeConfigUnlocked(cfg); err != nil {
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
	lockPath := filepath.Join(".tracemesh", "config.json") + ".lock"
	if _, err := os.Stat(filepath.Dir(lockPath)); errors.Is(err, os.ErrNotExist) {
		return readConfigUnlocked()
	} else if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", filepath.Dir(lockPath), err)
	}
	if err := acquireLock(lockPath); err != nil {
		return nil, err
	}
	defer releaseLock(lockPath)
	return readConfigUnlocked()
}

func readConfigUnlocked() (*config, error) {
	configPath := filepath.Join(".tracemesh", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Fatal: .tracemesh/config.json not found. Run `tm init` first")
		}
		return nil, fmt.Errorf("inspect %s: %w", configPath, err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", configPath, err)
	}
	if len(contents) > maxConfigSize {
		return nil, fmt.Errorf("Fatal: %s exceeds maximum size of %d bytes", configPath, maxConfigSize)
	}

	var cfg config
	if err := json.Unmarshal(contents, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}
	cfg.ensureDefaults()
	return &cfg, nil
}

func writeConfig(cfg *config) error {
	lockPath := filepath.Join(".tracemesh", "config.json") + ".lock"
	if err := acquireLock(lockPath); err != nil {
		return err
	}
	defer releaseLock(lockPath)
	return writeConfigUnlocked(cfg)
}

func writeConfigUnlocked(cfg *config) error {
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

func withConfigLock(fn func() error) error {
	lockPath := filepath.Join(".tracemesh", "config.json") + ".lock"
	if _, err := os.Stat(filepath.Dir(lockPath)); errors.Is(err, os.ErrNotExist) {
		return fn()
	} else if err != nil {
		return fmt.Errorf("inspect %s: %w", filepath.Dir(lockPath), err)
	}
	if err := acquireLock(lockPath); err != nil {
		return err
	}
	defer releaseLock(lockPath)
	return fn()
}

func readTaskFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxTaskFileSize {
		return nil, fmt.Errorf("Fatal: %s exceeds maximum size of %d bytes", path, maxTaskFileSize)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func acquireLock(lockPath string) error {
	for i := 0; i < lockRetries; i++ {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			file.Close()
			return nil
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("acquire lock %s: %w", lockPath, err)
		}
		info, statErr := os.Stat(lockPath)
		if statErr == nil && time.Since(info.ModTime()) > lockTimeout {
			os.Remove(lockPath)
			continue
		}
		time.Sleep(lockRetryDelay)
	}
	return fmt.Errorf("Fatal: timeout acquiring lock on config after %v", lockTimeout)
}

func releaseLock(lockPath string) {
	os.Remove(lockPath)
}
