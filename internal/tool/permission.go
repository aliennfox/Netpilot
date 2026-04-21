package tool

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// PermissionDecision 用户对某次工具调用的批准结果
type PermissionDecision string

const (
	PermAllowOnce    PermissionDecision = "allow_once"
	PermAllowSession PermissionDecision = "allow_session" // 本次进程内同名工具自动放行
	PermDeny         PermissionDecision = "deny"
)

// TrustMode 全局信任级别
type TrustMode string

const (
	TrustAsk    TrustMode = "ask"    // 默认：每次写操作都问
	TrustAuto   TrustMode = "auto"   // 全部自动放行（开发/CI 模式）
	TrustStrict TrustMode = "strict" // 一律拒绝写操作
)

// PermissionApprover 由调用方实现，用于向用户请求授权
type PermissionApprover interface {
	Ask(toolName string, params map[string]interface{}) PermissionDecision
}

// PermissionManager 管理 trust mode + session 白名单 + 调用 approver
type PermissionManager struct {
	mu       sync.Mutex
	mode     TrustMode
	approver PermissionApprover
	allowed  map[string]bool // session 内已批准 "allow_session" 的工具名
}

func NewPermissionManager(mode TrustMode, approver PermissionApprover) *PermissionManager {
	return &PermissionManager{
		mode:     mode,
		approver: approver,
		allowed:  map[string]bool{},
	}
}

func (p *PermissionManager) SetMode(m TrustMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = m
}

func (p *PermissionManager) Mode() TrustMode {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mode
}

// Check 决定一次写操作是否被允许执行
func (p *PermissionManager) Check(toolName string, params map[string]interface{}) PermissionDecision {
	p.mu.Lock()
	mode := p.mode
	if p.allowed[toolName] {
		p.mu.Unlock()
		return PermAllowOnce
	}
	p.mu.Unlock()

	switch mode {
	case TrustAuto:
		return PermAllowOnce
	case TrustStrict:
		return PermDeny
	}

	if p.approver == nil {
		// 没有 approver 又是 ask 模式 → 默认拒绝（安全失败）
		return PermDeny
	}

	decision := p.approver.Ask(toolName, params)
	if decision == PermAllowSession {
		p.mu.Lock()
		p.allowed[toolName] = true
		p.mu.Unlock()
	}
	return decision
}

// ResetSession 清空 session 白名单
func (p *PermissionManager) ResetSession() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.allowed = map[string]bool{}
}

// --- CLI 实现：从 stdin 读取 y/n/a ---

type CLIApprover struct {
	reader *bufio.Reader
}

func NewCLIApprover() *CLIApprover {
	return &CLIApprover{reader: bufio.NewReader(os.Stdin)}
}

func (c *CLIApprover) Ask(toolName string, params map[string]interface{}) PermissionDecision {
	fmt.Printf("\033[33m[Permission] Agent 请求执行写操作: %s %s\033[0m\n",
		toolName, formatParams(params))
	fmt.Print("\033[33m  允许? [y]es / [n]o / [a]lways (本次会话内不再询问): \033[0m")

	line, err := c.reader.ReadString('\n')
	if err != nil {
		return PermDeny
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return PermAllowOnce
	case "a", "always":
		return PermAllowSession
	default:
		return PermDeny
	}
}

// --- Auto approver：服务端/非交互场景 ---

type AutoApprover struct{}

func (AutoApprover) Ask(string, map[string]interface{}) PermissionDecision {
	return PermAllowOnce
}
