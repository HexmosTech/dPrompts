package main

type DBConfig struct {
	Engine   string
	Name     string
	User     string
	Password string
	Host     string
	Port     string
}


type DPromptsJobArgs struct {
	Prompt       string      `json:"prompt"`
	Schema       interface{} `json:"schema,omitempty"`
	BasePrompt string      `json:"base_prompt,omitempty"`
}


type DPromptsJobResult struct {
	Response string `json:"response"`
}

func (DPromptsJobArgs) Kind() string {
	return "dprompts-worker"
}

type LLMConfig struct {
	APIEndpoint string  `toml:"api-endpoint"`
	Model       string  `toml:"model"`
	Temperature float64 `toml:"temperature"`
	TopP        float64 `toml:"topP"`
}

type OllamaResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
}


type WorkerConfig struct {
	ConcurrentWorkers int `toml:"concurrent_workers"`
}

