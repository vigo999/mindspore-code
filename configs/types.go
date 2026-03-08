// Package configs provides configuration management for ms-cli.
package configs

import (
	"fmt"
)

// Config holds the complete application configuration.
type Config struct {
	Model       ModelConfig       `yaml:"model"`
	Budget      BudgetConfig      `yaml:"budget"`
	UI          UIConfig          `yaml:"ui"`
	Permissions PermissionsConfig `yaml:"permissions"`
	Context     ContextConfig     `yaml:"context"`
	Memory      MemoryConfig      `yaml:"memory"`
	Skills      SkillsConfig      `yaml:"skills"`
	Execution   ExecutionConfig   `yaml:"execution"`
	Training    TrainingConfig    `yaml:"training"`
}

// ModelConfig holds the LLM model configuration.
type ModelConfig struct {
	URL         string            `yaml:"url,omitempty"`
	Key         string            `yaml:"key,omitempty"`
	Model       string            `yaml:"model"`
	Temperature float64           `yaml:"temperature"`
	MaxTokens   int               `yaml:"max_tokens"`
	TimeoutSec  int               `yaml:"timeout_sec"`
	Headers     map[string]string `yaml:"headers,omitempty"`
}

// BudgetConfig holds the budget control configuration.
type BudgetConfig struct {
	MaxTokens  int     `yaml:"max_tokens"`
	MaxCostUSD float64 `yaml:"max_cost_usd"`
	DailyLimit int     `yaml:"daily_limit,omitempty"`
}

// UIConfig holds the UI configuration.
type UIConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Theme        string `yaml:"theme,omitempty"`
	ShowTokenBar bool   `yaml:"show_token_bar"`
	Animation    bool   `yaml:"animation"`
}

// PermissionsConfig holds the permission control configuration.
type PermissionsConfig struct {
	SkipRequests bool              `yaml:"skip_requests"`
	DefaultLevel string            `yaml:"default_level"`
	ToolPolicies map[string]string `yaml:"tool_policies,omitempty"`
	AllowedTools []string          `yaml:"allowed_tools"`
	BlockedTools []string          `yaml:"blocked_tools,omitempty"`
}

// ContextConfig holds the context management configuration.
type ContextConfig struct {
	MaxTokens           int     `yaml:"max_tokens"`
	ReserveTokens       int     `yaml:"reserve_tokens"`
	CompactionThreshold float64 `yaml:"compaction_threshold"`
	MaxHistoryRounds    int     `yaml:"max_history_rounds"`
}

// MemoryConfig holds the memory system configuration.
type MemoryConfig struct {
	Enabled   bool   `yaml:"enabled"`
	StorePath string `yaml:"store_path,omitempty"`
	MaxItems  int    `yaml:"max_items"`
	MaxBytes  int64  `yaml:"max_bytes"`
	TTLHours  int    `yaml:"ttl_hours"`
}

// SkillsConfig holds the skills system configuration.
type SkillsConfig struct {
	Repo      string   `yaml:"repo"`
	Revision  string   `yaml:"revision"`
	CacheDir  string   `yaml:"cache_dir"`
	Workflows []string `yaml:"workflows"`
}

// ExecutionConfig holds the execution configuration.
type ExecutionConfig struct {
	Mode           string       `yaml:"mode"`
	TimeoutSec     int          `yaml:"timeout_sec"`
	MaxConcurrency int          `yaml:"max_concurrency"`
	Docker         DockerConfig `yaml:"docker,omitempty"`
}

// DockerConfig holds the Docker execution configuration.
type DockerConfig struct {
	Image   string            `yaml:"image"`
	CPU     string            `yaml:"cpu"`
	Memory  string            `yaml:"memory"`
	Network string            `yaml:"network"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// TrainingConfig holds distributed training workflow configuration for /train.
type TrainingConfig struct {
	Enabled               bool                 `yaml:"enabled"`
	LocalPath             string               `yaml:"local_path"`
	TrainScript           string               `yaml:"train_script,omitempty"`
	StartupCommand        string               `yaml:"startup_command,omitempty"`
	RemoteCodePath        string               `yaml:"remote_code_path"`
	RunBaseDir            string               `yaml:"run_base_dir"`
	TrainCommand          string               `yaml:"train_command"`
	Exclude               []string             `yaml:"exclude"`
	SSHControlPersist     string               `yaml:"ssh_control_persist"`
	RsyncCompress         bool                 `yaml:"rsync_compress"`
	RsyncRespectGitIgnore bool                 `yaml:"rsync_respect_gitignore"`
	SyncParallelism       int                  `yaml:"sync_parallelism"`
	HostsFile             string               `yaml:"hosts_file,omitempty"`
	Hosts                 []TrainingHostConfig `yaml:"hosts"`
}

// TrainingHostConfig holds a single host config for /train workflow.
type TrainingHostConfig struct {
	Name           string `yaml:"name"`
	User           string `yaml:"user"`
	Address        string `yaml:"address"`
	LocalPath      string `yaml:"local_path,omitempty"`
	TrainScript    string `yaml:"train_script,omitempty"`
	StartupCommand string `yaml:"startup_command,omitempty"`
	RemoteCodePath string `yaml:"remote_code_path,omitempty"`
	RunBaseDir     string `yaml:"run_base_dir,omitempty"`
	TrainCommand   string `yaml:"train_command,omitempty"`
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig() *Config {
	return &Config{
		Model: ModelConfig{
			URL:         "https://api.openai.com/v1",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   4096,
			TimeoutSec:  180, // 3 minutes for longer conversations
			Headers:     make(map[string]string),
		},
		Budget: BudgetConfig{
			MaxTokens:  32768,
			MaxCostUSD: 10.0,
			DailyLimit: 0,
		},
		UI: UIConfig{
			Enabled:      true,
			Theme:        "default",
			ShowTokenBar: true,
			Animation:    true,
		},
		Permissions: PermissionsConfig{
			SkipRequests: false,
			DefaultLevel: "ask",
			ToolPolicies: make(map[string]string),
			AllowedTools: []string{},
			BlockedTools: []string{},
		},
		Context: ContextConfig{
			MaxTokens:           24000,
			ReserveTokens:       4000,
			CompactionThreshold: 0.85,
			MaxHistoryRounds:    10,
		},
		Memory: MemoryConfig{
			Enabled:   true,
			StorePath: "",
			MaxItems:  200,
			MaxBytes:  2 * 1024 * 1024, // 2MB
			TTLHours:  168,             // 7 days
		},
		Skills: SkillsConfig{
			Repo:      "https://github.com/vigo/mindspore-skills.git",
			Revision:  "main",
			CacheDir:  ".cache/skills",
			Workflows: []string{},
		},
		Execution: ExecutionConfig{
			Mode:           "local",
			TimeoutSec:     1800,
			MaxConcurrency: 2,
			Docker: DockerConfig{
				Image:   "ubuntu:22.04",
				CPU:     "2",
				Memory:  "4g",
				Network: "none",
				Env:     make(map[string]string),
			},
		},
		Training: TrainingConfig{
			Enabled:               false,
			LocalPath:             ".",
			TrainScript:           "",
			StartupCommand:        "",
			RemoteCodePath:        "~/workspace/project",
			RunBaseDir:            "~/workspace/runs",
			TrainCommand:          "python -u examples/fake_log_generator.py --run-id {{RUN_ID}} --host {{HOST_NAME}} --total-steps 120",
			Exclude:               DefaultTrainingExcludes(),
			SSHControlPersist:     "30m",
			RsyncCompress:         false,
			RsyncRespectGitIgnore: true,
			SyncParallelism:       0,
			HostsFile:             "configs/train_hosts.yaml",
			Hosts:                 []TrainingHostConfig{},
		},
	}
}

var defaultTrainingExcludePatterns = []string{
	".git",
	".cache",
	".mscli",
	"__pycache__",
	"*.pyc",
	"*.prof",
	"*.coverprofile",
	"*.test",
	"*.out",
	"ms-cli",
}

// DefaultTrainingExcludes returns the built-in ignore patterns for /train sync.
func DefaultTrainingExcludes() []string {
	return append([]string(nil), defaultTrainingExcludePatterns...)
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Model.URL == "" {
		return fmt.Errorf("model url is required")
	}

	if c.Model.Model == "" {
		return fmt.Errorf("model name is required")
	}

	if c.Model.Temperature < 0 || c.Model.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	if c.Budget.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be non-negative")
	}

	if c.Context.MaxTokens < c.Context.ReserveTokens {
		return fmt.Errorf("max_tokens must be greater than reserve_tokens")
	}

	if c.Training.Enabled {
		if len(c.Training.Hosts) == 0 {
			return fmt.Errorf("training.hosts or training.hosts_file is required when training.enabled=true")
		}
		if c.Training.SyncParallelism < 0 {
			return fmt.Errorf("training.sync_parallelism must be >= 0")
		}
		for i, host := range c.Training.Hosts {
			if host.Name == "" {
				return fmt.Errorf("training.hosts[%d].name is required", i)
			}
			if host.Address == "" {
				return fmt.Errorf("training.hosts[%d].address is required", i)
			}
			if host.User == "" {
				return fmt.Errorf("training.hosts[%d].user is required", i)
			}
		}
	}

	return nil
}

// Merge merges another config into this one (overwriting values).
func (c *Config) Merge(other *Config) {
	if other.Model.URL != "" {
		c.Model.URL = other.Model.URL
	}
	if other.Model.Key != "" {
		c.Model.Key = other.Model.Key
	}
	if other.Model.Model != "" {
		c.Model.Model = other.Model.Model
	}
	if other.Model.Temperature != 0 {
		c.Model.Temperature = other.Model.Temperature
	}
	if other.Model.MaxTokens != 0 {
		c.Model.MaxTokens = other.Model.MaxTokens
	}
	if other.Model.TimeoutSec != 0 {
		c.Model.TimeoutSec = other.Model.TimeoutSec
	}
	if len(other.Model.Headers) > 0 {
		c.Model.Headers = other.Model.Headers
	}

	if other.Budget.MaxTokens != 0 {
		c.Budget.MaxTokens = other.Budget.MaxTokens
	}
	if other.Budget.MaxCostUSD != 0 {
		c.Budget.MaxCostUSD = other.Budget.MaxCostUSD
	}

	if other.Context.MaxTokens != 0 {
		c.Context.MaxTokens = other.Context.MaxTokens
	}
	if other.Context.ReserveTokens != 0 {
		c.Context.ReserveTokens = other.Context.ReserveTokens
	}
	if other.Context.CompactionThreshold != 0 {
		c.Context.CompactionThreshold = other.Context.CompactionThreshold
	}
	if other.Context.MaxHistoryRounds != 0 {
		c.Context.MaxHistoryRounds = other.Context.MaxHistoryRounds
	}

	if other.Training.Enabled {
		c.Training.Enabled = true
	}
	if other.Training.LocalPath != "" {
		c.Training.LocalPath = other.Training.LocalPath
	}
	if other.Training.TrainScript != "" {
		c.Training.TrainScript = other.Training.TrainScript
	}
	if other.Training.StartupCommand != "" {
		c.Training.StartupCommand = other.Training.StartupCommand
	}
	if other.Training.RemoteCodePath != "" {
		c.Training.RemoteCodePath = other.Training.RemoteCodePath
	}
	if other.Training.RunBaseDir != "" {
		c.Training.RunBaseDir = other.Training.RunBaseDir
	}
	if other.Training.TrainCommand != "" {
		c.Training.TrainCommand = other.Training.TrainCommand
	}
	if len(other.Training.Exclude) > 0 {
		c.Training.Exclude = append([]string{}, other.Training.Exclude...)
	}
	if other.Training.SSHControlPersist != "" {
		c.Training.SSHControlPersist = other.Training.SSHControlPersist
	}
	if other.Training.HostsFile != "" {
		c.Training.HostsFile = other.Training.HostsFile
	}
	if len(other.Training.Hosts) > 0 {
		c.Training.Hosts = append([]TrainingHostConfig{}, other.Training.Hosts...)
	}
}
