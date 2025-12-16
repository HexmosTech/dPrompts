package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/BurntSushi/toml"
)



func LoadLLMConfig(configPath string) (*LLMConfig, error) {
	var conf struct {
		LLM LLMConfig
	}
	_, err := toml.DecodeFile(configPath, &conf)
	if err != nil {
		return nil, err
	}
	return &conf.LLM, nil
}

func CallOllama(
	prompt string,
	schema interface{},
	configPath string,
	basePrompt string,
) (string, error) {

	// Load config
	llmConfig, err := LoadLLMConfig(configPath)
	if err != nil {
		return "", err
	}

	// Build request
	req := map[string]any{
		"model":  llmConfig.Model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": basePrompt},
			{"role": "user", "content": prompt},
		},
		"options": map[string]float64{
			"temperature": llmConfig.Temperature,
			"top_p":       llmConfig.TopP,
		},
	}

	if schema != nil {
		req["format"] = schema
	} else {
		req["format"] = "json"
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	// HTTP call
	client := &http.Client{Timeout: 360 * time.Second}
	resp, err := client.Post(
		llmConfig.APIEndpoint,
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama API returned %s", resp.Status)
	}

	// Read & decode response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", err
	}

	return ollamaResp.Message.Content, nil
}



