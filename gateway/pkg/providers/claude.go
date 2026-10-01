package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antigravity/gateway/pkg/models"
)

const (
	AnthropicMessagesURL = "https://api.anthropic.com/v1/messages"
	AnthropicVersion     = "2023-06-01"
)

type ClaudeClient struct {
	httpClient *http.Client
}

func NewClaudeClient() *ClaudeClient {
	tr := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 30,
		IdleConnTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &ClaudeClient{
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   120 * time.Second,
		},
	}
}

// NormalizeClaudeModel ensures valid Anthropic model name
func NormalizeClaudeModel(rawModel string) string {
	m := strings.ToLower(strings.TrimSpace(rawModel))
	switch {
	case strings.Contains(m, "3-7-sonnet") || strings.Contains(m, "3.7-sonnet"):
		return "claude-3-7-sonnet-20250219"
	case strings.Contains(m, "3-5-sonnet") || strings.Contains(m, "3.5-sonnet"):
		return "claude-3-5-sonnet-20241022"
	case strings.Contains(m, "3-5-haiku") || strings.Contains(m, "3.5-haiku"):
		return "claude-3-5-haiku-20241022"
	case strings.Contains(m, "3-opus"):
		return "claude-3-opus-20240229"
	case strings.Contains(m, "sonnet"):
		return "claude-3-7-sonnet-20250219"
	default:
		return "claude-3-7-sonnet-20250219"
	}
}

// ConvertOpenAIToAnthropic converts OpenAI chat request to Anthropic message format
func ConvertOpenAIToAnthropic(req *models.ChatCompletionRequest) (string, map[string]any) {
	model := NormalizeClaudeModel(req.Model)
	var system string
	var messages []map[string]any

	for _, msg := range req.Messages {
		role := strings.ToLower(msg.Role)
		if role == "system" {
			if system != "" {
				system += "\n\n"
			}
			system += msg.Content
			continue
		}

		claudeRole := role
		if claudeRole != "user" && claudeRole != "assistant" {
			claudeRole = "user"
		}

		messages = append(messages, map[string]any{
			"role":    claudeRole,
			"content": msg.Content,
		})
	}

	maxTokens := 4096
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	body := map[string]any{
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
	}
	if system != "" {
		body["system"] = system
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.Stream {
		body["stream"] = true
	}

	return model, body
}

// StreamAnthropic calls Anthropic API and emits chunks
func (c *ClaudeClient) StreamAnthropic(ctx context.Context, creds *models.ClaudeCredentials, payload map[string]any, onChunk func(text string) error) error {
	payload["stream"] = true
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", AnthropicMessagesURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}

	httpReq.Header.Set("anthropic-version", AnthropicVersion)
	httpReq.Header.Set("content-type", "application/json")

	if creds.APIKey != "" {
		httpReq.Header.Set("x-api-key", creds.APIKey)
	} else if creds.AccessToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("anthropic error (%d): %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}

		if err := json.Unmarshal([]byte(data), &event); err == nil {
			if event.Type == "content_block_delta" && event.Delta.Text != "" {
				if err := onChunk(event.Delta.Text); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// GenerateUnary executes unary Anthropic completion
func (c *ClaudeClient) GenerateUnary(ctx context.Context, creds *models.ClaudeCredentials, payload map[string]any, modelName string) (*models.ChatCompletionResponse, error) {
	var sb strings.Builder
	err := c.StreamAnthropic(ctx, creds, payload, func(text string) error {
		sb.WriteString(text)
		return nil
	})
	if err != nil {
		return nil, err
	}

	fullText := sb.String()
	res := &models.ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []models.ChatChoice{
			{
				Index: 0,
				Message: models.ChatMessage{
					Role:    "assistant",
					Content: fullText,
				},
				FinishReason: "stop",
			},
		},
		Usage: models.ChatUsage{
			PromptTokens:     len(fullText) / 6,
			CompletionTokens: len(fullText) / 4,
			TotalTokens:      (len(fullText) / 6) + (len(fullText) / 4),
		},
	}
	return res, nil
}
