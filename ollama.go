package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog/log"
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

func CallOllama(prompt string, configPath string) (string, error) {
	llmConfig, err := LoadLLMConfig(configPath)
	if err != nil {
		return "", err
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"model":    llmConfig.Model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"stream":   false,
		"format":   "json",
		"options": map[string]float64{
			"temperature": llmConfig.Temperature,
			"top_p":       llmConfig.TopP,
			"num_predict": 100,
		},
	})
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 360 * time.Second}
	endpoint := llmConfig.APIEndpoint
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("ollama API returned non-200 status: " + resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", err
	}
	log.Info().Str("response", ollamaResp.Message.Content).Msg("Ollama response received")

	return ollamaResp.Message.Content, nil
}
