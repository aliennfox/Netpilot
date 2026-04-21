package libcore

// PlatformInterface 是 Android/iOS 侧实现, Go 侧回调以获得平台特定能力。
//
// 这一份是 Pilotty 自己的简化版, 不是 sing-box libbox.PlatformInterface 的镜像 —
// gomobile bind 对接口类型支持有限, 必须用只包含 gomobile 可绑定类型 (string/int32/int64/bool/[]byte/error) 的接口。
//
// 3B-3 真正接入 libbox.NewCommandServer 时, 会写一个 libboxPlatformAdapter
// 把这个接口转译成 libbox.PlatformInterface (约 15 个方法, 但大部分可返回默认值)。
//
// 对照: ~/References/nekobox/libcore/platform_java.go 的 BoxPlatformInterface (7 方法)。
type PlatformInterface interface {
	// OpenTun 由 sing-box 核在启动 TUN inbound 时反向调用。
	// Android: VpnService.Builder.establish().detachFd() 取得并返回。
	// iOS: 通过 KVC tunnel.packetFlow.valueForKey("socket.fileDescriptor") 返回。
	// 入参 tunOptionsJSON 是 sing-box 序列化的 tun 配置, 平台侧可解析里面的 MTU/IP 等信息。
	OpenTun(tunOptionsJSON string) (int32, error)

	// AutoDetectInterfaceControl 对单个 outbound socket fd 调 VpnService.protect(fd),
	// 防止出站连接自己再走回 TUN 造成回环。3B-3 才接上。
	AutoDetectInterfaceControl(fd int32) error

	// UseProcFS 返回 Android 上是否可读 /proc/net/ (Android 5.0+ 限制)。
	// 通常返回 false, 走 NetworkStatsManager 替代路径。
	UseProcFS() bool

	// WIFIState 返回 "SSID\tBSSID" 或空串。iOS 15+ 需要定位权限才能拿到。
	WIFIState() string
}

// NopPlatformInterface 是所有方法都返回零值的占位实现, 用于 3B-1 桌面编译测试。
type NopPlatformInterface struct{}

func (NopPlatformInterface) OpenTun(string) (int32, error)          { return -1, nil }
func (NopPlatformInterface) AutoDetectInterfaceControl(int32) error { return nil }
func (NopPlatformInterface) UseProcFS() bool                        { return false }
func (NopPlatformInterface) WIFIState() string                      { return "" }
