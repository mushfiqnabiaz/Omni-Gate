package models

import (
	"encoding/json"
	"time"
)

// Provider types
const (
	ProviderGoogle = "google"
	ProviderClaude = "claude"
	ProviderCodex  = "codex"
	ProviderOpenAI = "openai"
)

// Auth types
const (
	AuthOAuth    = "oauth"
	AuthAPIKey   = "api_key"
	AuthCodexCLI = "codex_cli"
	AuthSession  = "session"
)

// Account model mapped to PostgreSQL 'accounts' table
type Account struct {
	ID             string          `json:"id"`
	Provider       string          `json:"provider"`
	Name           string          `json:"name"`
	Email          string          `json:"email"`
	AuthType       string          `json:"auth_type"`
	Credentials    json.RawMessage `json:"credentials"`
	PlanType       string          `json:"plan_type"`
	Enabled        bool            `json:"enabled"`
	IsBanned       bool            `json:"is_banned"`
	CooldownUntil  int64           `json:"cooldown_until"`
	CooldownReason *string         `json:"cooldown_reason,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`

	// In-memory or joined quota details
	Quotas *AccountQuotas `json:"quotas,omitempty"`
}

type AccountQuotas struct {
	AccountID       string    `json:"account_id"`
	WeeklyPct       float64   `json:"weekly_pct"`
	Burst5hPct      float64   `json:"burst_5h_pct"`
	ClaudeWeeklyPct float64   `json:"claude_weekly_pct"`
	Claude5hPct     float64   `json:"claude_5h_pct"`
	WeeklyResetAt   int64     `json:"weekly_reset_at"`
	Burst5hResetAt  int64     `json:"burst_5h_reset_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Credentials helpers
type GoogleCredentials struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	IdToken         string `json:"id_token,omitempty"`
	ExpiryTimestamp int64  `json:"expiry_timestamp"`
	ProjectID       string `json:"project_id"`
}

type ClaudeCredentials struct {
	AccessToken      string `json:"access_token,omitempty"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	ExpiresAt        int64  `json:"expires_at,omitempty"`
	APIKey           string `json:"api_key,omitempty"`
	OrganizationUUID string `json:"organization_uuid,omitempty"`
}

type CodexCredentials struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
}

// Virtual API Key for client access
type VirtualAPIKey struct {
	ID            string    `json:"id"`
	KeyHash       string    `json:"-"`
	Name          string    `json:"name"`
	Prefix        string    `json:"prefix"`
	RateLimitRPM  int       `json:"rate_limit_rpm"`
	RateLimitTPM  int       `json:"rate_limit_tpm"`
	ExpiresAt     int64     `json:"expires_at"`
	Enabled       bool      `json:"enabled"`
	TotalRequests int64     `json:"total_requests"`
	TotalTokens   int64     `json:"total_tokens"`
	LastUsedAt    int64     `json:"last_used_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// OpenAI Chat Completion API structs
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Provider    string        `json:"provider,omitempty"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

type ChatChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type ChatChunkChoice struct {
	Index        int            `json:"index"`
	Delta        ChatChunkDelta `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

type ChatCompletionChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []ChatChunkChoice `json:"choices"`
}

// Anthropic /v1/messages structs
type AnthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicMessageRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	System    string        `json:"system,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Stream    bool          `json:"stream,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicMessageResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        AnthropicUsage          `json:"usage"`
}
