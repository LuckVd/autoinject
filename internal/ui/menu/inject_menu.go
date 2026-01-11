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
	color.Cyan("                     Agent 注入")
	fmt.Println()

	// 显示可用的 Agent
	fmt.Println("可用的 Agent:")
	configAgents := m.config.GetEnabledAgents()
	if len(configAgents) == 0 {
		color.Yellow("  没有启用的 Agent，请先在配置中启用")
		m.pause()
		return
	}

	// 转换为 detector.Agent
	var agents []detector.Agent
	for _, cfgAgent := range configAgents {
		agents = append(agents, detector.Agent{
			Path:    cfgAgent.Path,
			Options: cfgAgent.Options,
		})
	}

	for i, agent := range configAgents {
		color.Green("  %d. %s", i+1, agent.Name)
		fmt.Printf("     路径: %s\n", agent.Path)
		if agent.Options != "" {
			fmt.Printf("     选项: %s\n", agent.Options)
		}
		fmt.Println()
	}

	// 选择 Agent
	agentIndex, err := m.readIntInput("选择 Agent (输入序号): ")
	if err != nil || agentIndex < 1 || agentIndex > len(configAgents) {
		color.Red("无效的选择")
		m.pause()
		return
	}

	selectedAgent := agents[agentIndex-1]

	fmt.Println()
	// 选择目标进程
	fmt.Println("选择目标进程:")
	fmt.Println("  1. 指定 PID")
	fmt.Println("  2. 所有未注入的进程")

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
		// 所有未注入的进程
		agentList := []detector.Agent{{
			Path:    selectedAgent.Path,
			Options: selectedAgent.Options,
		}}

		for _, proc := range allProcs {
			if m.injector.NeedsInject(proc, agentList) {
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
	fmt.Fprintln(w, "PID\tUser\tMain Class/JAR")
	for _, proc := range targetProcs {
		main := proc.MainClass
		if proc.JarFile != "" {
			main = proc.JarFile
		}
		fmt.Fprintf(w, "%d\t%s\t%s\n", proc.PID, proc.User, main)
	}
	w.Flush()

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

	agentList := []detector.Agent{{
		Path:    selectedAgent.Path,
		Options: selectedAgent.Options,
	}}

	results := m.injector.BatchInject(ctx, targetProcs, agentList)

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
