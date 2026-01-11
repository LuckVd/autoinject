package menu

import (
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/fatih/color"
)

// showConfigMenu 显示配置管理菜单
func (m *Menu) showConfigMenu() {
	for {
		m.clearScreen()
		m.printHeader()

		fmt.Println()
		color.Cyan("                     配置管理")
		fmt.Println()

		fmt.Println("  1. 查看当前配置")
		fmt.Println("  2. 查看 Agent 配置")
		fmt.Println("  3. 查看进程配置")
		fmt.Println("  4. 查看守护进程配置")
		fmt.Println("  0. 返回主菜单")
		fmt.Println()

		choice := m.readInput("请选择 [0-4]: ")

		switch choice {
		case "1":
			m.showCurrentConfig()
		case "2":
			m.showAgentConfig()
		case "3":
			m.showProcessConfig()
		case "4":
			m.showDaemonConfig()
		case "0":
			return
		default:
			color.Red("无效的选择")
			m.pause()
		}
	}
}

// showCurrentConfig 显示当前配置
func (m *Menu) showCurrentConfig() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                     当前配置")
	fmt.Println()

	fmt.Printf("版本: %s\n", m.config.Version)
	fmt.Printf("调试模式: %v\n", m.config.Debug)

	fmt.Println()
	fmt.Println("日志配置:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  级别:\t%s\n", m.config.Log.Level)
	fmt.Fprintf(w, "  格式:\t%s\n", m.config.Log.Format)
	fmt.Fprintf(w, "  输出:\t%s\n", m.config.Log.Output)
	w.Flush()

	m.pause()
}

// showAgentConfig 显示 Agent 配置
func (m *Menu) showAgentConfig() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                     Agent 配置")
	fmt.Println()

	if len(m.config.Agents) == 0 {
		color.Yellow("没有配置 Agent")
		m.pause()
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "名称\t路径\t选项\t启用\t优先级")

	for _, agent := range m.config.Agents {
		var enabled string
		if agent.Enabled {
			enabled = "是"
			color.Green(enabled)
		} else {
			enabled = "否"
			color.Red(enabled)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			agent.Name, agent.Path, agent.Options, enabled, agent.Priority)
	}

	w.Flush()
	m.pause()
}

// showProcessConfig 显示进程配置
func (m *Menu) showProcessConfig() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                     进程配置")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "扫描间隔:\t%v\n", m.config.Process.ScanInterval)
	fmt.Fprintf(w, "自动重启:\t%v\n", m.config.Process.AutoRestart)
	fmt.Fprintln(w, "\n包含模式:")
	for _, pattern := range m.config.Process.IncludePattern {
		fmt.Fprintf(w, "  - %s\n", pattern)
	}
	fmt.Fprintln(w, "\n用户过滤:")
	for _, user := range m.config.Process.UserFilter {
		fmt.Fprintf(w, "  - %s\n", user)
	}
	w.Flush()

	m.pause()
}

// showDaemonConfig 显示守护进程配置
func (m *Menu) showDaemonConfig() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                   守护进程配置")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var enabled string
	if m.config.Daemon.Enabled {
		enabled = "是"
		color.Green(enabled)
	} else {
		enabled = "否"
		color.Red(enabled)
	}
	fmt.Fprintf(w, "启用:\t%s\n", enabled)
	fmt.Fprintf(w, "间隔:\t%v\n", m.config.Daemon.Interval)
	fmt.Fprintf(w, "日志级别:\t%s\n", m.config.Daemon.LogLevel)
	fmt.Fprintf(w, "PID 文件:\t%s\n", m.config.Daemon.PidFile)
	w.Flush()

	m.pause()
}

// showDaemonMenu 显示守护进程菜单
func (m *Menu) showDaemonMenu() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	color.Cyan("                   守护进程管理")
	fmt.Println()

	fmt.Println("  1. 查看守护进程状态")
	fmt.Println("  2. 启动守护进程")
	fmt.Println("  3. 停止守护进程")
	fmt.Println("  4. 重启守护进程")
	fmt.Println("  5. 查看日志")
	fmt.Println("  0. 返回主菜单")
	fmt.Println()

	choice := m.readInput("请选择 [0-5]: ")

	switch choice {
	case "1":
		m.showDaemonStatus()
	case "2":
		m.startDaemon()
	case "3":
		m.stopDaemon()
	case "4":
		m.restartDaemon()
	case "5":
		m.viewDaemonLogs()
	case "0":
		return
	default:
		color.Red("无效的选择")
		m.pause()
	}
}

// showDaemonStatus 显示守护进程状态
func (m *Menu) showDaemonStatus() {
	fmt.Println()
	color.Cyan("守护进程状态:")
	fmt.Println()

	// 使用 systemctl 检查状态
	cmd := exec.Command("systemctl", "is-active", "iast-auto-inject")
	output, _ := cmd.Output()

	status := string(output)
	if len(status) > 0 {
		status = status[:len(status)-1] // 去掉换行符
		if status == "active" {
			color.Green("● 运行中: %s", status)
		} else {
			color.Red("● 未运行: %s", status)
		}
	} else {
		color.Yellow("○ 未安装服务")
	}

	// 显示详细状态
	cmd = exec.Command("systemctl", "status", "iast-auto-inject", "--no-pager")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	m.pause()
}

// startDaemon 启动守护进程
func (m *Menu) startDaemon() {
	fmt.Println()
	color.Cyan("启动守护进程...")
	fmt.Println()

	cmd := exec.Command("sudo", "systemctl", "start", "iast-auto-inject")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if err != nil {
		color.Red("启动失败: %v", err)
	} else {
		color.Green("启动成功")
	}

	m.pause()
}

// stopDaemon 停止守护进程
func (m *Menu) stopDaemon() {
	fmt.Println()
	color.Cyan("停止守护进程...")
	fmt.Println()

	cmd := exec.Command("sudo", "systemctl", "stop", "iast-auto-inject")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if err != nil {
		color.Red("停止失败: %v", err)
	} else {
		color.Green("停止成功")
	}

	m.pause()
}

// restartDaemon 重启守护进程
func (m *Menu) restartDaemon() {
	fmt.Println()
	color.Cyan("重启守护进程...")
	fmt.Println()

	cmd := exec.Command("sudo", "systemctl", "restart", "iast-auto-inject")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if err != nil {
		color.Red("重启失败: %v", err)
	} else {
		color.Green("重启成功")
	}

	m.pause()
}

// viewDaemonLogs 查看日志
func (m *Menu) viewDaemonLogs() {
	fmt.Println()
	color.Cyan("守护进程日志 (最近 50 行):")
	fmt.Println()

	cmd := exec.Command("journalctl", "-u", "iast-auto-inject", "-n", "50", "--no-pager")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	m.pause()
}
