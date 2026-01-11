package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 顶层配置结构
type Config struct {
	Version string       `yaml:"version"`
	Debug   bool         `yaml:"debug"`
	Log     *LogConfig   `yaml:"log"`
	Agents  []AgentConfig `yaml:"agents"`
	Process *ProcessConfig `yaml:"process"`
	Daemon  *DaemonConfig  `yaml:"daemon"`
	Exclude []ExcludeRule  `yaml:"exclude"`
	Restart *RestartConfig `yaml:"restart"`
	Security *SecurityConfig `yaml:"security"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
}

// AgentConfig Agent 配置
type AgentConfig struct {
	Name     string `yaml:"name"`
	Path     string `yaml:"path"`
	Options  string `yaml:"options"`
	Enabled  bool   `yaml:"enabled"`
	Priority int    `yaml:"priority"`
}

// ProcessConfig 进程配置
type ProcessConfig struct {
	ScanInterval   time.Duration `yaml:"scan_interval"`
	IncludePattern []string      `yaml:"include_pattern"`
	UserFilter     []string      `yaml:"user_filter"`
	AutoRestart    bool          `yaml:"auto_restart"`
}

// DaemonConfig 守护进程配置
type DaemonConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
	LogLevel string        `yaml:"log_level"`
	PidFile  string        `yaml:"pid_file"`
}

// ExcludeRule 排除规则
type ExcludeRule struct {
	Name     string   `yaml:"name"`
	PIDs     []int    `yaml:"pids"`
	Patterns []string `yaml:"patterns"`
	Users    []string `yaml:"users"`
}

// RestartConfig 重启配置
type RestartConfig struct {
	GracePeriod time.Duration `yaml:"grace_period"`
	KillTimeout time.Duration `yaml:"kill_timeout"`
	MaxRetries  int           `yaml:"max_retries"`
	VerifyWait  time.Duration `yaml:"verify_wait"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	CheckPermissions     bool     `yaml:"check_permissions"`
	AllowedUsers         []string `yaml:"allowed_users"`
	AllowedGroups        []string `yaml:"allowed_groups"`
	RequireConfirmation  bool     `yaml:"require_confirmation"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Version: "1.0",
		Debug:   false,
		Log: &LogConfig{
			Level:      "info",
			Format:     "console",
			Output:     "/var/log/iast-auto-inject.log",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
		Agents: []AgentConfig{
			{
				Name:     "iast-agent",
				Path:     "/opt/iast/agent/iast-agent.jar",
				Options:  "",
				Enabled:  true,
				Priority: 100,
			},
		},
		Process: &ProcessConfig{
			ScanInterval:   30 * time.Second,
			IncludePattern: []string{".*"},
			UserFilter:     []string{},
			AutoRestart:    true,
		},
		Daemon: &DaemonConfig{
			Enabled:  false,
			Interval: 60 * time.Second,
			LogLevel: "info",
			PidFile:  "/var/run/iast-auto-inject.pid",
		},
		Exclude: []ExcludeRule{},
		Restart: &RestartConfig{
			GracePeriod: 10 * time.Second,
			KillTimeout: 30 * time.Second,
			MaxRetries:  3,
			VerifyWait:  5 * time.Second,
		},
		Security: &SecurityConfig{
			CheckPermissions:    true,
			AllowedUsers:        []string{},
			AllowedGroups:       []string{},
			RequireConfirmation: true,
		},
	}
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML
	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// LoadFromDefaultPaths 从默认路径加载配置
func LoadFromDefaultPaths() (*Config, error) {
	paths := []string{
		"config.yaml",
		"configs/config.yaml",
		filepath.Join(os.Getenv("HOME"), ".iast-inject", "config.yaml"),
		"/etc/iast-inject/config.yaml",
	}

	for _, path := range paths {
		config, err := Load(path)
		if err == nil {
			return config, nil
		}
	}

	// 返回默认配置
	return DefaultConfig(), nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证 Agent 配置
	for i, agent := range c.Agents {
		if agent.Name == "" {
			return fmt.Errorf("agent[%d]: name cannot be empty", i)
		}
		if agent.Path == "" {
			return fmt.Errorf("agent[%d]: path cannot be empty", i)
		}
		// 只在启用时检查 agent 文件是否存在
		if agent.Enabled {
			if _, err := os.Stat(agent.Path); os.IsNotExist(err) {
				return fmt.Errorf("agent[%d]: file not found: %s", i, agent.Path)
			}
		}
	}

	// 验证进程配置
	if c.Process != nil {
		if c.Process.ScanInterval <= 0 {
			return fmt.Errorf("process.scan_interval must be positive")
		}
	}

	// 验证守护进程配置
	if c.Daemon != nil {
		if c.Daemon.Enabled && c.Daemon.Interval <= 0 {
			return fmt.Errorf("daemon.interval must be positive when enabled")
		}
	}

	return nil
}

// GetEnabledAgents 获取启用的 Agent
func (c *Config) GetEnabledAgents() []AgentConfig {
	var agents []AgentConfig
	for _, agent := range c.Agents {
		if agent.Enabled {
			agents = append(agents, agent)
		}
	}
	return agents
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	// 创建目录
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 序列化为 YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
