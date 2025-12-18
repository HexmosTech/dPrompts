package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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



func isOllamaRunning() bool {
	client := http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}


func startOllama() error {
	var cmd *exec.Cmd

	// Works on Linux / macOS / Windows (if ollama is in PATH)
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ollama", "serve")
	} else {
		cmd = exec.Command("ollama", "serve")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start() // non-blocking
}


func waitForOllama(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if isOllamaRunning() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("ollama did not start within %s", timeout)
}