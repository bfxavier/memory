package extractor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bfxavier/memory/internal/config"
	"github.com/bfxavier/memory/internal/model"
)

const extractionSchema = `{"type":"object","properties":{"memories":{"type":"array","maxItems":32,"items":{"type":"object","properties":{"kind":{"type":"string","enum":["decision","fact","preference","procedure","failure","outcome","note"]},"content":{"type":"string"},"confidence":{"type":"number","minimum":0.5,"maximum":1},"tags":{"type":"array","items":{"type":"string"},"maxItems":16},"source_event_id":{"type":"string"},"supersedes_id":{"type":"string"}},"required":["kind","content","confidence","tags","source_event_id","supersedes_id"],"additionalProperties":false}}},"required":["memories"],"additionalProperties":false}`

type HostClient struct {
	config config.Extraction
}

func NewHost(settings config.Extraction) *HostClient {
	return &HostClient{config: settings}
}

func (c *HostClient) Extract(ctx context.Context, agent string, events []model.Event, active []model.Memory) ([]model.ExtractedMemory, error) {
	input, eventIDs, activeIDs := renderInput(events, active, c.config.MaxInputBytes)
	if input == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.TimeoutSeconds)*time.Second)
	defer cancel()
	var output string
	var err error
	switch agent {
	case "codex":
		output, err = c.runCodex(ctx, input)
	case "claude":
		output, err = c.runClaude(ctx, input)
	default:
		return nil, fmt.Errorf("host extraction is unsupported for agent %q", agent)
	}
	if err != nil {
		return nil, err
	}
	return parseExtraction(output, eventIDs, activeIDs, c.config.MaxMemories)
}

func (c *HostClient) runCodex(ctx context.Context, input string) (string, error) {
	commandPath, err := resolveCommand(c.config.CodexCommand, "codex")
	if err != nil {
		return "", err
	}
	schemaPath, removeSchema, err := createTemporaryFile("memory-extraction-schema-*.json", extractionSchema)
	if err != nil {
		return "", err
	}
	defer removeSchema()
	outputPath, removeOutput, err := createTemporaryFile("memory-extraction-output-*.json", "")
	if err != nil {
		return "", err
	}
	defer removeOutput()
	command := exec.CommandContext(ctx, commandPath,
		"exec", "--ephemeral", "--ignore-user-config", "--ignore-rules",
		"--skip-git-repo-check", "--sandbox", "read-only", "--model", c.config.CodexModel,
		"-c", `approval_policy="never"`, "-c", `model_reasoning_effort="low"`,
		"--color", "never", "--output-schema", schemaPath, "--output-last-message", outputPath, "-",
	)
	command.Stdin = strings.NewReader(systemPrompt + "\n\n" + input)
	stdout, stderr := newLimitedBuffer(1024*1024), newLimitedBuffer(64*1024)
	command.Stdout, command.Stderr, command.Env = stdout, stderr, extractorEnvironment()
	if err := command.Run(); err != nil {
		return "", hostCommandError(ctx, "codex", err, stderr.String())
	}
	file, err := os.Open(outputPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 1024*1024))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(data)) == "" {
		return stdout.String(), nil
	}
	return string(data), nil
}

func (c *HostClient) runClaude(ctx context.Context, input string) (string, error) {
	commandPath, err := resolveCommand(c.config.ClaudeCommand, "claude")
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, commandPath,
		"--safe-mode", "--print", "--model", c.config.ClaudeModel,
		"--tools", "", "--permission-mode", "dontAsk", "--permission-prompts", "none",
		"--no-session-persistence", "--output-format", "text", "--json-schema", extractionSchema,
		"--system-prompt", systemPrompt,
	)
	command.Stdin = strings.NewReader(input)
	stdout, stderr := newLimitedBuffer(1024*1024), newLimitedBuffer(64*1024)
	command.Stdout, command.Stderr, command.Env = stdout, stderr, extractorEnvironment()
	if err := command.Run(); err != nil {
		return "", hostCommandError(ctx, "claude", err, stderr.String())
	}
	return stdout.String(), nil
}

func hostCommandError(ctx context.Context, name string, err error, stderr string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s extraction timed out: %w", name, context.DeadlineExceeded)
	}
	return commandError(name, err, stderr)
}
