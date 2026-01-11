package cmd

import (
	"fmt"

	"iast-auto-inject/internal/core/detector"
	"iast-auto-inject/internal/core/injector"
	"iast-auto-inject/internal/core/process"
	"iast-auto-inject/internal/ui/menu"

	"github.com/spf13/cobra"
)

// interactiveCmd 交互式菜单命令
var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "启动交互式菜单",
	Long:  `启动交互式菜单，通过菜单选择操作`,
	RunE:  runInteractive,
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

func runInteractive(cmd *cobra.Command, args []string) error {
	cfg := GetConfig()

	// 创建组件
	det := detector.NewDetector(cfg)
	procMgr := process.NewManager(
		cfg.Restart.GracePeriod,
		cfg.Restart.KillTimeout,
		cfg.Restart.VerifyWait,
		cfg.Restart.MaxRetries,
	)
	inj := injector.NewStaticInjector(cfg, det, procMgr)

	// 创建并显示菜单
	m := menu.NewMenu(cfg, det, inj)
	if err := m.Show(); err != nil {
		return fmt.Errorf("menu error: %w", err)
	}

	return nil
}
