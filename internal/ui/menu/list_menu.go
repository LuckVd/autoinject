package menu

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"iast-auto-inject/internal/core/detector"

	"github.com/fatih/color"
)

// showProcessListMenu 显示进程列表菜单
func (m *Menu) showProcessListMenu() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                    Java 进程列表")
	fmt.Println()

	// 发现进程
	ctx := context.Background()
	procs, err := m.detector.DiscoverJavaProcesses(ctx, nil)
	if err != nil {
		color.Red("发现进程失败: %v", err)
		m.pause()
		return
	}

	if len(procs) == 0 {
		color.Yellow("未发现 Java 进程")
		m.pause()
		return
	}

	// 显示进程列表
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PID\tUser\tMain Class/JAR\t\tAgent Status")
	fmt.Fprintln(w, "---\t----\t-------------\t\t------------")

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	for _, proc := range procs {
		var agentStatus string
		if len(proc.Agents) > 0 {
			agentStatus = green(fmt.Sprintf("✓ %d 个 Agent", len(proc.Agents)))
		} else {
			agentStatus = red("✗ 无")
		}

		main := proc.MainClass
		if proc.JarFile != "" {
			main = proc.JarFile
		}
		if len(main) > 30 {
			main = main[:27] + "..."
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t\t%s\n",
			proc.PID, proc.User, main, agentStatus)
	}

	w.Flush()

	fmt.Printf("\n总计: %d 个 Java 进程\n", len(procs))
	m.pause()
}

// showInjectedProcesses 显示已注入进程
func (m *Menu) showInjectedProcesses() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                    已注入 Agent 的进程")
	fmt.Println()

	ctx := context.Background()
	procs, err := m.detector.DiscoverJavaProcesses(ctx, nil)
	if err != nil {
		color.Red("发现进程失败: %v", err)
		m.pause()
		return
	}

	var injectedProcs []*detector.JavaProcess
	for _, proc := range procs {
		if len(proc.Agents) > 0 {
			injectedProcs = append(injectedProcs, proc)
		}
	}

	if len(injectedProcs) == 0 {
		color.Yellow("没有已注入 Agent 的进程")
		m.pause()
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PID\tUser\tMain Class/JAR\t\tAgents")
	fmt.Fprintln(w, "---\t----\t-------------\t\t------")

	for _, proc := range injectedProcs {
		main := proc.MainClass
		if proc.JarFile != "" {
			main = proc.JarFile
		}
		if len(main) > 25 {
			main = main[:22] + "..."
		}

		agents := ""
		for i, agent := range proc.Agents {
			if i > 0 {
				agents += ", "
			}
			agents += agent.Path
		}
		if len(agents) > 40 {
			agents = agents[:37] + "..."
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t\t%s\n",
			proc.PID, proc.User, main, agents)
	}

	w.Flush()

	fmt.Printf("\n总计: %d 个已注入进程\n", len(injectedProcs))
	m.pause()
}

// showSystemInfo 显示系统信息
func (m *Menu) showSystemInfo() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                        系统信息")
	fmt.Println()

	fmt.Println("程序信息:")
	fmt.Printf("  版本: %s\n", m.config.Version)
	fmt.Printf("  调试模式: %v\n", m.config.Debug)
	fmt.Println()

	fmt.Println("Agent 配置:")
	for _, agent := range m.config.Agents {
		var status string
		if agent.Enabled {
			status = "启用"
			color.Green(status)
		} else {
			status = "禁用"
			color.Red(status)
		}
		fmt.Printf("  - %s: %s [%s]\n", agent.Name, agent.Path, status)
	}
	fmt.Println()

	fmt.Println("进程配置:")
	fmt.Printf("  扫描间隔: %v\n", m.config.Process.ScanInterval)
	fmt.Printf("  自动重启: %v\n", m.config.Process.AutoRestart)
	fmt.Println()

	fmt.Println("守护进程配置:")
	var status string
	if m.config.Daemon.Enabled {
		status = "启用"
		color.Green(status)
	} else {
		status = "禁用"
		color.Red(status)
	}
	fmt.Printf("  状态: %s\n", status)
	fmt.Printf("  间隔: %v\n", m.config.Daemon.Interval)

	m.pause()
}
