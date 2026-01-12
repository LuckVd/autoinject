package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"iast-auto-inject/internal/core/detector"
	"iast-auto-inject/internal/core/injector"
	"iast-auto-inject/internal/core/process"
	"iast-auto-inject/internal/pkg/logger"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	injectPids     []int
	injectAll      bool
	injectSecPoint string
	injectDryRun   bool
	injectForce    bool
)

// injectCmd inject 命令
var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "注入 SecPoint agent 到 Java 进程",
	Long:  `向指定的 Java 进程注入 SecPoint.jar，需要重启进程`,
	RunE:  runInject,
}

func init() {
	rootCmd.AddCommand(injectCmd)

	injectCmd.Flags().IntSliceVarP(&injectPids, "pid", "p", []int{}, "目标进程 PID（可多次指定）")
	injectCmd.Flags().BoolVarP(&injectAll, "all", "a", false, "注入所有未包含 SecPoint 的进程")
	injectCmd.Flags().StringVarP(&injectSecPoint, "secpoint", "s", "", "SecPoint.jar 路径（必需）")
	injectCmd.Flags().BoolVarP(&injectDryRun, "dry-run", "n", false, "模拟运行（不实际注入）")
	injectCmd.Flags().BoolVarP(&injectForce, "force", "f", false, "强制注入（跳过确认）")
}

func runInject(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 检查参数
	if injectSecPoint == "" {
		return fmt.Errorf("请指定 SecPoint.jar 路径（使用 --secpoint 或 -s）")
	}

	if len(injectPids) == 0 && !injectAll {
		return fmt.Errorf("请指定目标进程（使用 --pid 或 --all）")
	}

	if len(injectPids) > 0 && injectAll {
		return fmt.Errorf("--pid 和 --all 不能同时使用")
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

	logger.Info("Injecting SecPoint agent",
		zap.String("agent_path", injectSecPoint),
		zap.Int("targets", len(injectPids)))

	// 获取目标进程
	var targetProcs []*detector.JavaProcess

	if injectAll {
		// 获取所有进程
		procs, err := det.DiscoverJavaProcesses(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to discover processes: %w", err)
		}

		// 过滤需要注入的进程（未包含 SecPoint 的）
		for _, proc := range procs {
			if inj.NeedsInject(proc) {
				targetProcs = append(targetProcs, proc)
			}
		}
	} else {
		// 获取指定 PID 的进程
		for _, pid := range injectPids {
			procs, err := det.DiscoverJavaProcesses(ctx, &detector.ProcessFilter{PIDs: []int{pid}})
			if err != nil {
				logger.Warn("Failed to get process info", zap.Int("pid", pid), zap.Error(err))
				continue
			}
			if len(procs) > 0 {
				targetProcs = append(targetProcs, procs[0])
			}
		}
	}

	if len(targetProcs) == 0 {
		color.Yellow("No target processes found")
		return nil
	}

	// 显示目标进程
	fmt.Println("\nTarget processes:")
	fmt.Printf("SecPoint Agent: %s\n\n", injectSecPoint)
	printInjectTargets(targetProcs)

	// 确认
	if !injectForce && !injectDryRun {
		fmt.Print("\nProceed with injection? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Injection cancelled")
			return nil
		}
	}

	// 模拟运行
	if injectDryRun {
		color.Yellow("\n[DRY RUN] Would inject SecPoint to:")
		for _, proc := range targetProcs {
			fmt.Printf("  PID %d: %s\n", proc.PID, proc.JarFile)
		}
		return nil
	}

	// 执行注入
	results := inj.BatchInject(ctx, targetProcs, injectSecPoint)

	// 显示结果
	printInjectResults(results)

	// 记录日志
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	logger.Info("Injection completed",
		zap.Int("total", len(results)),
		zap.Int("success", successCount),
		zap.Int("failed", len(results)-successCount))

	return nil
}

// printInjectTargets 打印注入目标
func printInjectTargets(procs []*detector.JavaProcess) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "PID\tUser\tMain Class/JAR\t\tSecPoint Status")

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	for _, proc := range procs {
		main := proc.MainClass
		if proc.JarFile != "" {
			main = proc.JarFile
		}
		if main == "" {
			main = "unknown"
		}

		secPointStatus := red("✗ Not attached")
		if len(proc.Agents) > 0 {
			secPointStatus = green("✓ Attached")
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t\t%s\n",
			proc.PID, proc.User, truncate(main, 25), secPointStatus)
	}

	w.Flush()
}

// printInjectResults 打印注入结果
func printInjectResults(results []*injector.InjectResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "\nResults:")
	fmt.Fprintln(w, "PID\tStatus\tNew PID\tMessage")

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	for _, result := range results {
		status := green("✓ Success")
		if !result.Success {
			status = red("✗ Failed")
		}

		newPid := "-"
		if result.NewPID > 0 {
			newPid = strconv.Itoa(result.NewPID)
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			result.PID, status, newPid, result.Message)
	}

	w.Flush()
}
