package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antigravity/gateway/pkg/models"
)

var defaultCodexBinaryPaths = []string{
	"/opt/homebrew/lib/node_modules/@openai/codex/node_modules/@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex",
	"/usr/local/lib/node_modules/@openai/codex/node_modules/@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex",
	"codex",
}

type CodexClient struct {
	binaryPath string
}

func NewCodexClient() *CodexClient {
	bin := "codex"
	for _, p := range defaultCodexBinaryPaths {
		if _, err := os.Stat(p); err == nil {
			bin = p
			break
		}
	}
	return &CodexClient{binaryPath: bin}
}

// NormalizeCodexModel maps OpenAI requested models to Codex CLI compatible models
func NormalizeCodexModel(rawModel string) string {
	m := strings.ToLower(strings.TrimSpace(rawModel))
	validModels := map[string]bool{
		"gpt-5.5":           true,
		"gpt-5.6-luna":      true,
		"gpt-5.6-sol":       true,
		"gpt-5.6-terra":     true,
		"gpt-reserve":       true,
		"codex-auto-review": true,
	}

	if validModels[m] {
		return m
	}
	return "gpt-5.5"
}

// BuildCodexPrompt builds single consolidated prompt from chat messages
func BuildCodexPrompt(messages []models.ChatMessage) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := strings.ToUpper(msg.Role)
		sb.WriteString(fmt.Sprintf("[%s]\n%s\n\n", role, msg.Content))
	}
	return sb.String()
}

// StreamCodex runs native Mach-O binary and parses real-time JSON stream
func (c *CodexClient) StreamCodex(ctx context.Context, model string, prompt string, onChunk func(text string) error) (*models.ChatUsage, error) {
	codexModel := NormalizeCodexModel(model)

	// Arguments: -C /tmp exec --skip-git-repo-check --ephemeral --color never -m <model> --json <prompt>
	args := []string{
		"-C", "/tmp",
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"--color", "never",
		"-m", codexModel,
		"--json",
		prompt,
	}

	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	cmd.Dir = "/tmp"
	cmd.Stdin = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stdout pipe error: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stderr pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex binary error: %w", err)
	}

	var usage models.ChatUsage
	var lastText string
	scanner := bufio.NewScanner(stdout)

	// Stream stdout line-by-line
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var ev struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(line), &ev); err == nil {
			if ev.Type == "item.completed" && ev.Item.Type == "agent_message" && ev.Item.Text != "" {
				delta := ev.Item.Text
				if strings.HasPrefix(delta, lastText) {
					delta = delta[len(lastText):]
				}
				lastText = ev.Item.Text
				if len(delta) > 0 {
					if err := onChunk(delta); err != nil {
						_ = cmd.Process.Kill()
						return nil, err
					}
				}
			}
			if ev.Type == "turn.completed" {
				usage.PromptTokens = ev.Usage.InputTokens
				usage.CompletionTokens = ev.Usage.OutputTokens
				usage.TotalTokens = ev.Usage.InputTokens + ev.Usage.OutputTokens
			}
		}
	}

	// Read any error from stderr
	var stderrBuf strings.Builder
	errScanner := bufio.NewScanner(stderr)
	for errScanner.Scan() {
		stderrBuf.WriteString(errScanner.Text())
		stderrBuf.WriteString("\n")
	}

	waitErr := cmd.Wait()
	if waitErr != nil && lastText == "" {
		errStr := stderrBuf.String()
		if strings.Contains(strings.ToLower(errStr), "rate_limit") ||
			strings.Contains(strings.ToLower(errStr), "quota") ||
			strings.Contains(strings.ToLower(errStr), "usage-limit") {
			return nil, fmt.Errorf("codex quota limit hit: %s", errStr)
		}
		return nil, fmt.Errorf("codex exited with error (%v): %s", waitErr, errStr)
	}

	if usage.TotalTokens == 0 {
		usage.PromptTokens = len(prompt) / 4
		usage.CompletionTokens = len(lastText) / 4
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return &usage, nil
}

// GenerateUnary executes unary completion
func (c *CodexClient) GenerateUnary(ctx context.Context, model string, messages []models.ChatMessage) (*models.ChatCompletionResponse, error) {
	prompt := BuildCodexPrompt(messages)
	var sb strings.Builder

	usage, err := c.StreamCodex(ctx, model, prompt, func(text string) error {
		sb.WriteString(text)
		return nil
	})
	if err != nil {
		return nil, err
	}

	finalText := sb.String()
	res := &models.ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   NormalizeCodexModel(model),
		Choices: []models.ChatChoice{
			{
				Index: 0,
				Message: models.ChatMessage{
					Role:    "assistant",
					Content: finalText,
				},
				FinishReason: "stop",
			},
		},
		Usage: *usage,
	}
	return res, nil
}
