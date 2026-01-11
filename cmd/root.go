package cmd

import (
	"fmt"
	"os"

	"iast-auto-inject/internal/core/config"
	"iast-auto-inject/internal/pkg/logger"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	cfgFile string
	debug   bool
	globalCfg *config.Config
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "iast-auto-inject",
	Short: "IAST Agent Auto Inject - Java Agent 自动注入工具",
	Long: `IAST Agent Auto Inject 是一个用于自动向 Java 进程注入 javaagent 的工具。

支持功能：
  - 自动发现系统中的 Java 进程
  - 静态注入（通过修改启动参数）
  - 单次执行和守护进程模式
  - 交互式菜单和命令行参数两种方式`,
	PersistentPreRunE: persistentPreRun,
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "配置文件路径")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "启用调试模式")
}

// persistentPreRun 持久化前置运行
func persistentPreRun(cmd *cobra.Command, args []string) error {
	// 加载配置
	var err error
	if cfgFile != "" {
		globalCfg, err = config.Load(cfgFile)
	} else {
		globalCfg, err = config.LoadFromDefaultPaths()
	}
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 调试模式覆盖
	if debug {
		globalCfg.Debug = true
	}

	// 初始化日志
	logLevel := globalCfg.Log.Level
	if globalCfg.Debug {
		logLevel = "debug"
	}

	if err := logger.Init(logLevel, globalCfg.Log.Format, globalCfg.Log.Output); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	logger.Debug("Configuration loaded",
		zap.String("config_file", cfgFile),
		zap.Bool("debug", globalCfg.Debug))

	return nil
}

// GetConfig 获取全局配置
func GetConfig() *config.Config {
	return globalCfg
}
