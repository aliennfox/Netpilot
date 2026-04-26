package config

const (
	DefaultClashAPIAddr = "http://127.0.0.1:9090"
	DefaultTestURL      = "https://www.gstatic.com/generate_204"
	DefaultTestTimeout  = 5000 // ms
	DefaultDataDir      = "data"

	// LLM
	DefaultLLMBaseURL = "https://api.siliconflow.cn/v1"
	DefaultLLMModel   = "deepseek-ai/DeepSeek-V4-Flash"
	// DefaultLLMTimeout V4 Flash 是 MoE + reasoning, 首 token 30-90s 常态。 60s 会撞 awaiting-headers 超时。
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
