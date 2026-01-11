package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"iast-auto-inject/internal/pkg/logger"

	"go.uber.org/zap"
)

// Manager 进程管理器
type Manager struct {
	gracePeriod time.Duration
	killTimeout time.Duration
	maxRetries  int
	verifyWait  time.Duration
}

// NewManager 创建进程管理器
func NewManager(gracePeriod, killTimeout, verifyWait time.Duration, maxRetries int) *Manager {
	return &Manager{
		gracePeriod: gracePeriod,
		killTimeout: killTimeout,
		maxRetries:  maxRetries,
		verifyWait:  verifyWait,
	}
}

// StopOptions 停止选项
type StopOptions struct {
	Signal  syscall.Signal
	Timeout time.Duration
	Force   bool
}

// StartOptions 启动选项
type StartOptions struct {
	Cwd  string
	Envs map[string]string
}

// RestartOptions 重启选项
type RestartOptions struct {
	GracePeriod time.Duration
	KillTimeout time.Duration
	VerifyWait  time.Duration
	MaxRetries  int
}

// Stop 停止进程
func (m *Manager) Stop(ctx context.Context, pid int, opts *StopOptions) error {
	if opts == nil {
		opts = &StopOptions{
			Signal:  syscall.SIGTERM,
			Timeout: m.killTimeout,
		}
	}

	logger.Info("Stopping process", zap.Int("pid", pid), zap.String("signal", opts.Signal.String()))

	// 查找进程
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// 发送信号
	if err := proc.Signal(opts.Signal); err != nil {
		return fmt.Errorf("failed to send signal to process %d: %w", pid, err)
	}

	// 等待进程退出
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = m.killTimeout
	}

	done := make(chan error, 1)
	go func() {
		_, err := proc.Wait()
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		// 超时，强制杀死
		if opts.Force {
			logger.Warn("Process stop timeout, killing", zap.Int("pid", pid))
			if err := proc.Kill(); err != nil {
				return fmt.Errorf("failed to kill process %d: %w", pid, err)
			}
			return nil
		}
		return fmt.Errorf("timeout waiting for process %d to exit", pid)
	case err := <-done:
		if err != nil && !isProcessExitedError(err) {
			return fmt.Errorf("process %d wait error: %w", pid, err)
		}
		logger.Info("Process stopped", zap.Int("pid", pid))
		return nil
	}
}

// Start 启动进程
func (m *Manager) Start(ctx context.Context, cmdLine []string, opts *StartOptions) (int, error) {
	if len(cmdLine) == 0 {
		return 0, fmt.Errorf("command line is empty")
	}

	if opts == nil {
		opts = &StartOptions{}
	}

	logger.Info("Starting process", zap.Strings("cmdline", cmdLine), zap.String("cwd", opts.Cwd))

	// 创建命令
	cmd := exec.CommandContext(ctx, cmdLine[0], cmdLine[1:]...)

	// 设置工作目录
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}

	// 设置环境变量
	if opts.Envs != nil {
		env := os.Environ()
		for k, v := range opts.Envs {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	pid := cmd.Process.Pid
	logger.Info("Process started", zap.Int("pid", pid))

	return pid, nil
}

// Restart 重启进程
func (m *Manager) Restart(ctx context.Context, pid int, newCmdLine []string, opts *RestartOptions) (int, error) {
	if opts == nil {
		opts = &RestartOptions{
			GracePeriod: m.gracePeriod,
			KillTimeout: m.killTimeout,
			VerifyWait:  m.verifyWait,
			MaxRetries:  m.maxRetries,
		}
	}

	// 获取原进程信息
	procInfo, err := m.getProcessInfo(pid)
	if err != nil {
		return 0, fmt.Errorf("failed to get process info: %w", err)
	}

	logger.Info("Restarting process",
		zap.Int("old_pid", pid),
		zap.Strings("cmdline", newCmdLine))

	// 停止原进程
	stopOpts := &StopOptions{
		Signal:  syscall.SIGTERM,
		Timeout: opts.GracePeriod,
		Force:   true,
	}

	if err := m.Stop(ctx, pid, stopOpts); err != nil {
		logger.Warn("Failed to stop process gracefully", zap.Int("pid", pid), zap.Error(err))
		// 继续尝试启动
	}

	// 启动新进程
	startOpts := &StartOptions{
		Cwd:  procInfo.Cwd,
		Envs: procInfo.Envs,
	}

	var newPid int
	var lastErr error

	// 重试机制
	for i := 0; i < opts.MaxRetries; i++ {
		newPid, err = m.Start(ctx, newCmdLine, startOpts)
		if err == nil {
			break
		}
		lastErr = err
		logger.Warn("Failed to start process, retrying",
			zap.Int("attempt", i+1),
			zap.Int("max_retries", opts.MaxRetries),
			zap.Error(err))
		time.Sleep(time.Second)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to start process after %d retries: %w", opts.MaxRetries, lastErr)
	}

	// 验证新进程
	if opts.VerifyWait > 0 {
		logger.Info("Waiting for new process to stabilize",
			zap.Int("new_pid", newPid),
			zap.Duration("wait", opts.VerifyWait))
		time.Sleep(opts.VerifyWait)

		// 检查新进程是否还在运行
		if !m.isRunning(newPid) {
			return 0, fmt.Errorf("new process %d exited during verification", newPid)
		}
	}

	logger.Info("Process restarted successfully",
		zap.Int("old_pid", pid),
		zap.Int("new_pid", newPid))

	return newPid, nil
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID     int
	CmdLine []string
	Cwd     string
	Envs    map[string]string
}

// getProcessInfo 获取进程信息
func (m *Manager) getProcessInfo(pid int) (*ProcessInfo, error) {
	// 读取命令行
	cmdline, err := readCmdline(pid)
	if err != nil {
		return nil, err
	}

	// 读取工作目录
	cwd, err := readCwd(pid)
	if err != nil {
		cwd = ""
	}

	// 读取环境变量
	envs, err := readEnvs(pid)
	if err != nil {
		envs = make(map[string]string)
	}

	return &ProcessInfo{
		PID:     pid,
		CmdLine: cmdline,
		Cwd:     cwd,
		Envs:    envs,
	}, nil
}

// isRunning 检查进程是否在运行
func (m *Manager) isRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 发送信号 0 检查进程是否存在
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// readCmdline 读取进程命令行
func readCmdline(pid int) ([]string, error) {
	path := fmt.Sprintf("/proc/%d/cmdline", pid)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cmdline: %w", err)
	}

	if len(data) == 0 {
		return []string{}, nil
	}

	var cmdline []string
	start := 0
	for i, b := range data {
		if b == 0 {
			if start < i {
				cmdline = append(cmdline, string(data[start:i]))
			}
			start = i + 1
		}
	}

	return cmdline, nil
}

// readCwd 读取进程工作目录
func readCwd(pid int) (string, error) {
	path := fmt.Sprintf("/proc/%d/cwd", pid)
	cwd, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read cwd: %w", err)
	}
	return cwd, nil
}

// readEnvs 读取进程环境变量
func readEnvs(pid int) (map[string]string, error) {
	path := fmt.Sprintf("/proc/%d/environ", pid)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read environ: %w", err)
	}

	envs := make(map[string]string)
	start := 0
	for i, b := range data {
		if b == 0 {
			if start < i {
				part := string(data[start:i])
				// 解析 key=value
				for j, c := range part {
					if c == '=' {
						key := part[:j]
						value := part[j+1:]
						envs[key] = value
						break
					}
				}
			}
			start = i + 1
		}
	}

	return envs, nil
}

// isProcessExitedError 检查是否是进程退出错误
func isProcessExitedError(err error) bool {
	if err == nil {
		return false
	}
	// 检查是否是 "signal: killed" 等错误
	return true
}
