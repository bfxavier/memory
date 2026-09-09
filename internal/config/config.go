package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Extraction Extraction `json:"extraction"`
}

type Extraction struct {
	Enabled         bool   `json:"enabled"`
	Provider        string `json:"provider"`
	BaseURL         string `json:"base_url"`
	Model           string `json:"model"`
	APIKeyEnv       string `json:"api_key_env,omitempty"`
	CodexModel      string `json:"codex_model"`
	ClaudeModel     string `json:"claude_model"`
	CodexCommand    string `json:"codex_command,omitempty"`
	ClaudeCommand   string `json:"claude_command,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	MaxInputBytes   int    `json:"max_input_bytes"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	MaxMemories     int    `json:"max_memories"`
	MaxJobsPerHour  int    `json:"max_jobs_per_hour"`
}

func Default() Config {
	return Config{
		Extraction: Extraction{
			Enabled:         true,
			Provider:        "host",
			BaseURL:         "http://127.0.0.1:11434/v1",
			CodexModel:      "gpt-5.6-luna",
			ClaudeModel:     "haiku",
			TimeoutSeconds:  90,
			MaxInputBytes:   48 * 1024,
			MaxOutputTokens: 2048,
			MaxMemories:     8,
			MaxJobsPerHour:  12,
		},
	}
}

func Load(path string) (Config, error) {
	result := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	applyDefaults(&result)
	if err := result.Validate(); err != nil {
		return Config{}, err
	}
	return result, nil
}

func Save(path string, value Config) error {
	applyDefaults(&value)
	if err := value.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func (c Config) Validate() error {
	if !c.Extraction.Enabled {
		return nil
	}
	switch c.Extraction.Provider {
	case "host":
		if strings.TrimSpace(c.Extraction.CodexModel) == "" || strings.TrimSpace(c.Extraction.ClaudeModel) == "" {
			return errors.New("host extraction requires codex_model and claude_model")
		}
	case "openai":
		if strings.TrimSpace(c.Extraction.BaseURL) == "" {
			return errors.New("extraction base_url is required")
		}
		if strings.TrimSpace(c.Extraction.Model) == "" {
			return errors.New("extraction model is required")
		}
	default:
		return fmt.Errorf("invalid extraction provider %q", c.Extraction.Provider)
	}
	if c.Extraction.TimeoutSeconds < 1 || c.Extraction.TimeoutSeconds > 600 {
		return errors.New("extraction timeout_seconds must be between 1 and 600")
	}
	if c.Extraction.MaxInputBytes < 1024 || c.Extraction.MaxInputBytes > 256*1024 {
		return errors.New("extraction max_input_bytes must be between 1024 and 262144")
	}
	if c.Extraction.MaxMemories < 1 || c.Extraction.MaxMemories > 32 {
		return errors.New("extraction max_memories must be between 1 and 32")
	}
	if c.Extraction.MaxOutputTokens < 128 || c.Extraction.MaxOutputTokens > 8192 {
		return errors.New("extraction max_output_tokens must be between 128 and 8192")
	}
	if c.Extraction.MaxJobsPerHour < 1 || c.Extraction.MaxJobsPerHour > 1000 {
		return errors.New("extraction max_jobs_per_hour must be between 1 and 1000")
	}
	return nil
}

func applyDefaults(config *Config) {
	defaults := Default().Extraction
	if config.Extraction.Provider == "" {
		if config.Extraction.Model != "" {
			config.Extraction.Provider = "openai"
		} else {
			config.Extraction.Provider = defaults.Provider
		}
	}
	if config.Extraction.BaseURL == "" {
		config.Extraction.BaseURL = defaults.BaseURL
	}
	if config.Extraction.TimeoutSeconds == 0 {
		config.Extraction.TimeoutSeconds = defaults.TimeoutSeconds
	}
	if config.Extraction.MaxInputBytes == 0 {
		config.Extraction.MaxInputBytes = defaults.MaxInputBytes
	}
	if config.Extraction.MaxMemories == 0 {
		config.Extraction.MaxMemories = defaults.MaxMemories
	}
	if config.Extraction.MaxOutputTokens == 0 {
		config.Extraction.MaxOutputTokens = defaults.MaxOutputTokens
	}
	if config.Extraction.MaxJobsPerHour == 0 {
		config.Extraction.MaxJobsPerHour = defaults.MaxJobsPerHour
	}
	if config.Extraction.CodexModel == "" {
		config.Extraction.CodexModel = defaults.CodexModel
	}
	if config.Extraction.ClaudeModel == "" {
		config.Extraction.ClaudeModel = defaults.ClaudeModel
	}
}
