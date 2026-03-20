// Package config manages devctx configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the devctx configuration.
type Config struct {
	VCS    VCSConfig    `toml:"vcs"`
	Docker DockerConfig `toml:"docker"`
	AI     AIConfig     `toml:"ai"`
	Hooks  HooksConfig  `toml:"hooks"`
}

// VCSConfig holds version control settings.
type VCSConfig struct {
	Prefer string `toml:"prefer"` // "jj" or "git"
}

// DockerConfig holds Docker settings.
type DockerConfig struct {
	PortBase int      `toml:"port_base"`
	PortStep int      `toml:"port_step"`
	Services []string `toml:"services"`
}

// AIConfig holds AI assistant settings.
type AIConfig struct {
	Default    string `toml:"default"` // "claude" or "kimi"
	SessionDir string `toml:"session_dir"`
}

// HooksConfig holds lifecycle hooks.
type HooksConfig struct {
	PostCreate []string `toml:"post-create"`
	PreSwitch  []string `toml:"pre-switch"`
	PostSwitch []string `toml:"post-switch"`
	PreRemove  []string `toml:"pre-remove"`
}

// Default returns the default configuration.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		VCS: VCSConfig{
			Prefer: "jj",
		},
		Docker: DockerConfig{
			PortBase: 3000,
			PortStep: 10,
			Services: []string{"app", "db", "redis"},
		},
		AI: AIConfig{
			Default:    "claude",
			SessionDir: filepath.Join(home, ".config", "devctx", "sessions"),
		},
		Hooks: HooksConfig{},
	}
}

// Load loads configuration from file.
func Load(path string) (*Config, error) {
	cfg := Default()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}

// LoadProject loads project-specific configuration.
func LoadProject(projectPath string) (*Config, error) {
	configPath := filepath.Join(projectPath, ".devctx", "config.toml")
	return Load(configPath)
}

// Save saves configuration to file.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// SaveProject saves project-specific configuration.
func (c *Config) SaveProject(projectPath string) error {
	configPath := filepath.Join(projectPath, ".devctx", "config.toml")
	return c.Save(configPath)
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devctx", "config.toml")
}

// LoadGlobal loads the global configuration.
func LoadGlobal() (*Config, error) {
	return Load(GlobalConfigPath())
}

// SaveGlobal saves the global configuration.
func (c *Config) SaveGlobal() error {
	return c.Save(GlobalConfigPath())
}

// Merge merges another config into this one (override with non-zero values).
func (c *Config) Merge(other *Config) {
	if other.VCS.Prefer != "" {
		c.VCS.Prefer = other.VCS.Prefer
	}
	if other.Docker.PortBase != 0 {
		c.Docker.PortBase = other.Docker.PortBase
	}
	if other.Docker.PortStep != 0 {
		c.Docker.PortStep = other.Docker.PortStep
	}
	if len(other.Docker.Services) > 0 {
		c.Docker.Services = other.Docker.Services
	}
	if other.AI.Default != "" {
		c.AI.Default = other.AI.Default
	}
	if other.AI.SessionDir != "" {
		c.AI.SessionDir = other.AI.SessionDir
	}
	if len(other.Hooks.PostCreate) > 0 {
		c.Hooks.PostCreate = other.Hooks.PostCreate
	}
	if len(other.Hooks.PreSwitch) > 0 {
		c.Hooks.PreSwitch = other.Hooks.PreSwitch
	}
	if len(other.Hooks.PostSwitch) > 0 {
		c.Hooks.PostSwitch = other.Hooks.PostSwitch
	}
	if len(other.Hooks.PreRemove) > 0 {
		c.Hooks.PreRemove = other.Hooks.PreRemove
	}
}
