package config

const (
	DefaultClashAPIAddr = "http://127.0.0.1:9090"
	DefaultTestURL      = "https://www.gstatic.com/generate_204"
	DefaultTestTimeout  = 5000 // ms
	DefaultDataDir      = "data"

	// LLM
	DefaultLLMBaseURL = "https://api.siliconflow.cn/v1"
	// 2026-04-27 切 Qwen3-32B: V4 Flash 在测速→选最快节点的 2nd-turn tool_call 上反复卡死
	// (reasoning model 把后续 tool_call 当 text 输出, stream 不结束). Qwen3-32B 默认开 thinking,
	// 需在 request body 加 enable_thinking=false (见 llm_client.go) 才能稳定走 tool-use 路径.
	DefaultLLMModel = "Qwen/Qwen3-32B"
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
