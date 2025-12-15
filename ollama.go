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

func CallOllama(prompt string, schema interface{}, configPath string, groupName string, base_prompt string) (string, error) {
	startTotal := time.Now()

	// 1️⃣ Load config
	start := time.Now()
	llmConfig, err := LoadLLMConfig(configPath)
	if err != nil {
		return "", err
	}
	log.Info().
		Dur("duration", time.Since(start)).
		Msg("LoadLLMConfig duration")

	// 2️⃣ Build request body
	start = time.Now()
	req := map[string]interface{}{
		"model":  llmConfig.Model,
		"stream": false,
		"session_id": groupName,
		"messages": []map[string]string{
			{"role": "system", "content": base_prompt },
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
	log.Info().
		Dur("duration", time.Since(start)).
		Msg("Marshal request duration")

	// 3️⃣ HTTP POST
	start = time.Now()
	client := &http.Client{Timeout: 360 * time.Second}
	resp, err := client.Post(llmConfig.APIEndpoint, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	log.Info().
		Dur("duration", time.Since(start)).
		Msg("HTTP POST duration")

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("ollama API returned non-200 status: " + resp.Status)
	}

	// 4️⃣ Read response
	start = time.Now()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	log.Info().
		Dur("duration", time.Since(start)).
		Msg("Read response duration")

	// 5️⃣ Unmarshal response
	start = time.Now()
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", err
	}
	log.Info().
		Dur("duration", time.Since(start)).
		Msg("Unmarshal response duration")

	log.Info().
		Dur("total_duration", time.Since(startTotal)).
		Msg("Ollama call completed")

	return ollamaResp.Message.Content, nil
}


