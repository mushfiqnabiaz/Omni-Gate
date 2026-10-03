package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antigravity/gateway/pkg/models"
)

var (
	GoogleTokenURL     = "https://oauth2.googleapis.com/token"
	CloudCodeQuotaURL  = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"
	CloudCodeUserAgent = "antigravity/4.3.0 darwin/arm64"
)

var CloudCodeStreamEndpoints = []string{
	"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:streamGenerateContent?alt=sse",
	"https://cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse",
	"https://daily-cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse",
}

type GoogleClient struct {
	httpClient *http.Client
}

func NewGoogleClient() *GoogleClient {
	tr := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 30,
		IdleConnTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &GoogleClient{
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   120 * time.Second,
		},
	}
}

// RefreshTokenIfNeeded checks if Google token is expired and refreshes it using OAuth client
func (c *GoogleClient) RefreshTokenIfNeeded(ctx context.Context, creds *models.GoogleCredentials) (bool, error) {
	nowSec := time.Now().Unix()
	expSec := creds.ExpiryTimestamp
	if expSec > 1e11 {
		expSec = expSec / 1000
	}

	// Token is valid if expires more than 120 seconds in the future
	if expSec > (nowSec+120) && creds.AccessToken != "" {
		return false, nil
	}

	if creds.RefreshToken == "" {
		return false, fmt.Errorf("missing refresh token")
	}

	form := url.Values{}
	form.Set("client_id", os.Getenv("GOOGLE_CLIENT_ID"))
	form.Set("client_secret", os.Getenv("GOOGLE_CLIENT_SECRET"))
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", creds.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", GoogleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("token refresh network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var res struct {
		AccessToken string `json:"access_token"`
		IdToken     string `json:"id_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return false, err
	}

	creds.AccessToken = res.AccessToken
	if res.IdToken != "" {
		creds.IdToken = res.IdToken
	} else {
		log.Printf("[Gateway] Google token refresh returned no id_token")
	}
	creds.ExpiryTimestamp = time.Now().Unix() + res.ExpiresIn
	return true, nil
}

var CloudCodeQuotaEndpoints = []string{
	"https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
	"https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
	"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:retrieveUserQuotaSummary",
}

// FetchQuotaSummary calls retrieveUserQuotaSummary across fallback endpoints
func (c *GoogleClient) FetchQuotaSummary(ctx context.Context, creds *models.GoogleCredentials) (weeklyPct, burst5hPct, claudeWeeklyPct, claude5hPct float64, err error) {
	_, err = c.RefreshTokenIfNeeded(ctx, creds)
	if err != nil {
		return 100, 100, 100, 100, err
	}

	weeklyPct = 100
	burst5hPct = 100
	claudeWeeklyPct = 100
	claude5hPct = 100

	reqBody := []byte(`{}`)
	var lastErr error

	for _, ep := range CloudCodeQuotaEndpoints {
		req, reqErr := http.NewRequestWithContext(ctx, "POST", ep, bytes.NewReader(reqBody))
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", CloudCodeUserAgent)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("endpoint %s returned status %d", ep, resp.StatusCode)
			continue
		}

		var data struct {
			Groups []struct {
				DisplayName string `json:"displayName"`
				Buckets     []struct {
					Window            string   `json:"window"`
					RemainingFraction *float64 `json:"remainingFraction"`
					ResetTime         string   `json:"resetTime"`
				} `json:"buckets"`
			} `json:"groups"`
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()
		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}

		for _, g := range data.Groups {
			name := strings.ToLower(g.DisplayName)
			isGemini := strings.Contains(name, "gemini")
			isClaude := strings.Contains(name, "claude") || strings.Contains(name, "gpt") || strings.Contains(name, "3p")

			for _, b := range g.Buckets {
				pct := 100.0
				if b.RemainingFraction != nil {
					pct = math.Round(*b.RemainingFraction*1000) / 10
				}

				if isGemini {
					if b.Window == "weekly" {
						weeklyPct = pct
					} else if b.Window == "5h" {
						burst5hPct = pct
					}
				} else if isClaude {
					if b.Window == "weekly" {
						claudeWeeklyPct = pct
					} else if b.Window == "5h" {
						claude5hPct = pct
					}
				}
			}
		}

		return weeklyPct, burst5hPct, claudeWeeklyPct, claude5hPct, nil
	}

	return 100, 100, 100, 100, lastErr
}

// MapToGoogleInternalModel maps external requested model name to Google internal model
func MapToGoogleInternalModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(m, "3.8-flash") || strings.Contains(m, "3.8"):
		return "gemini-3.8-flash-medium"
	case strings.Contains(m, "3.7-flash"):
		return "gemini-3.7-flash-medium"
	case strings.Contains(m, "3-flash"):
		return "gemini-3-flash"
	case strings.Contains(m, "2.5-flash"):
		return "gemini-2.5-flash"
	case strings.Contains(m, "2.5-pro"):
		return "gemini-2.5-pro"
	case strings.Contains(m, "sonnet") || strings.Contains(m, "claude"):
		return "claude-sonnet-4-6"
	case strings.Contains(m, "opus"):
		return "claude-opus-4-6-thinking"
	default:
		return "gemini-2.5-flash"
	}
}

// ConvertOpenAIToCloudCode formats OpenAI messages for Cloud Code internal API
func ConvertOpenAIToCloudCode(req *models.ChatCompletionRequest, projectId string) (string, map[string]any) {
	internalModel := MapToGoogleInternalModel(req.Model)

	var contents []map[string]any
	var systemParts []map[string]string

	for _, msg := range req.Messages {
		role := strings.ToLower(msg.Role)
		if role == "system" {
			systemParts = append(systemParts, map[string]string{"text": msg.Content})
			continue
		}
		var geminiRole string
		switch role {
		case "assistant":
			geminiRole = "model"
		case "user":
			geminiRole = "user"
		default:
			geminiRole = "user"
		}

		contents = append(contents, map[string]any{
			"role": geminiRole,
			"parts": []map[string]string{
				{"text": msg.Content},
			},
		})
	}

	generationConfig := map[string]any{}
	if req.Temperature != nil {
		generationConfig["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		generationConfig["maxOutputTokens"] = *req.MaxTokens
	}
	if strings.Contains(internalModel, "3.8") || strings.Contains(internalModel, "3.7") {
		generationConfig["thinkingConfig"] = map[string]any{"thinkingBudget": 512}
	}

	requestObj := map[string]any{
		"contents": contents,
	}
	if len(systemParts) > 0 {
		requestObj["systemInstruction"] = map[string]any{
			"parts": systemParts,
		}
	}
	if len(generationConfig) > 0 {
		requestObj["generationConfig"] = generationConfig
	}

	if projectId == "" {
		projectId = "aicode-consumers"
	}

	body := map[string]any{
		"model":   internalModel,
		"project": projectId,
		"request": requestObj,
	}

	return internalModel, body
}

// StreamGenerate executes streaming completions against Cloud Code internal API
func (c *GoogleClient) StreamGenerate(ctx context.Context, creds *models.GoogleCredentials, payload map[string]any, onChunk func(text string) error) error {
	_, err := c.RefreshTokenIfNeeded(ctx, creds)
	if err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var lastErr error
	for _, ep := range CloudCodeStreamEndpoints {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", ep, bytes.NewReader(jsonBytes))
		if err != nil {
			lastErr = err
			continue
		}
		httpReq.Header.Set("Authorization", "Bearer "+creds.AccessToken)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("User-Agent", CloudCodeUserAgent)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 502 || resp.StatusCode == 503 {
			resp.Body.Close()
			lastErr = fmt.Errorf("upstream status %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("upstream cloud code error (%d): %s", resp.StatusCode, string(body))
		}

		// Read SSE stream
		defer resp.Body.Close()
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

			var chunk struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text    string `json:"text"`
							Thought bool   `json:"thought"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
				Response struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text    string `json:"text"`
								Thought bool   `json:"thought"`
							} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
				} `json:"response"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				var textParts []string
				cands := chunk.Candidates
				if len(cands) == 0 {
					cands = chunk.Response.Candidates
				}
				for _, cand := range cands {
					for _, part := range cand.Content.Parts {
						if !part.Thought && part.Text != "" {
							textParts = append(textParts, part.Text)
						}
					}
				}
				if len(textParts) > 0 {
					fullText := strings.Join(textParts, "")
					if err := onChunk(fullText); err != nil {
						return err
					}
				}
			}
		}

		return nil
	}

	return fmt.Errorf("all cloud code endpoints failed: %v", lastErr)
}

// GenerateUnary executes non-streaming completion
func (c *GoogleClient) GenerateUnary(ctx context.Context, creds *models.GoogleCredentials, payload map[string]any, modelName string) (*models.ChatCompletionResponse, error) {
	var accumulatedText strings.Builder
	err := c.StreamGenerate(ctx, creds, payload, func(text string) error {
		accumulatedText.WriteString(text)
		return nil
	})
	if err != nil {
		return nil, err
	}

	finalText := accumulatedText.String()
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
					Content: finalText,
				},
				FinishReason: "stop",
			},
		},
		Usage: models.ChatUsage{
			PromptTokens:     10,
			CompletionTokens: len(finalText) / 4,
			TotalTokens:      10 + (len(finalText) / 4),
		},
	}
	return res, nil
}
