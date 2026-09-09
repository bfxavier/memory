package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/bfxavier/memory/internal/config"
	"github.com/bfxavier/memory/internal/paths"
)

func runProvider(args []string, appPaths paths.Paths, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: memory provider <host|configure|disable|retry|status>")
	}
	settings, err := config.Load(appPaths.Config)
	if err != nil {
		return err
	}
	switch args[0] {
	case "host":
		settings.Extraction, err = hostExtraction(args[1:], settings.Extraction)
	case "configure":
		settings.Extraction, err = openAIExtraction(args[1:], settings.Extraction)
	case "disable":
		settings.Extraction.Enabled = false
	case "retry":
		return retryExtractionJobs(appPaths, output)
	case "status":
		return writeJSON(output, settings.Extraction)
	default:
		return fmt.Errorf("unknown provider command %q", args[0])
	}
	if err != nil {
		return err
	}
	if err := config.Save(appPaths.Config, settings); err != nil {
		return err
	}
	return writeJSON(output, settings.Extraction)
}

func hostExtraction(args []string, current config.Extraction) (config.Extraction, error) {
	flags := commandFlags("provider host")
	codexModel := flags.String("codex-model", current.CodexModel, "Codex subscription model")
	claudeModel := flags.String("claude-model", current.ClaudeModel, "Claude subscription model")
	codexCommand := flags.String("codex-command", current.CodexCommand, "Codex executable path")
	claudeCommand := flags.String("claude-command", current.ClaudeCommand, "Claude executable path")
	if err := flags.Parse(args); err != nil {
		return config.Extraction{}, err
	}
	current.Enabled = true
	current.Provider = "host"
	current.CodexModel = *codexModel
	current.ClaudeModel = *claudeModel
	current.CodexCommand = *codexCommand
	current.ClaudeCommand = *claudeCommand
	return current, nil
}

func openAIExtraction(args []string, current config.Extraction) (config.Extraction, error) {
	flags := commandFlags("provider configure")
	baseURL := flags.String("url", current.BaseURL, "OpenAI-compatible base URL")
	modelName := flags.String("model", current.Model, "model name")
	apiKeyEnv := flags.String("api-key-env", current.APIKeyEnv, "environment variable containing the API key")
	timeout := flags.Int("timeout", current.TimeoutSeconds, "request timeout in seconds")
	maxInputBytes := flags.Int("max-input-bytes", current.MaxInputBytes, "maximum event bytes per extraction")
	maxOutputTokens := flags.Int("max-output-tokens", current.MaxOutputTokens, "maximum generated tokens per extraction")
	maxMemories := flags.Int("max-memories", current.MaxMemories, "maximum memories per extraction")
	maxJobsPerHour := flags.Int("max-jobs-per-hour", current.MaxJobsPerHour, "maximum provider calls per hour")
	if err := flags.Parse(args); err != nil {
		return config.Extraction{}, err
	}
	current.Enabled = true
	current.Provider = "openai"
	current.BaseURL = *baseURL
	current.Model = *modelName
	current.APIKeyEnv = *apiKeyEnv
	current.TimeoutSeconds = *timeout
	current.MaxInputBytes = *maxInputBytes
	current.MaxOutputTokens = *maxOutputTokens
	current.MaxMemories = *maxMemories
	current.MaxJobsPerHour = *maxJobsPerHour
	return current, nil
}

func retryExtractionJobs(appPaths paths.Paths, output io.Writer) error {
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	count, err := database.RetryFailedExtractionJobs()
	if err != nil {
		return err
	}
	return writeJSON(output, map[string]any{"retried": count})
}
