package menu

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"iast-auto-inject/internal/core/config"
	"iast-auto-inject/internal/core/detector"
	"iast-auto-inject/internal/core/injector"

	"github.com/fatih/color"
)

// Menu 交互式菜单
type Menu struct {
	config    *config.Config
	detector  *detector.Detector
	injector  *injector.StaticInjector
	scanner   *bufio.Scanner
	running   bool
}

// NewMenu 创建菜单
func NewMenu(cfg *config.Config, det *detector.Detector, inj *injector.StaticInjector) *Menu {
	return &Menu{
		config:   cfg,
		detector: det,
		injector: inj,
		scanner:  bufio.NewScanner(os.Stdin),
		running:  true,
	}
}

// Show 显示菜单
func (m *Menu) Show() error {
	color.Green("欢迎使用 IAST Auto Inject 交互式菜单！")
	fmt.Println()

	for m.running {
		m.showMainMenu()
		if !m.running {
			break
		}
	}

	color.Yellow("感谢使用！")
	return nil
}

// showMainMenu 显示主菜单
func (m *Menu) showMainMenu() {
	m.clearScreen()
	m.printHeader()

	fmt.Println()
	fmt.Println("  1. 查看进程列表               2. 注入 Agent")
	fmt.Println("  3. 查看已注入进程           4. 配置管理")
	fmt.Println("  5. 启动守护进程             6. 系统信息")
	fmt.Println("  0. 退出")
	fmt.Println()

	choice := m.readInput("请选择 [0-6]: ")

	switch choice {
	case "1":
		m.showProcessListMenu()
	case "2":
		m.showInjectMenu()
	case "3":
		m.showInjectedProcesses()
	case "4":
		m.showConfigMenu()
	case "5":
		m.showDaemonMenu()
	case "6":
		m.showSystemInfo()
	case "0", "q", "Q":
		m.running = false
	default:
		color.Red("无效的选择，请重新输入")
		m.pause()
	}
}

// printHeader 打印头部
func (m *Menu) printHeader() {
	color.Cyan("╔═══════════════════════════════════════════════════════════════╗")
	color.Cyan("║         IAST Auto Inject - Java Agent 注入工具 v1.0          ║")
	color.Cyan("╠═══════════════════════════════════════════════════════════════╣")
	color.Cyan("║                                                               ║")
}

// readInput 读取输入
func (m *Menu) readInput(prompt string) string {
	fmt.Print(prompt)
	m.scanner.Scan()
	return strings.TrimSpace(m.scanner.Text())
}

// readIntInput 读取整数输入
func (m *Menu) readIntInput(prompt string) (int, error) {
	input := m.readInput(prompt)
	return strconv.Atoi(input)
}

// pause 暂停
func (m *Menu) pause() {
	fmt.Println()
	fmt.Print("按 Enter 键继续...")
	m.scanner.Scan()
}

// clearScreen 清屏（简单实现）
func (m *Menu) clearScreen() {
	// 打印多个空行来模拟清屏
	for i := 0; i < 3; i++ {
		fmt.Println()
	}
}

// printBox 打印边框
func (m *Menu) printBox(title string, content string) {
	color.Cyan("╔═══════════════════════════════════════════════════════════════╗")
	color.Cyan("║ %-63s ║", title)
	color.Cyan("╠═══════════════════════════════════════════════════════════════╣")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// 截断过长的行
		if len(line) > 63 {
			line = line[:60] + "..."
		}
		color.Cyan("║ %-63s ║", line)
	}

	color.Cyan("╚═══════════════════════════════════════════════════════════════╝")
}
