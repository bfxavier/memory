package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bfxavier/memory/internal/config"
	"github.com/bfxavier/memory/internal/model"
)

type Client struct {
	config config.Extraction
	http   *http.Client
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	MaxTokens      int           `json:"max_tokens"`
	ResponseFormat any           `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func New(settings config.Extraction) *Client {
	return &Client{
		config: settings,
		http:   &http.Client{Timeout: time.Duration(settings.TimeoutSeconds) * time.Second},
	}
}

func (c *Client) Extract(ctx context.Context, events []model.Event, active []model.Memory) ([]model.ExtractedMemory, error) {
	input, eventIDs, activeIDs := renderInput(events, active, c.config.MaxInputBytes)
	if input == "" {
		return nil, nil
	}
	payload, err := c.requestPayload(input)
	if err != nil {
		return nil, err
	}
	endpoint, err := chatEndpoint(c.config.BaseURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if err := c.authorize(request); err != nil {
		return nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	content, err := decodeChatResponse(response)
	if err != nil {
		return nil, err
	}
	return parseExtraction(content, eventIDs, activeIDs, c.config.MaxMemories)
}

func (c *Client) requestPayload(input string) ([]byte, error) {
	return json.Marshal(chatRequest{
		Model:          c.config.Model,
		Messages:       []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: input}},
		Temperature:    0,
		MaxTokens:      c.config.MaxOutputTokens,
		ResponseFormat: map[string]string{"type": "json_object"},
	})
}

func (c *Client) authorize(request *http.Request) error {
	if c.config.APIKeyEnv == "" {
		return nil
	}
	key := os.Getenv(c.config.APIKeyEnv)
	if key == "" {
		return fmt.Errorf("environment variable %s is empty", c.config.APIKeyEnv)
	}
	request.Header.Set("Authorization", "Bearer "+key)
	return nil
}

func decodeChatResponse(response *http.Response) (string, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("provider returned %s: %s", response.Status, clip(string(body), 512))
	}
	var chat chatResponse
	if err := json.Unmarshal(body, &chat); err != nil {
		return "", fmt.Errorf("decode provider response: %w", err)
	}
	if len(chat.Choices) == 0 {
		return "", errors.New("provider returned no choices")
	}
	return chat.Choices[0].Message.Content, nil
}

func chatEndpoint(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid extraction base URL")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/chat/completions"
	return parsed.String(), nil
}
