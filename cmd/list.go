package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"iast-auto-inject/internal/core/detector"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	listPid        int
	listAgent      string
	listNoAgent    bool
	listFormat     string
)

// listCmd list 命令
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出 Java 进程及其 agent 状态",
	Long:  `列出系统中所有 Java 进程及其已附加的 javaagent 状态`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().IntVarP(&listPid, "pid", "p", 0, "显示指定 PID 的详细信息")
	listCmd.Flags().StringVarP(&listAgent, "agent", "a", "", "只显示已附加指定 agent 的进程")
	listCmd.Flags().BoolVar(&listNoAgent, "no-agent", false, "只显示未附加 agent 的进程")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "输出格式 (table, json)")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 创建检测器
	det := detector.NewDetector(GetConfig())

	// 构建过滤器
	filter := &detector.ProcessFilter{}
	if listPid > 0 {
		filter.PIDs = []int{listPid}
	}

	// 发现进程
	procs, err := det.DiscoverJavaProcesses(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to discover processes: %w", err)
	}

	// 过滤
	var filtered []*detector.JavaProcess
	for _, proc := range procs {
		if listNoAgent && len(proc.Agents) > 0 {
			continue
		}
		if listAgent != "" {
			has := false
			for _, agent := range proc.Agents {
				if agent.Path == listAgent {
					has = true
					break
				}
			}
			if !has {
				continue
			}
		}
		filtered = append(filtered, proc)
	}

	// 显示结果
	switch listFormat {
	case "json":
		printJSON(filtered)
	default:
		printTable(filtered)
	}

	return nil
}

// printTable 打印表格格式
func printTable(procs []*detector.JavaProcess) {
	if len(procs) == 0 {
		color.Yellow("No Java processes found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// 表头
	fmt.Fprintln(w, "PID\tUser\tMain Class/JAR\tAgents")

	// 数据行
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	for _, proc := range procs {
		agentStatus := red("✗ None")
		if len(proc.Agents) > 0 {
			agentStr := ""
			for i, agent := range proc.Agents {
				if i > 0 {
					agentStr += ", "
				}
				agentStr += green("✓ ") + agent.Path
			}
			agentStatus = agentStr
		}

		main := proc.MainClass
		if proc.JarFile != "" {
			main = proc.JarFile
		}
		if main == "" {
			main = "unknown"
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			proc.PID,
			proc.User,
			truncate(main, 30),
			agentStatus)
	}

	w.Flush()

	fmt.Printf("\nTotal: %d Java process(es)\n", len(procs))
}

// printJSON 打印 JSON 格式
func printJSON(procs []*detector.JavaProcess) {
	// 简化实现
	for _, proc := range procs {
		agentStr := "none"
		if len(proc.Agents) > 0 {
			agentStr = ""
			for i, agent := range proc.Agents {
				if i > 0 {
					agentStr += ", "
				}
				agentStr += agent.Path
			}
		}
		fmt.Printf("PID: %d, User: %s, Agents: [%s]\n", proc.PID, proc.User, agentStr)
	}
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
