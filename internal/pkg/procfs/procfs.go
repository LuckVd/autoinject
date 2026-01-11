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

	return &Process{
		PID:       pid,
		Name:      status.Name,
		CmdLine:   cmdline,
		Envs:      envs,
		User:      userName,
		UID:       status.UID,
		StartTime: startTime,
		Cwd:       cwd,
		ExecPath:  exe,
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
