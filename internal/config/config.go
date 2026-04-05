package config

const (
	DefaultClashAPIAddr = "http://127.0.0.1:9090"
	DefaultTestURL      = "https://www.gstatic.com/generate_204"
	DefaultTestTimeout  = 5000 // ms
	DefaultDataDir      = "data"

	// LLM
	DefaultLLMBaseURL = "https://api.siliconflow.cn/v1"
	DefaultLLMModel   = "deepseek-ai/DeepSeek-V3"
	DefaultLLMTimeout = 30 // seconds
	DefaultMaxIter    = 10
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
