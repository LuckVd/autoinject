package injector

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"iast-auto-inject/internal/core/config"
	"iast-auto-inject/internal/core/detector"
	"iast-auto-inject/internal/core/process"
	"iast-auto-inject/internal/pkg/logger"

	"go.uber.org/zap"
)

// StaticInjector 静态注入器
type StaticInjector struct {
	config      *config.Config
	detector    *detector.Detector
	processMgr  *process.Manager
}

// InjectResult 注入结果
type InjectResult struct {
	PID         int      `json:"pid"`
	Success     bool     `json:"success"`
	OldCmdLine  []string `json:"old_cmdline"`
	NewCmdLine  []string `json:"new_cmdline"`
	NewPID      int      `json:"new_pid"`
	OldAgents   []detector.Agent `json:"old_agents"`
	NewAgents   []detector.Agent `json:"new_agents"`
	Error       error    `json:"error,omitempty"`
	Message     string   `json:"message"`
}

// NewStaticInjector 创建静态注入器
func NewStaticInjector(cfg *config.Config, det *detector.Detector, mgr *process.Manager) *StaticInjector {
	return &StaticInjector{
		config:     cfg,
		detector:   det,
		processMgr: mgr,
	}
}

// Inject 向指定进程注入 Agent
func (s *StaticInjector) Inject(ctx context.Context, javaProc *detector.JavaProcess, agents []detector.Agent) (*InjectResult, error) {
	logger.Info("Injecting agent",
		zap.Int("pid", javaProc.PID),
		zap.Int("agents", len(agents)))

	result := &InjectResult{
		PID:        javaProc.PID,
		OldCmdLine: javaProc.CmdLine,
		OldAgents:  javaProc.Agents,
	}

	// 检查权限
	if err := s.detector.CheckPermissions(javaProc); err != nil {
		result.Error = err
		result.Message = fmt.Sprintf("Permission denied: %v", err)
		return result, err
	}

	// 构建新的命令行
	newCmdLine := s.buildNewCmdLine(javaProc.CmdLine, agents)
	result.NewCmdLine = newCmdLine

	// 重启进程
	restartOpts := &process.RestartOptions{
		GracePeriod: s.config.Restart.GracePeriod,
		KillTimeout: s.config.Restart.KillTimeout,
		VerifyWait:  s.config.Restart.VerifyWait,
		MaxRetries:  s.config.Restart.MaxRetries,
	}

	newPid, err := s.processMgr.Restart(ctx, javaProc.PID, newCmdLine, restartOpts)
	if err != nil {
		result.Error = err
		result.Message = fmt.Sprintf("Failed to restart process: %v", err)
		return result, err
	}

	result.NewPID = newPid
	result.Success = true
	result.Message = fmt.Sprintf("Successfully injected agent and restarted process (new PID: %d)", newPid)

	// 获取新进程的 Agent 状态
	if procInfo, err := s.detector.DiscoverJavaProcesses(ctx, &detector.ProcessFilter{PIDs: []int{newPid}}); err == nil && len(procInfo) > 0 {
		result.NewAgents = procInfo[0].Agents
	}

	logger.Info("Agent injected successfully",
		zap.Int("old_pid", javaProc.PID),
		zap.Int("new_pid", newPid),
		zap.Int("agents", len(agents)))

	return result, nil
}

// BatchInject 批量注入多个进程
func (s *StaticInjector) BatchInject(ctx context.Context, javaProcs []*detector.JavaProcess, agents []detector.Agent) []*InjectResult {
	results := make([]*InjectResult, 0, len(javaProcs))

	for _, javaProc := range javaProcs {
		select {
		case <-ctx.Done():
			logger.Warn("Batch inject cancelled", zap.Error(ctx.Err()))
			break
		default:
		}

		result, err := s.Inject(ctx, javaProc, agents)
		if err != nil {
			logger.Error("Failed to inject agent",
				zap.Int("pid", javaProc.PID),
				zap.Error(err))
		}
		results = append(results, result)
	}

	return results
}

// buildNewCmdLine 构建新的命令行（插入 javaagent 参数）
func (s *StaticInjector) buildNewCmdLine(oldCmdLine []string, agents []detector.Agent) []string {
	newCmdLine := make([]string, 0, len(oldCmdLine)+len(agents))

	// 查找 java 命令的位置
	javaIdx := -1
	for i, arg := range oldCmdLine {
		if strings.Contains(filepath.Base(arg), "java") {
			javaIdx = i
			break
		}
	}

	// 如果找不到 java 命令，直接在开头插入
	if javaIdx == -1 {
		javaIdx = 0
	}

	// 复制 java 命令
	newCmdLine = append(newCmdLine, oldCmdLine[:javaIdx+1]...)

	// 插入 agent 参数
	for _, agent := range agents {
		param := s.buildAgentParam(agent)
		newCmdLine = append(newCmdLine, param)
	}

	// 复制剩余参数
	newCmdLine = append(newCmdLine, oldCmdLine[javaIdx+1:]...)

	return newCmdLine
}

// buildAgentParam 构建 agent 参数
func (s *StaticInjector) buildAgentParam(agent detector.Agent) string {
	if agent.Options != "" {
		return fmt.Sprintf("-javaagent:%s=%s", agent.Path, agent.Options)
	}
	return fmt.Sprintf("-javaagent:%s", agent.Path)
}

// GetAgentsFromConfig 从配置中获取启用的 Agent
func (s *StaticInjector) GetAgentsFromConfig() []detector.Agent {
	enabledAgents := s.config.GetEnabledAgents()
	agents := make([]detector.Agent, 0, len(enabledAgents))

	for _, cfg := range enabledAgents {
		agents = append(agents, detector.Agent{
			Path:    cfg.Path,
			Options: cfg.Options,
		})
	}

	return agents
}

// NeedsInject 检查进程是否需要注入
func (s *StaticInjector) NeedsInject(javaProc *detector.JavaProcess, agents []detector.Agent) bool {
	// 检查是否在排除列表中
	if s.detector.IsExcluded(javaProc) {
		return false
	}

	// 检查是否已经有所有需要的 agent
	for _, agent := range agents {
		if !s.detector.HasAgent(javaProc, agent.Path) {
			return true
		}
	}

	return false
}

// Validate 验证注入结果
func (s *StaticInjector) Validate(ctx context.Context, pid int, expectedAgents []detector.Agent) error {
	procs, err := s.detector.DiscoverJavaProcesses(ctx, &detector.ProcessFilter{PIDs: []int{pid}})
	if err != nil {
		return fmt.Errorf("failed to discover process: %w", err)
	}

	if len(procs) == 0 {
		return fmt.Errorf("process %d not found", pid)
	}

	javaProc := procs[0]

	// 检查是否已附加所有 agent
	for _, agent := range expectedAgents {
		if !s.detector.HasAgent(javaProc, agent.Path) {
			return fmt.Errorf("agent not found: %s", agent.Path)
		}
	}

	return nil
}
