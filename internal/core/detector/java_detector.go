package detector

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"iast-auto-inject/internal/core/config"
	"iast-auto-inject/internal/pkg/logger"
	"iast-auto-inject/internal/pkg/procfs"

	"go.uber.org/zap"
)

// Agent Java Agent 信息
type Agent struct {
	Path       string `json:"path"`
	Options    string `json:"options"`
	FullParam  string `json:"full_param"`
}

// JavaProcess Java 进程信息
type JavaProcess struct {
	PID        int       `json:"pid"`
	Name       string    `json:"name"`
	User       string    `json:"user"`
	UID        int       `json:"uid"`
	CmdLine    []string  `json:"cmdline"`
	Envs       map[string]string `json:"envs"`
	StartTime  string    `json:"start_time"`
	Cwd        string    `json:"cwd"`
	ExecPath   string    `json:"exec_path"`
	Agents     []Agent   `json:"agents"`
	MainClass  string    `json:"main_class"`
	JarFile    string    `json:"jar_file"`
	// 进程元数据
	MemoryRSS  uint64    `json:"memory_rss"`    // 驻留内存大小 (bytes)
	MemoryVMS  uint64    `json:"memory_vms"`    // 虚拟内存大小 (bytes)
	CPUPercent float64   `json:"cpu_percent"`   // CPU 使用率
	Threads    int       `json:"threads"`       // 线程数
	OpenFDs    int       `json:"open_fds"`      // 打开的文件描述符数量
}

// ProcessFilter 进程过滤器
type ProcessFilter struct {
	PIDs         []int
	Names        []string
	Users        []string
	Patterns     []string
	HasAgent     *bool // true: 有agent, false: 无agent, nil: 不限制
	MinUptime    *int  // 最小运行时间（秒）
}

// Detector 进程检测器
type Detector struct {
	config *config.Config
}

// NewDetector 创建检测器
func NewDetector(cfg *config.Config) *Detector {
	return &Detector{
		config: cfg,
	}
}

// DiscoverJavaProcesses 发现所有 Java 进程
func (d *Detector) DiscoverJavaProcesses(ctx context.Context, filter *ProcessFilter) ([]*JavaProcess, error) {
	pids, err := procfs.ListAllProcesses()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}

	logger.Debug("Found total processes", zap.Int("count", len(pids)))

	var javaProcesses []*JavaProcess
	for _, pid := range pids {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 获取进程信息
		procInfo, err := procfs.GetProcessInfo(pid)
		if err != nil {
			continue
		}

		// 判断是否为 Java 进程
		if !d.isJavaProcess(procInfo) {
			continue
		}

		// 解析 Java 进程信息
		javaProc := d.parseJavaProcess(procInfo)

		// 应用过滤器
		if filter != nil && !d.matchFilter(javaProc, filter) {
			continue
		}

		javaProcesses = append(javaProcesses, javaProc)
	}

	logger.Info("Discovered Java processes", zap.Int("count", len(javaProcesses)))

	return javaProcesses, nil
}

// isJavaProcess 判断是否为 Java 进程
func (d *Detector) isJavaProcess(proc *procfs.Process) bool {
	// 检查可执行文件名
	if filepath.Base(proc.ExecPath) == "java" {
		return true
	}

	// 检查命令行是否包含 java 特征
	for _, arg := range proc.CmdLine {
		if strings.Contains(arg, "java") || strings.HasSuffix(arg, ".jar") {
			return true
		}
	}

	return false
}

// parseJavaProcess 解析 Java 进程信息
func (d *Detector) parseJavaProcess(proc *procfs.Process) *JavaProcess {
	javaProc := &JavaProcess{
		PID:        proc.PID,
		Name:       proc.Name,
		User:       proc.User,
		UID:        proc.UID,
		CmdLine:    proc.CmdLine,
		Envs:       proc.Envs,
		StartTime:  proc.StartTime.Format("2006-01-02 15:04:05"),
		Cwd:        proc.Cwd,
		ExecPath:   proc.ExecPath,
		Agents:     d.extractAgents(proc.CmdLine),
		MemoryRSS:  proc.MemoryRSS,
		MemoryVMS:  proc.MemoryVMS,
		CPUPercent: proc.CPUPercent,
		Threads:    proc.Threads,
		OpenFDs:    proc.OpenFDs,
	}

	// 解析主类或 JAR 文件
	for _, arg := range proc.CmdLine {
		if strings.HasSuffix(arg, ".jar") {
			javaProc.JarFile = arg
			break
		}
		if !strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") {
			javaProc.MainClass = arg
		}
	}

	return javaProc
}

// extractAgents 从命令行中提取 Agent 信息（仅检测 SecPoint.jar）
func (d *Detector) extractAgents(cmdline []string) []Agent {
	var agents []Agent

	for _, arg := range cmdline {
		// -javaagent: 或 -javaagent=
		if strings.HasPrefix(arg, "-javaagent:") || strings.HasPrefix(arg, "-javaagent=") {
			// 提取路径和选项
			agent := parseAgentParam(arg)
			// 只添加 SecPoint.jar 相关的 Agent
			if agent != nil && strings.Contains(agent.Path, "SecPoint.jar") {
				agents = append(agents, *agent)
			}
		}
	}

	return agents
}

// parseAgentParam 解析 -javaagent 参数
func parseAgentParam(arg string) *Agent {
	// 移除 -javaagent: 或 -javaagent= 前缀
	var param string
	if strings.HasPrefix(arg, "-javaagent:") {
		param = strings.TrimPrefix(arg, "-javaagent:")
	} else if strings.HasPrefix(arg, "-javaagent=") {
		param = strings.TrimPrefix(arg, "-javaagent=")
	} else {
		return nil
	}

	// 分离路径和选项（选项以 = 开头）
	parts := strings.SplitN(param, "=", 2)
	path := parts[0]
	options := ""
	if len(parts) == 2 {
		options = parts[1]
	}

	return &Agent{
		Path:      path,
		Options:   options,
		FullParam: arg,
	}
}

// HasAgent 检查进程是否已附加指定 Agent（仅支持 SecPoint.jar）
func (d *Detector) HasAgent(javaProc *JavaProcess, agentPath string) bool {
	// 只检查 SecPoint.jar
	if !strings.Contains(agentPath, "SecPoint.jar") {
		return false
	}

	// 规范化路径
	normalizedPath := filepath.Clean(agentPath)

	for _, agent := range javaProc.Agents {
		if filepath.Clean(agent.Path) == normalizedPath {
			return true
		}
	}

	return false
}

// HasSecPointAgent 检查进程是否已附加 SecPoint Agent
func (d *Detector) HasSecPointAgent(javaProc *JavaProcess) bool {
	return len(javaProc.Agents) > 0
}

// matchFilter 检查进程是否匹配过滤条件
func (d *Detector) matchFilter(javaProc *JavaProcess, filter *ProcessFilter) bool {
	// PID 过滤
	if len(filter.PIDs) > 0 {
		found := false
		for _, pid := range filter.PIDs {
			if javaProc.PID == pid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 用户过滤
	if len(filter.Users) > 0 {
		found := false
		for _, user := range filter.Users {
			if javaProc.User == user {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Agent 状态过滤
	if filter.HasAgent != nil {
		hasAny := len(javaProc.Agents) > 0
		if *filter.HasAgent != hasAny {
			return false
		}
	}

	// 模式匹配
	if len(filter.Patterns) > 0 {
		matched := false
		for _, pattern := range filter.Patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				logger.Warn("Invalid regex pattern", zap.String("pattern", pattern), zap.Error(err))
				continue
			}

			// 匹配进程名、JAR 文件、主类
			if re.MatchString(javaProc.Name) ||
				re.MatchString(javaProc.JarFile) ||
				re.MatchString(javaProc.MainClass) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// CheckPermissions 检查是否有权限操作进程
func (d *Detector) CheckPermissions(javaProc *JavaProcess) error {
	// 检查是否是当前用户的进程
	uid := syscall.Getuid()

	if javaProc.UID != uid && uid != 0 {
		return fmt.Errorf("insufficient permissions for process %d (owned by %s, requires root)", javaProc.PID, javaProc.User)
	}

	return nil
}

// IsExcluded 检查进程是否在排除列表中
func (d *Detector) IsExcluded(javaProc *JavaProcess) bool {
	for _, rule := range d.config.Exclude {
		// PID 排除
		for _, pid := range rule.PIDs {
			if javaProc.PID == pid {
				return true
			}
		}

		// 用户排除
		for _, user := range rule.Users {
			if javaProc.User == user {
				return true
			}
		}

		// 模式排除
		for _, pattern := range rule.Patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(javaProc.Name) ||
				re.MatchString(javaProc.JarFile) ||
				re.MatchString(javaProc.MainClass) {
				return true
			}
		}
	}

	return false
}
