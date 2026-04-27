package config

const (
	DefaultClashAPIAddr = "http://127.0.0.1:9090"
	DefaultTestURL      = "https://www.gstatic.com/generate_204"
	DefaultTestTimeout  = 5000 // ms
	DefaultDataDir      = "data"

	// LLM
	DefaultLLMBaseURL = "https://api.siliconflow.cn/v1"
	// 2026-04-27 切 Qwen3.6-35B-A3B (用户指定): V4 Flash / Qwen3-32B 在 tool-use 多轮上
	// 都被 thinking mode 吞 tool_call 卡住. Qwen3.6 是 2026/04 阿里新一代 MoE,35B 总参 / 3B
	// 激活, 原生支持 thinking 与 non-thinking 切换 + OpenAI tool-use 格式. SiliconFlow
	// dashboard 已上架 (文档站尚未同步). 仍走 enable_thinking=false 强制 non-thinking 路径.
	DefaultLLMModel = "Qwen/Qwen3.6-35B-A3B"
	// DefaultLLMTimeout Qwen3-32B 关 thinking 后首 token 一般 < 5s, 但留够 tool 长链路 buffer.
	DefaultLLMTimeout = 180 // seconds
	DefaultMaxIter    = 10
	// DefaultLLMTemperature 降低 DeepSeek-V3 的 tool-pick 随机性。默认 1.0 会导致同一 prompt
	// 选不同 tool / 有时 fabricate。0.2 是 "稳定但不至于僵化" 的工程常用值。
	DefaultLLMTemperature = 0.2
)

type Config struct {
	ClashAPIAddr string
	TestURL      string
	TestTimeout  int // ms
	DataDir      string
}

func Default() *Config {
	return &Config{
		ClashAPIAddr: DefaultClashAPIAddr,
		TestURL:      DefaultTestURL,
		TestTimeout:  DefaultTestTimeout,
		DataDir:      DefaultDataDir,
	}
}
