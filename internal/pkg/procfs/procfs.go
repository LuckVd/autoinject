package procfs

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Process 进程信息
type Process struct {
	PID        int            `json:"pid"`
	Name       string         `json:"name"`
	CmdLine    []string       `json:"cmdline"`
	Envs       map[string]string `json:"envs"`
	User       string         `json:"user"`
	UID        int            `json:"uid"`
	StartTime  time.Time      `json:"start_time"`
	Cwd        string         `json:"cwd"`
	ExecPath   string         `json:"exec_path"`
	// 新增元数据
	MemoryRSS  uint64         `json:"memory_rss"`    // 驻留内存大小 (bytes)
	MemoryVMS  uint64         `json:"memory_vms"`    // 虚拟内存大小 (bytes)
	CPUPercent float64        `json:"cpu_percent"`   // CPU 使用率
	Threads    int            `json:"threads"`       // 线程数
	OpenFDs    int            `json:"open_fds"`      // 打开的文件描述符数量
}

// MemoryStats 内存统计信息
type MemoryStats struct {
	RSS    uint64  // 驻留集大小
	VMS    uint64  // 虚拟内存大小
	Shared uint64  // 共享内存大小
	Text   uint64  // 代码段大小
	Data   uint64  // 数据段大小
}

// ReadCmdline 读取进程命令行参数
func ReadCmdline(pid int) ([]string, error) {
	path := fmt.Sprintf("/proc/%d/cmdline", pid)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cmdline: %w", err)
	}

	if len(data) == 0 {
		return []string{}, nil
	}

	// cmdline 中的参数用 \0 分隔
	parts := bytes.Split(bytes.TrimRight(data, "\x00"), []byte{0})
	var cmdline []string
	for _, part := range parts {
		if len(part) > 0 {
			cmdline = append(cmdline, string(part))
		}
	}

	return cmdline, nil
}

// ReadEnviron 读取进程环境变量
func ReadEnviron(pid int) (map[string]string, error) {
	path := fmt.Sprintf("/proc/%d/environ", pid)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read environ: %w", err)
	}

	envs := make(map[string]string)
	if len(data) == 0 {
		return envs, nil
	}

	// environ 中的变量用 \0 分隔
	parts := bytes.Split(bytes.TrimRight(data, "\x00"), []byte{0})
	for _, part := range parts {
		if len(part) > 0 {
			str := string(part)
			// 分割 key=value
			if idx := strings.Index(str, "="); idx > 0 {
				key := str[:idx]
				value := str[idx+1:]
				envs[key] = value
			}
		}
	}

	return envs, nil
}

// ReadStatus 读取进程状态
func ReadStatus(pid int) (*ProcessStatus, error) {
	path := fmt.Sprintf("/proc/%d/status", pid)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	status := &ProcessStatus{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Name":
			status.Name = value
		case "State":
			status.State = value
		case "Pid":
			if pid, err := strconv.Atoi(value); err == nil {
				status.PID = pid
			}
		case "PPid":
			if ppid, err := strconv.Atoi(value); err == nil {
				status.PPID = ppid
			}
		case "Uid":
			// Uid 格式: real	effective	saved	set filesystem
			parts := strings.Fields(value)
			if len(parts) > 0 {
				if uid, err := strconv.Atoi(parts[0]); err == nil {
					status.UID = uid
				}
			}
		case "Gid":
			parts := strings.Fields(value)
			if len(parts) > 0 {
				if gid, err := strconv.Atoi(parts[0]); err == nil {
					status.GID = gid
				}
			}
		}
	}

	return status, nil
}

// ProcessStatus 进程状态
type ProcessStatus struct {
	Name   string
	State  string
	PID    int
	PPID   int
	UID    int
	GID    int
}

// ReadCwd 读取进程工作目录
func ReadCwd(pid int) (string, error) {
	path := fmt.Sprintf("/proc/%d/cwd", pid)

	cwd, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read cwd: %w", err)
	}

	return cwd, nil
}

// ReadExe 读取进程可执行文件路径
func ReadExe(pid int) (string, error) {
	path := fmt.Sprintf("/proc/%d/exe", pid)

	exe, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read exe: %w", err)
	}

	return exe, nil
}

// GetStartTime 获取进程启动时间
func GetStartTime(pid int) (time.Time, error) {
	path := fmt.Sprintf("/proc/%d/stat", pid)

	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read stat: %w", err)
	}

	// stat 文件格式，参考 man 5 proc
	// 获取 starttime（字段 22）
	parts := strings.Fields(string(data))
	if len(parts) < 22 {
		return time.Time{}, fmt.Errorf("invalid stat format")
	}

	// 获取系统启动时间
	var sysInfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysInfo); err != nil {
		return time.Time{}, fmt.Errorf("failed to get sysinfo: %w", err)
	}

	bootTime := time.Now().Add(-time.Duration(sysInfo.Uptime) * time.Second)

	// 解析 starttime（单位是 jiffies，即 clock ticks）
	startTimeTicks, err := strconv.ParseInt(parts[21], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse starttime: %w", err)
	}

	// 获取 clock ticks
	clockTicks := int64(syscall.Getpagesize())

	// 计算启动时间
	startTime := bootTime.Add(time.Duration(startTimeTicks*1000/clockTicks) * time.Millisecond)

	return startTime, nil
}

// GetUserName 获取用户名
func GetUserName(uid int) (string, error) {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", fmt.Errorf("failed to lookup user: %w", err)
	}
	return u.Username, nil
}

// IsProcessRunning 检查进程是否在运行
func IsProcessRunning(pid int) bool {
	path := fmt.Sprintf("/proc/%d", pid)
	_, err := os.Stat(path)
	return err == nil
}

// GetProcessInfo 获取完整的进程信息
func GetProcessInfo(pid int) (*Process, error) {
	// 读取命令行
	cmdline, err := ReadCmdline(pid)
	if err != nil {
		return nil, err
	}

	// 读取状态
	status, err := ReadStatus(pid)
	if err != nil {
		return nil, err
	}

	// 读取工作目录
	cwd, err := ReadCwd(pid)
	if err != nil {
		cwd = ""
	}

	// 读取可执行文件路径
	exe, err := ReadExe(pid)
	if err != nil {
		exe = ""
	}

	// 获取启动时间
	startTime, err := GetStartTime(pid)
	if err != nil {
		startTime = time.Time{}
	}

	// 读取环境变量
	envs, err := ReadEnviron(pid)
	if err != nil {
		envs = make(map[string]string)
	}

	// 获取用户名
	userName, err := GetUserName(status.UID)
	if err != nil {
		userName = strconv.Itoa(status.UID)
	}

	// 读取内存统计
	memStats, _ := ReadMemoryStats(pid)

	// 读取线程数
	threads := ReadThreads(pid)

	// 读取文件描述符数量
	openFDs := ReadOpenFDs(pid)

	// 计算 CPU 使用率
	cpuPercent := CalculateCPUPercent(pid)

	return &Process{
		PID:        pid,
		Name:       status.Name,
		CmdLine:    cmdline,
		Envs:       envs,
		User:       userName,
		UID:        status.UID,
		StartTime:  startTime,
		Cwd:        cwd,
		ExecPath:   exe,
		MemoryRSS:  memStats.RSS,
		MemoryVMS:  memStats.VMS,
		CPUPercent: cpuPercent,
		Threads:    threads,
		OpenFDs:    openFDs,
	}, nil
}

// ListAllProcesses 列出所有进程
func ListAllProcesses() ([]int, error) {
	procDir, err := os.Open("/proc")
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc: %w", err)
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	var pids []int
	for _, entry := range entries {
		// 检查是否为数字目录（PID）
		if pid, err := strconv.Atoi(entry); err == nil {
			pids = append(pids, pid)
		}
	}

	return pids, nil
}

// ReadMemoryStats 读取内存统计信息
func ReadMemoryStats(pid int) (*MemoryStats, error) {
	path := fmt.Sprintf("/proc/%d/statm", pid)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read statm: %w", err)
	}

	// statm 格式：rss vms shared text data lib dt
	fields := strings.Fields(string(data))
	if len(fields) < 6 {
		return nil, fmt.Errorf("invalid statm format")
	}

	rss, _ := strconv.ParseUint(fields[0], 10, 64)
	vms, _ := strconv.ParseUint(fields[1], 10, 64)
	shared, _ := strconv.ParseUint(fields[2], 10, 64)
	text, _ := strconv.ParseUint(fields[3], 10, 64)
	dataSize, _ := strconv.ParseUint(fields[5], 10, 64)

	// 将页面大小转换为字节（通常一页是 4KB）
	const pageSize = 4096

	return &MemoryStats{
		RSS:    rss * pageSize,
		VMS:    vms * pageSize,
		Shared: shared * pageSize,
		Text:   text * pageSize,
		Data:   dataSize * pageSize,
	}, nil
}

// ReadThreads 读取线程数
func ReadThreads(pid int) int {
	// 从 /proc/[pid]/status 读取 Threads 字段
	path := fmt.Sprintf("/proc/%d/status", pid)

	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Threads:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if threads, err := strconv.Atoi(parts[1]); err == nil {
					return threads
				}
			}
			break
		}
	}

	return 0
}

// ReadOpenFDs 读取打开的文件描述符数量
func ReadOpenFDs(pid int) int {
	// 计算 /proc/[pid]/fd 目录中的文件数量
	path := fmt.Sprintf("/proc/%d/fd", pid)

	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}

	return len(entries)
}

// CalculateCPUPercent 计算 CPU 使用率（简化版）
func CalculateCPUPercent(pid int) float64 {
	// 从 /proc/[pid]/stat 读取 CPU 时间
	path := fmt.Sprintf("/proc/%d/stat", pid)

	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	parts := strings.Fields(string(data))
	if len(parts) < 17 {
		return 0
	}

	// utime (字段 14) + stime (字段 15) 是用户态和内核态时间
	// 这里简化处理，返回 0 表示无法准确计算
	// 实际应用中需要采样两次来计算差值

	_ = parts // 避免未使用警告
	return 0
}

// FormatMemory 格式化内存大小
func FormatMemory(bytes uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

