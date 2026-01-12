package menu

import (
	"context"
	"fmt"
	"strconv"
	"text/tabwriter"

	"iast-auto-inject/internal/core/detector"

	"github.com/fatih/color"
	"os"
)

// showInjectMenu 显示注入菜单
func (m *Menu) showInjectMenu() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                  SecPoint Agent 注入")
	fmt.Println()

	// 输入 SecPoint.jar 路径
	secPointPath := m.readInput("请输入 SecPoint.jar 路径: ")
	if secPointPath == "" {
		color.Red("路径不能为空")
		m.pause()
		return
	}

	fmt.Println()
	// 选择目标进程
	fmt.Println("选择目标进程:")
	fmt.Println("  1. 指定 PID")
	fmt.Println("  2. 所有未注入 SecPoint 的进程")

	choice := m.readInput("请选择 [1-2]: ")

	var targetProcs []*detector.JavaProcess
	ctx := context.Background()
	allProcs, _ := m.detector.DiscoverJavaProcesses(ctx, nil)

	switch choice {
	case "1":
		// 指定 PID
		pid, err := m.readIntInput("输入进程 PID: ")
		if err != nil {
			color.Red("无效的 PID")
			m.pause()
			return
		}

		for _, proc := range allProcs {
			if proc.PID == pid {
				targetProcs = append(targetProcs, proc)
				break
			}
		}

		if len(targetProcs) == 0 {
			color.Red("未找到 PID 为 %d 的进程", pid)
			m.pause()
			return
		}

	case "2":
		// 所有未注入 SecPoint 的进程
		for _, proc := range allProcs {
			if m.injector.NeedsInject(proc) {
				targetProcs = append(targetProcs, proc)
			}
		}

		if len(targetProcs) == 0 {
			color.Yellow("没有需要注入的进程")
			m.pause()
			return
		}

	default:
		color.Red("无效的选择")
		m.pause()
		return
	}

	// 显示目标进程
	fmt.Println()
	color.Cyan("目标进程:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PID\tUser\tMain Class/JAR\t\tSecPoint")

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	for _, proc := range targetProcs {
		main := proc.MainClass
		if proc.JarFile != "" {
			main = proc.JarFile
		}

		secPointStatus := red("✗")
		if len(proc.Agents) > 0 {
			secPointStatus = green("✓")
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t\t%s\n", proc.PID, proc.User, main, secPointStatus)
	}
	w.Flush()

	fmt.Printf("\nSecPoint Agent: %s\n", secPointPath)
	fmt.Println()
	confirm := m.readInput("确认注入? (y/N): ")
	if confirm != "y" && confirm != "Y" {
		color.Yellow("已取消")
		m.pause()
		return
	}

	// 执行注入
	fmt.Println()
	color.Cyan("开始注入...")

	results := m.injector.BatchInject(ctx, targetProcs, secPointPath)

	// 显示结果
	fmt.Println()
	color.Cyan("注入结果:")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PID\t状态\t新 PID\t消息")

	for _, result := range results {
		var status string
		if result.Success {
			status = "✓ 成功"
		} else {
			status = "✗ 失败"
		}

		newPID := "-"
		if result.NewPID > 0 {
			newPID = strconv.Itoa(result.NewPID)
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			result.PID, status, newPID, result.Message)
	}

	w.Flush()

	// 统计
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	fmt.Printf("\n成功: %d/%d\n", successCount, len(results))
	m.pause()
}
