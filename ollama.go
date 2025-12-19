package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/AlecAivazis/survey/v2"
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

func hasSystemdOllama() bool {
	cmd := exec.Command("systemctl", "list-unit-files", "ollama.service")
	return cmd.Run() == nil
}

func hasSnapOllama() bool {
	cmd := exec.Command("snap", "list", "ollama")
	return cmd.Run() == nil
}

func hasOllamaBinary() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

func askLinuxStartMethod() (string, error) {
	options := []string{}

	if hasSystemdOllama() {
		options = append(options, "Start via systemd (recommended)")
	}
	if hasSnapOllama() {
		options = append(options, "Start via snap")
	}
	if hasOllamaBinary() {
		options = append(options, "Start manually (ollama serve)")
	}

	options = append(options, "Do not start Ollama")

	var choice string
	prompt := &survey.Select{
		Message: "Ollama is not running. How do you want to start it?",
		Options: options,
	}

	err := survey.AskOne(prompt, &choice)
	return choice, err
}

func startOllamaLinux() error {
	choice, err := askLinuxStartMethod()
	if err != nil {
		return err
	}

	switch choice {

	case "Start via systemd (recommended)":
		fmt.Println("[INFO] Starting Ollama via systemd...")
		return exec.Command("sudo", "systemctl", "start", "ollama").Run()

	case "Start via snap":
		fmt.Println("[INFO] Starting Ollama via snap...")
		return exec.Command("sudo", "snap", "start", "ollama").Run()

	case "Start manually (ollama serve)":
		fmt.Println("[INFO] Starting Ollama manually...")
		cmd := exec.Command("ollama", "serve")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Start()

	default:
		fmt.Println("[INFO] Ollama start skipped by user.")
		return nil
	}
}

func startOllamaWindows() error {
	fmt.Println("[INFO] Running on Windows. Looking for Ollama in PATH...")
	path, err := exec.LookPath("ollama app")
	if err != nil {
		fmt.Println("[ERROR] Ollama not found in PATH. Make sure it is installed.")
		return err
	}

	fmt.Printf("[INFO] Found Ollama executable at: %s\n", path)
	cmd := exec.Command(path) // Tray/background app
	return cmd.Start()
}

func startOllama() error {
	switch runtime.GOOS {
	case "windows":
		return startOllamaWindows()

	case "linux":
		return startOllamaLinux()

	case "darwin": // macOS
		fmt.Println("[INFO] Running on macOS. Starting Ollama manually...")
		cmd := exec.Command("ollama", "serve")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Start()

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
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
