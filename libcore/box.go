// Package libcore 封装 sing-box libbox,作为 NetPilot 的数据面入口。
//
// 抄作业对象: ~/References/nekobox/libcore/ (NekoBox 的同名 package)。
// 职责:
//  1. 向 Go 侧暴露 BoxInstance (start/stop/close sing-box 内核)
//  2. 向 Android/iOS 暴露 PlatformInterface 接口, 让平台端回调提供 TUN fd 等
//
// 当前 (3B-1) 实现为最小 stub, 真实 service 启停逻辑留给 3B-3:
// sing-box v1.13.8 libbox 公共入口改为 NewCommandServer + PlatformInterface,
// 不再有 NewService 直接构造函数, 需要用 CommandServer RPC 模式对接。
package libcore

import (
	"sync"

	"github.com/sagernet/sing-box/experimental/libbox"
)

// BoxInstance 是 sing-box 内核的 Go 句柄。线程安全。
type BoxInstance struct {
	mu      sync.Mutex
	config  string
	iface   PlatformInterface
	running bool
	// TODO(3B-3): *libbox.CommandServer 或类似对象, 用来真控制 sing-box 启停
}

// NewBoxInstance 根据 sing-box 配置 JSON 创建一个实例。
// iface 允许为 nil —— 测试或桌面场景用, Android/iOS 必须传。
func NewBoxInstance(configJSON string, iface PlatformInterface) (*BoxInstance, error) {
	return &BoxInstance{
		config: configJSON,
		iface:  iface,
	}, nil
}

// Start 启动 sing-box 数据面。
// 3B-1 阶段: 仅标记状态, 不真启动。3B-3 接入 libbox.NewCommandServer 真跑流量。
func (b *BoxInstance) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running = true
	// TODO(3B-3): 用 libbox.NewCommandServer(handler, platformInterfaceAdapter(b.iface))
	// 构造 CommandServer, 通过它发送 StartCommand 加载 b.config。
	return nil
}

// Close 关闭 sing-box 数据面。
func (b *BoxInstance) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running = false
	// TODO(3B-3): CommandServer.Close()
	return nil
}

// IsRunning 返回当前是否在跑。
func (b *BoxInstance) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// Version 返回嵌入的 sing-box libbox 版本字符串。
// 该函数做两件事: 暴露给 Kotlin 作为自检点, 并迫使 libbox package 进入符号表。
func Version() string {
	return libbox.Version()
}

// Setup 初始化 libbox 全局状态 (基路径、日志上限等)。
// Android VpnService onCreate 或 iOS provider init 时调用一次。
func Setup(basePath, workingPath, tempPath string) error {
	return libbox.Setup(&libbox.SetupOptions{
		BasePath:    basePath,
		WorkingPath: workingPath,
		TempPath:    tempPath,
	})
}
