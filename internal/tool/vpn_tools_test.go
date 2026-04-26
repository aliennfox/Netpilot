package tool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// fakeVpnController 让测试可控 IsRunning 返回值 + 模拟"延迟启动"。
//
// 字段说明:
//   - running: 当前状态 (atomic.Bool 让 polling goroutine 安全读)
//   - readyAfter: RequestStart 调用后多久 ms 翻转 running=true; 0 = 立即; <0 = 永不 (模拟用户拒绝)
//   - startErr: RequestStart 直接返回的 error
//   - startCalls/stopCalls: 调用计数, 让测试断言 idempotency
type fakeVpnController struct {
	running    atomic.Bool
	readyAfter time.Duration
	stopAfter  time.Duration
	startErr   error
	startCalls atomic.Int32
	stopCalls  atomic.Int32
}

func (f *fakeVpnController) RequestStart() error {
	f.startCalls.Add(1)
	if f.startErr != nil {
		return f.startErr
	}
	if f.readyAfter < 0 {
		return nil // 永不就绪 (模拟用户拒绝授权)
	}
	go func() {
		time.Sleep(f.readyAfter)
		f.running.Store(true)
	}()
	return nil
}

func (f *fakeVpnController) RequestStop() error {
	f.stopCalls.Add(1)
	if f.stopAfter < 0 {
		return nil
	}
	go func() {
		time.Sleep(f.stopAfter)
		f.running.Store(false)
	}()
	return nil
}

func (f *fakeVpnController) IsRunning() bool { return f.running.Load() }

// TestStartVpn_BlocksUntilReady 验证 toolStartVpn 同步语义 — 返回 Success=true
// 之前 IsRunning 必须真的翻成 true (而不是早返回的 "启动请求已发送")
func TestStartVpn_BlocksUntilReady(t *testing.T) {
	ctrl := &fakeVpnController{readyAfter: 800 * time.Millisecond}
	tool := toolStartVpn(ctrl)

	t0 := time.Now()
	res, err := tool.Execute(context.Background(), nil, nil)
	elapsed := time.Since(t0)

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected Success=true, got msg=%q", res.Message)
	}
	if !ctrl.IsRunning() {
		t.Fatal("ctrl.IsRunning still false after start_vpn returned Success")
	}
	// 必须等到 readyAfter (~800ms) + warm-up (~1500ms) 才返回; 总不会 <2s
	if elapsed < 2*time.Second {
		t.Errorf("returned too fast (%v); expected >2s for poll + warmup", elapsed)
	}
	if ctrl.startCalls.Load() != 1 {
		t.Errorf("expected exactly 1 RequestStart call, got %d", ctrl.startCalls.Load())
	}
}

// TestStartVpn_AlreadyRunningSkipsRequest 已 running 时不要重复 RequestStart
func TestStartVpn_AlreadyRunningSkipsRequest(t *testing.T) {
	ctrl := &fakeVpnController{}
	ctrl.running.Store(true)
	tool := toolStartVpn(ctrl)

	res, _ := tool.Execute(context.Background(), nil, nil)
	if !res.Success {
		t.Fatalf("expected Success=true on already-running, got %q", res.Message)
	}
	if ctrl.startCalls.Load() != 0 {
		t.Errorf("expected 0 RequestStart calls (idempotent), got %d", ctrl.startCalls.Load())
	}
}

// TestStartVpn_TimeoutReturnsFailure readyAfter < 0 模拟用户拒绝授权场景 —
// 必须超时 (15s) 返回 Success=false, 不能 hang 无限
func TestStartVpn_TimeoutReturnsFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 15s timeout test in short mode")
	}
	ctrl := &fakeVpnController{readyAfter: -1}
	tool := toolStartVpn(ctrl)

	t0 := time.Now()
	res, _ := tool.Execute(context.Background(), nil, nil)
	elapsed := time.Since(t0)

	if res.Success {
		t.Fatal("expected Success=false on never-ready VPN")
	}
	// 应在 15-16s 区间, 不能 hang 更久
	if elapsed < 14*time.Second || elapsed > 17*time.Second {
		t.Errorf("timeout window wrong: elapsed=%v, want ~15s", elapsed)
	}
}

// TestStopVpn_BlocksUntilDown stop_vpn 同步语义对称 — Success 后 IsRunning 必须 false
func TestStopVpn_BlocksUntilDown(t *testing.T) {
	ctrl := &fakeVpnController{stopAfter: 400 * time.Millisecond}
	ctrl.running.Store(true)
	tool := toolStopVpn(ctrl)

	t0 := time.Now()
	res, _ := tool.Execute(context.Background(), nil, nil)
	elapsed := time.Since(t0)

	if !res.Success {
		t.Fatalf("expected Success=true, got %q", res.Message)
	}
	if ctrl.IsRunning() {
		t.Fatal("IsRunning still true after stop_vpn returned Success")
	}
	if elapsed < 400*time.Millisecond {
		t.Errorf("returned too fast (%v); should wait stopAfter", elapsed)
	}
}

// TestStopVpn_AlreadyStoppedIdempotent 已停时不重复请求
func TestStopVpn_AlreadyStoppedIdempotent(t *testing.T) {
	ctrl := &fakeVpnController{}
	tool := toolStopVpn(ctrl)

	res, _ := tool.Execute(context.Background(), nil, nil)
	if !res.Success {
		t.Fatal("expected Success=true on already-stopped")
	}
	if ctrl.stopCalls.Load() != 0 {
		t.Errorf("expected 0 RequestStop calls, got %d", ctrl.stopCalls.Load())
	}
}
