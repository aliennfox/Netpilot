package engine

import "time"

// EngineAdapter is the abstraction layer for the network engine.
// Current implementation: SingBoxAdapter (Clash API HTTP).
// Future: RustCoreAdapter or LibboxGRPCAdapter.
type EngineAdapter interface {
	// Lifecycle — not implemented in Phase 1
	Start(configPath string) error
	Stop() error
	Reload() error
	IsRunning() bool

	// Config management — not implemented in Phase 1
	GetCurrentConfig() ([]byte, error)
	PatchConfig(patch []byte) error
	ReplaceConfig(config []byte) error
	ValidateConfig(config []byte) error

	// Proxy operations
	GetProxies() ([]ProxyInfo, error)
	GetProxyGroup(groupTag string) (*ProxyGroup, error)
	SetActiveProxy(groupTag, proxyTag string) error
	TestLatency(proxyTag string, url string, timeout time.Duration) (int, error)
	TestLatencyBatch(tags []string, url string, timeout time.Duration) ([]LatencyResult, error)

	// Connections & traffic
	GetConnections() ([]ConnectionInfo, error)
	CloseConnection(id string) error
	GetTrafficStats() (*TrafficStats, error)

	// Logs
	GetLogs(level string, lines int) ([]LogEntry, error)
	SubscribeLogs(level string) (<-chan LogEntry, func())

	// DNS
	QueryDNS(domain string) (*DNSResult, error)

	// Network events
	OnNetworkChanged()
}

type ProxyInfo struct {
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Alive    bool   `json:"alive"`
	Latency  int    `json:"latency"`
	GroupTag string `json:"group_tag"`
}

type ProxyGroup struct {
	Tag     string      `json:"tag"`
	Type    string      `json:"type"`
	Now     string      `json:"now"`
	All     []ProxyInfo `json:"all"`
}

type LatencyResult struct {
	Tag     string `json:"tag"`
	Latency int    `json:"latency"`
	Error   string `json:"error,omitempty"`
}

type ConnectionInfo struct {
	ID          string `json:"id"`
	Destination string `json:"destination"`
	Protocol    string `json:"protocol"`
	ProcessName string `json:"process_name"`
	Upload      int64  `json:"upload"`
	Download    int64  `json:"download"`
	StartTime   string `json:"start_time"`
	Chain       string `json:"chain"`
	Rule        string `json:"rule"`
}

type TrafficStats struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

type LogEntry struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type DNSResult struct {
	Answer []string `json:"answer"`
}
