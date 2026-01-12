package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iast-auto-inject/internal/core/detector"
	"iast-auto-inject/internal/core/injector"
	"iast-auto-inject/internal/core/process"
	"iast-auto-inject/internal/pkg/logger"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	daemonInterval   time.Duration
	daemonOnce       bool
	daemonNoDaemon   bool
	daemonPidFile    string
	daemonSecPoint   string
)

// daemonCmd daemon 命令
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "启动守护进程模式",
	Long:  `启动守护进程模式，定期扫描并自动注入 SecPoint 到 Java 进程`,
	RunE:  runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)

	daemonCmd.Flags().DurationVarP(&daemonInterval, "interval", "i", 0, "扫描间隔（默认使用配置文件中的值）")
	daemonCmd.Flags().BoolVar(&daemonOnce, "once", false, "只执行一次然后退出")
	daemonCmd.Flags().BoolVar(&daemonNoDaemon, "no-daemon", false, "前台运行（不后台化）")
	daemonCmd.Flags().StringVar(&daemonPidFile, "pid-file", "", "PID 文件路径")
	daemonCmd.Flags().StringVarP(&daemonSecPoint, "secpoint", "s", "", "SecPoint.jar 路径（必需）")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// 检查 SecPoint 路径
	if daemonSecPoint == "" {
		return fmt.Errorf("请指定 SecPoint.jar 路径（使用 --secpoint 或 -s）")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 获取扫描间隔
	interval := daemonInterval
	if interval == 0 {
		interval = GetConfig().Daemon.Interval
	}
	if interval == 0 {
		interval = 60 * time.Second
	}

	// 创建组件
	det := detector.NewDetector(GetConfig())
	procMgr := process.NewManager(
		GetConfig().Restart.GracePeriod,
		GetConfig().Restart.KillTimeout,
		GetConfig().Restart.VerifyWait,
		GetConfig().Restart.MaxRetries,
	)
	inj := injector.NewStaticInjector(GetConfig(), det, procMgr)

	color.Green("Starting daemon mode")
	logger.Info("Daemon started",
		zap.Duration("interval", interval),
		zap.Bool("once", daemonOnce),
		zap.String("secpoint", daemonSecPoint))

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 扫描循环
	scanCount := 0
	injectCount := 0

	for {
		scanCount++

		logger.Info("Scanning for Java processes", zap.Int("scan", scanCount))
		fmt.Printf("\n[%s] Scan #%d\n", time.Now().Format("2006-01-02 15:04:05"), scanCount)

		// 发现进程
		procs, err := det.DiscoverJavaProcesses(ctx, nil)
		if err != nil {
			logger.Error("Failed to discover processes", zap.Error(err))
		} else {
			logger.Debug("Found processes", zap.Int("count", len(procs)))

			// 找出需要注入的进程（未包含 SecPoint 的）
			var targets []*detector.JavaProcess
			for _, proc := range procs {
				if inj.NeedsInject(proc) {
					targets = append(targets, proc)
				}
			}

			if len(targets) == 0 {
				fmt.Println("No processes need injection")
			} else {
				color.Cyan("Found %d process(es) needing injection", len(targets))

				// 执行注入
				results := inj.BatchInject(ctx, targets, daemonSecPoint)

				// 统计成功数量
				for _, result := range results {
					if result.Success {
						injectCount++
						logger.Info("Injected SecPoint agent",
							zap.Int("pid", result.PID),
							zap.Int("new_pid", result.NewPID))
					} else {
						logger.Error("Failed to inject",
							zap.Int("pid", result.PID),
							zap.Error(result.Error))
					}
				}

				fmt.Printf("Injected: %d/%d\n", injectCount, len(results))
			}
		}

		// 单次执行模式
		if daemonOnce {
			color.Green("\nSingle execution completed")
			logger.Info("Single execution completed",
				zap.Int("scans", scanCount),
				zap.Int("injections", injectCount))
			break
		}

		// 等待下次扫描或信号
		select {
		case <-sigChan:
			color.Yellow("\nReceived signal, shutting down...")
			logger.Info("Received shutdown signal")
			return nil
		case <-time.After(interval):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
