package router

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/antigravity/gateway/pkg/antigravity"
	"github.com/antigravity/gateway/pkg/db"
	"github.com/antigravity/gateway/pkg/models"
	"github.com/antigravity/gateway/pkg/providers"
	"github.com/antigravity/gateway/pkg/shield"
)

type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
	}
}

func (rl *RateLimiter) Allow(keyID string, rpm int) (bool, time.Duration) {
	if rpm <= 0 {
		return true, 0
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	reqs := rl.requests[keyID]
	var valid []time.Time
	for _, t := range reqs {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rpm {
		oldest := valid[0]
		retryAfter := oldest.Add(1 * time.Minute).Sub(now)
		rl.requests[keyID] = valid
		return false, retryAfter
	}

	valid = append(valid, now)
	rl.requests[keyID] = valid
	return true, 0
}

func estimatePromptTokens(messages []models.ChatMessage) int {
	var totalChars int
	for _, m := range messages {
		totalChars += len(m.Content) + len(m.Role) + 4
	}
	tokens := int(math.Ceil(float64(totalChars) / 4.0))
	if tokens < 1 {
		return 1
	}
	return tokens
}

func estimateCompletionTokens(totalChars int) int {
	tokens := int(math.Ceil(float64(totalChars) / 4.0))
	if tokens < 1 {
		return 1
	}
	return tokens
}

func analyzePromptComplexity(messages []models.ChatMessage) int {
	var fullText string
	for _, m := range messages {
		fullText += m.Content + "\n"
	}
	score := 0
	if len(fullText) > 1000 {
		score += 30
	}
	if strings.Contains(fullText, "```") {
		score += 30
	}
	if strings.Contains(fullText, "{") && strings.Contains(fullText, "}") {
		score += 15
	}
	lower := strings.ToLower(fullText)
	complexKeywords := []string{"refactor", "architect", "debug", "analyze", "explain", "generate", "build", "design"}
	for _, kw := range complexKeywords {
		if strings.Contains(lower, kw) {
			score += 10
		}
	}
	if score > 100 {
		score = 100
	}
	return score
}

type Router struct {
	db          *db.DB
	rotator     *shield.Rotator
	supervisor  *antigravity.Supervisor
	googleCl    *providers.GoogleClient
	claudeCl    *providers.ClaudeClient
	codexCl     *providers.CodexClient
	rateLimiter *RateLimiter
	mux         *chi.Mux

	cacheMutex  sync.RWMutex
	promptCache map[string]*CachedResponse
}

type CachedResponse struct {
	Response     *models.ChatCompletionResponse
	StreamChunks []string
	ExpiresAt    time.Time
}

func generateCacheKey(req *models.ChatCompletionRequest) string {
	h := sha256.New()
	h.Write([]byte(req.Model))
	for _, m := range req.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func NewRouter(database *db.DB, rot *shield.Rotator, sup *antigravity.Supervisor) *Router {
	r := &Router{
		db:          database,
		rotator:     rot,
		supervisor:  sup,
		googleCl:    providers.NewGoogleClient(),
		claudeCl:    providers.NewClaudeClient(),
		codexCl:     providers.NewCodexClient(),
		rateLimiter: NewRateLimiter(),
		mux:         chi.NewRouter(),
		promptCache: make(map[string]*CachedResponse),
	}

	r.setupRoutes()
	return r
}

func (r *Router) authenticateRequest(req *http.Request) (*models.VirtualAPIKey, error) {
	var rawKey string

	authHeader := req.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		rawKey = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	} else if strings.HasPrefix(authHeader, "bearer ") {
		rawKey = strings.TrimSpace(strings.TrimPrefix(authHeader, "bearer "))
	}

	if rawKey == "" {
		rawKey = strings.TrimSpace(req.Header.Get("x-api-key"))
	}

	if rawKey == "" {
		return nil, fmt.Errorf("missing API key: provide 'Authorization: Bearer ag_live_...' or 'x-api-key: ag_live_...'")
	}

	if !strings.HasPrefix(rawKey, "ag_live_") {
		return nil, fmt.Errorf("invalid API key prefix: must start with 'ag_live_'")
	}

	k, err := r.db.ValidateVirtualKey(req.Context(), rawKey)
	if err != nil {
		return nil, fmt.Errorf("invalid or revoked API key: %w", err)
	}

	if !k.Enabled {
		return nil, fmt.Errorf("API key is disabled")
	}

	if k.ExpiresAt > 0 && k.ExpiresAt < time.Now().UnixMilli() {
		return nil, fmt.Errorf("API key has expired")
	}

	if allowed, retryAfter := r.rateLimiter.Allow(k.ID, k.RateLimitRPM); !allowed {
		return nil, fmt.Errorf("rate limit exceeded (%d RPM limit reached): retry in %v", k.RateLimitRPM, retryAfter.Round(time.Second))
	}

	return k, nil
}

func (r *Router) setupRoutes() {
	r.mux.Use(middleware.RequestID)
	r.mux.Use(middleware.RealIP)
	r.mux.Use(middleware.Recoverer)
	r.mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "x-api-key", "anthropic-version", "x-provider"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.mux.Get("/health", r.handleHealth)

	// OpenAI Compatible API
	r.mux.Route("/v1", func(cr chi.Router) {
		cr.Get("/models", r.handleModels)
		cr.Post("/chat/completions", r.handleChatCompletions)
		cr.Post("/messages", r.handleAnthropicMessages)
	})

	// Anthropic Native Path Alias
	r.mux.Post("/v1/messages", r.handleAnthropicMessages)

	// Admin / Management API
	r.mux.Route("/api", func(ar chi.Router) {
		ar.Get("/fleet", r.handleFleet)
		ar.Post("/set-active-account", r.handleSetActiveAccount)
		ar.Get("/ide/status", r.handleIDEStatus)
		ar.Get("/keys", r.handleListKeys)
		ar.Post("/keys", r.handleCreateKey)
		ar.Delete("/keys/{id}", r.handleDeleteKey)
		ar.Get("/logs", r.handleLogs)
		ar.Get("/settings", r.handleGetSettings)
		ar.Post("/settings", r.handleUpdateSettings)
		ar.Get("/analytics", r.handleAnalytics)
		ar.Get("/analytics/export", r.handleAnalyticsExport)
		ar.Post("/shield/evaluate", r.handleShieldEvaluate)
		ar.Post("/account/toggle", r.handleToggleAccount)
	})
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "healthy",
		"service":   "antigravity-gateway",
		"engine":    "go1.26.2 darwin/arm64",
		"database":  "postgresql (OrbStack dev-postgres)",
		"timestamp": time.Now().Unix(),
	})
}

func (r *Router) handleModels(w http.ResponseWriter, req *http.Request) {
	modelsList := []map[string]any{
		{"id": "gemini-3.8-flash", "object": "model", "owned_by": "google"},
		{"id": "gemini-3.8-flash-medium", "object": "model", "owned_by": "google"},
		{"id": "gemini-3.7-flash", "object": "model", "owned_by": "google"},
		{"id": "gemini-2.5-pro", "object": "model", "owned_by": "google"},
		{"id": "gemini-2.5-flash", "object": "model", "owned_by": "google"},
		{"id": "claude-3-7-sonnet", "object": "model", "owned_by": "anthropic"},
		{"id": "claude-3-5-sonnet", "object": "model", "owned_by": "anthropic"},
		{"id": "claude-3-5-haiku", "object": "model", "owned_by": "anthropic"},
		{"id": "claude-sonnet-4-6", "object": "model", "owned_by": "google-claude-bridge"},
		{"id": "gpt-5.5", "object": "model", "owned_by": "openai-codex"},
		{"id": "gpt-5.6-luna", "object": "model", "owned_by": "openai-codex"},
		{"id": "gpt-5.6-sol", "object": "model", "owned_by": "openai-codex"},
		{"id": "gpt-4o", "object": "model", "owned_by": "openai-codex"},
		{"id": "gpt-4o-mini", "object": "model", "owned_by": "openai-codex"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   modelsList,
	})
}

func (r *Router) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now()

	// Authenticate and enforce rate limits
	vKey, authErr := r.authenticateRequest(req)
	if authErr != nil {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(authErr.Error(), "rate limit exceeded") {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": authErr.Error(),
					"type":    "rate_limit_error",
					"code":    "rate_limit_exceeded",
				},
			})
		} else if strings.Contains(authErr.Error(), "disabled") || strings.Contains(authErr.Error(), "expired") {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": authErr.Error(),
					"type":    "invalid_request_error",
					"code":    "key_forbidden",
				},
			})
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": authErr.Error(),
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				},
			})
		}
		return
	}
	vKeyID := vKey.ID

	var chatReq models.ChatCompletionRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	promptTokens := estimatePromptTokens(chatReq.Messages)

	// Determine provider
	provider := chatReq.Provider
	if provider == "" {
		m := strings.ToLower(chatReq.Model)
		switch {
		case strings.HasPrefix(m, "claude"):
			provider = models.ProviderClaude
		case strings.HasPrefix(m, "gpt") || strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3"):
			provider = models.ProviderCodex
		default:
			provider = models.ProviderGoogle
		}
	}

	// 0. Semantic Cost-Arbitrage Routing (Auto-Downgrade/Upgrade)
	if req.Header.Get("X-OmniGate-Auto-Route") == "true" || chatReq.Model == "auto" {
		complexity := analyzePromptComplexity(chatReq.Messages)
		oldModel := chatReq.Model
		if complexity < 30 {
			chatReq.Model = "gemini-1.5-flash"
			provider = models.ProviderGoogle
		} else if complexity < 70 {
			chatReq.Model = "claude-3-5-haiku" // or gemini-1.5-pro
			provider = models.ProviderClaude
		} else {
			chatReq.Model = "claude-3-5-sonnet-20240620"
			provider = models.ProviderClaude
		}
		log.Printf("[Gateway] 🧠 Cost-Arbitrage Engine: Complexity %d/100. Routed %s -> %s", complexity, oldModel, chatReq.Model)
	}

	// 0.5. Multi-Model Race Engine
	raceHeader := req.Header.Get("X-OmniGate-Race")
	if raceHeader != "" && chatReq.Stream {
		r.handleRaceEngine(req.Context(), w, req, vKeyID, chatReq, raceHeader, promptTokens, startTime)
		return
	}

	// 1. Semantic Prompt Cache Check (Sub-100ms)
	cacheKey := generateCacheKey(&chatReq)
	r.cacheMutex.RLock()
	cached, ok := r.promptCache[cacheKey]
	r.cacheMutex.RUnlock()

	if ok && time.Now().Before(cached.ExpiresAt) {
		log.Printf("[Gateway] ⚡ CACHE HIT for model %s. Served in sub-100ms.", chatReq.Model)
		durMs := int(time.Since(startTime).Milliseconds())
		
		if chatReq.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, ok := w.(http.Flusher)
			if ok {
				for _, chunk := range cached.StreamChunks {
					fmt.Fprintf(w, "data: %s\n\n", chunk)
					flusher.Flush()
					// Provide minimal artificial stream delay to appease client parsers
					time.Sleep(2 * time.Millisecond)
				}
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cached.Response)
		}
		
		_ = r.db.RecordRequestLog(req.Context(), uuid.New().String(), vKeyID, "CACHE", "cache", chatReq.Model, promptTokens, 0, durMs, http.StatusOK, "Semantic Cache Hit")
		return
	}

	// Try selected provider with automatic failover
	var finalErr error
	for attempt := 0; attempt < 3; attempt++ {
		account, err := r.rotator.SelectAccount(req.Context(), provider)
		if err != nil {
			// Failover to Google bridge if Claude or Codex exhausted
			if provider != models.ProviderGoogle {
				log.Printf("[Gateway] 🔄 Fleet for %s exhausted. Failing over to Google Cloud Code bridge...\n", provider)
				provider = models.ProviderGoogle
				continue
			}
			finalErr = err
			break
		}

		if chatReq.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}

			var chunks []string
			var ttft int
			completionTokens, err := r.executeStream(req.Context(), w, flusher, provider, account, &chatReq, func(ms int) {
				ttft = ms
			}, func(chunkJSON string) {
				chunks = append(chunks, chunkJSON)
			})
			if err == nil {
				durMs := int(time.Since(startTime).Milliseconds())
				logLatency := durMs
				if ttft > 0 {
					logLatency = ttft
				}

				r.cacheMutex.Lock()
				r.promptCache[cacheKey] = &CachedResponse{
					StreamChunks: chunks,
					ExpiresAt:    time.Now().Add(1 * time.Hour),
				}
				r.cacheMutex.Unlock()

				_ = r.db.RecordRequestLog(req.Context(), uuid.New().String(), vKeyID, account.ID, provider, chatReq.Model, promptTokens, completionTokens, logLatency, http.StatusOK, "")
				_ = r.db.RecordKeyUsage(req.Context(), vKeyID, promptTokens+completionTokens)
				return
			}
		} else {
			resp, err := r.executeUnary(req.Context(), provider, account, &chatReq)
			if err == nil {
				durMs := int(time.Since(startTime).Milliseconds())
				if resp.Usage.PromptTokens == 0 {
					resp.Usage.PromptTokens = promptTokens
				}
				if resp.Usage.CompletionTokens == 0 && len(resp.Choices) > 0 {
					resp.Usage.CompletionTokens = estimateCompletionTokens(len(resp.Choices[0].Message.Content))
				}
				resp.Usage.TotalTokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens

				r.cacheMutex.Lock()
				r.promptCache[cacheKey] = &CachedResponse{
					Response:  resp,
					ExpiresAt: time.Now().Add(1 * time.Hour),
				}
				r.cacheMutex.Unlock()

				_ = r.db.RecordRequestLog(req.Context(), resp.ID, vKeyID, account.ID, provider, resp.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, durMs, http.StatusOK, "")
				_ = r.db.RecordKeyUsage(req.Context(), vKeyID, resp.Usage.TotalTokens)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
		}

		// Hit error on Claude: auto failover to Google Cloud Code Claude bridge
		if provider == models.ProviderClaude {
			log.Printf("[Gateway] 🔄 Claude provider error (%v). Failing over to Google Cloud Code bridge...\n", err)
			_ = r.rotator.HandleQuotaExceeded(req.Context(), account, err.Error(), 86400)
			provider = models.ProviderGoogle
			continue
		}

		// Hit error on Codex: auto failover to Google Cloud Code bridge
		if provider == models.ProviderCodex {
			log.Printf("[Gateway] 🔄 Codex provider error (%v). Failing over to Google Cloud Code bridge...\n", err)
			_ = r.rotator.HandleQuotaExceeded(req.Context(), account, err.Error(), 300)
			provider = models.ProviderGoogle
			continue
		}

		// Hit rate limit / quota error on Google: cooldown and failover to next Google account candidate
		if err != nil && (strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")) {
			_ = r.rotator.HandleQuotaExceeded(req.Context(), account, err.Error(), 300)
		}
		finalErr = err
	}

	durMs := int(time.Since(startTime).Milliseconds())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	errMsg := "Unknown error"
	if finalErr != nil {
		errMsg = finalErr.Error()
	}
	_ = r.db.RecordRequestLog(req.Context(), uuid.New().String(), vKeyID, "", provider, chatReq.Model, promptTokens, 0, durMs, http.StatusServiceUnavailable, errMsg)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": errMsg})
}

func (r *Router) executeStream(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, provider string, account *models.Account, req *models.ChatCompletionRequest, onFirstChunk func(ttftMs int), onChunkData func(chunkJSON string)) (int, error) {
	id := "chatcmpl-" + uuid.New().String()
	created := time.Now().Unix()
	streamStart := time.Now()
	var firstChunkRecorded bool
	var totalChars int

	onChunk := func(text string) error {
		totalChars += len(text)
		if !firstChunkRecorded {
			firstChunkRecorded = true
			if onFirstChunk != nil {
				onFirstChunk(int(time.Since(streamStart).Milliseconds()))
			}
		}
		chunk := models.ChatCompletionChunk{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   req.Model,
			Choices: []models.ChatChunkChoice{
				{
					Index: 0,
					Delta: models.ChatChunkDelta{
						Content: text,
					},
				},
			},
		}
		data, _ := json.Marshal(chunk)
		if onChunkData != nil {
			onChunkData(string(data))
		}
		_, err := fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
		return err
	}

	var err error
	if provider == models.ProviderGoogle {
		var gCreds models.GoogleCredentials
		_ = json.Unmarshal(account.Credentials, &gCreds)
		_, payload := providers.ConvertOpenAIToCloudCode(req, gCreds.ProjectID)
		err = r.googleCl.StreamGenerate(ctx, &gCreds, payload, onChunk)
		if err == nil {
			_ = r.db.UpdateAccountCredentials(ctx, account.ID, gCreds)
		}
	} else if provider == models.ProviderClaude {
		var cCreds models.ClaudeCredentials
		_ = json.Unmarshal(account.Credentials, &cCreds)
		_, payload := providers.ConvertOpenAIToAnthropic(req)
		err = r.claudeCl.StreamAnthropic(ctx, &cCreds, payload, onChunk)
	} else if provider == models.ProviderCodex {
		prompt := providers.BuildCodexPrompt(req.Messages)
		_, err = r.codexCl.StreamCodex(ctx, req.Model, prompt, onChunk)
	}

	if err == nil {
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
	return estimateCompletionTokens(totalChars), err
}

func (r *Router) executeUnary(ctx context.Context, provider string, account *models.Account, req *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	if provider == models.ProviderGoogle {
		var gCreds models.GoogleCredentials
		_ = json.Unmarshal(account.Credentials, &gCreds)
		internalModel, payload := providers.ConvertOpenAIToCloudCode(req, gCreds.ProjectID)
		resp, err := r.googleCl.GenerateUnary(ctx, &gCreds, payload, internalModel)
		if err == nil {
			_ = r.db.UpdateAccountCredentials(ctx, account.ID, gCreds)
		}
		return resp, err
	} else if provider == models.ProviderClaude {
		var cCreds models.ClaudeCredentials
		_ = json.Unmarshal(account.Credentials, &cCreds)
		model, payload := providers.ConvertOpenAIToAnthropic(req)
		return r.claudeCl.GenerateUnary(ctx, &cCreds, payload, model)
	} else if provider == models.ProviderCodex {
		return r.codexCl.GenerateUnary(ctx, req.Model, req.Messages)
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func (r *Router) handleAnthropicMessages(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now()

	// Authenticate and enforce rate limits
	vKey, authErr := r.authenticateRequest(req)
	if authErr != nil {
		w.Header().Set("Content-Type", "application/json")
		status := http.StatusUnauthorized
		errType := "authentication_error"
		if strings.Contains(authErr.Error(), "rate limit") {
			status = http.StatusTooManyRequests
			errType = "rate_limit_error"
		} else if strings.Contains(authErr.Error(), "disabled") || strings.Contains(authErr.Error(), "expired") {
			status = http.StatusForbidden
			errType = "permission_error"
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    errType,
				"message": authErr.Error(),
			},
		})
		return
	}

	var aReq models.AnthropicMessageRequest
	if err := json.NewDecoder(req.Body).Decode(&aReq); err != nil {
		http.Error(w, `{"type": "error", "error": {"type": "invalid_request_error", "message": "Invalid request body"}}`, http.StatusBadRequest)
		return
	}

	messages := aReq.Messages
	if aReq.System != "" {
		messages = append([]models.ChatMessage{{Role: "system", Content: aReq.System}}, messages...)
	}

	cReq := models.ChatCompletionRequest{
		Model:     aReq.Model,
		Messages:  messages,
		Stream:    aReq.Stream,
		MaxTokens: &aReq.MaxTokens,
		Provider:  models.ProviderClaude,
	}

	promptTokens := estimatePromptTokens(messages)

	account, err := r.rotator.SelectAccount(req.Context(), models.ProviderClaude)
	if err != nil {
		// Fallback to Google Cloud Code Claude bridge
		account, err = r.rotator.SelectAccount(req.Context(), models.ProviderGoogle)
		if err != nil {
			http.Error(w, `{"type": "error", "error": {"type": "api_error", "message": "All provider accounts unavailable"}}`, http.StatusServiceUnavailable)
			return
		}
		cReq.Provider = models.ProviderGoogle
	}

	if aReq.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		msgID := "msg_" + uuid.New().String()

		// 1. event: message_start
		startEv, _ := json.Marshal(map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":            msgID,
				"type":          "message",
				"role":          "assistant",
				"content":       []any{},
				"model":         aReq.Model,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]any{
					"input_tokens":  promptTokens,
					"output_tokens": 0,
				},
			},
		})
		_, _ = fmt.Fprintf(w, "event: message_start\ndata: %s\n\n", string(startEv))

		// 2. event: content_block_start
		blockStartEv, _ := json.Marshal(map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]string{
				"type": "text",
				"text": "",
			},
		})
		_, _ = fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", string(blockStartEv))
		flusher.Flush()

		var totalChars int
		onChunk := func(text string) error {
			totalChars += len(text)
			deltaEv, _ := json.Marshal(map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]string{
					"type": "text_delta",
					"text": text,
				},
			})
			_, err := fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", string(deltaEv))
			flusher.Flush()
			return err
		}

		var streamErr error
		if cReq.Provider == models.ProviderGoogle {
			var gCreds models.GoogleCredentials
			_ = json.Unmarshal(account.Credentials, &gCreds)
			_, payload := providers.ConvertOpenAIToCloudCode(&cReq, gCreds.ProjectID)
			streamErr = r.googleCl.StreamGenerate(req.Context(), &gCreds, payload, onChunk)
			if streamErr == nil {
				_ = r.db.UpdateAccountCredentials(req.Context(), account.ID, gCreds)
			}
		} else if cReq.Provider == models.ProviderClaude {
			var cCreds models.ClaudeCredentials
			_ = json.Unmarshal(account.Credentials, &cCreds)
			_, payload := providers.ConvertOpenAIToAnthropic(&cReq)
			streamErr = r.claudeCl.StreamAnthropic(req.Context(), &cCreds, payload, onChunk)
		}

		if streamErr == nil {
			completionTokens := estimateCompletionTokens(totalChars)

			// 3. event: content_block_stop
			blockStopEv, _ := json.Marshal(map[string]any{
				"type":  "content_block_stop",
				"index": 0,
			})
			_, _ = fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", string(blockStopEv))

			// 4. event: message_delta
			msgDeltaEv, _ := json.Marshal(map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason":   "end_turn",
					"stop_sequence": nil,
				},
				"usage": map[string]any{
					"output_tokens": completionTokens,
				},
			})
			_, _ = fmt.Fprintf(w, "event: message_delta\ndata: %s\n\n", string(msgDeltaEv))

			// 5. event: message_stop
			msgStopEv, _ := json.Marshal(map[string]string{"type": "message_stop"})
			_, _ = fmt.Fprintf(w, "event: message_stop\ndata: %s\n\n", string(msgStopEv))
			flusher.Flush()

			durMs := int(time.Since(startTime).Milliseconds())
			_ = r.db.RecordRequestLog(req.Context(), msgID, vKey.ID, account.ID, cReq.Provider, aReq.Model, promptTokens, completionTokens, durMs, http.StatusOK, "")
			_ = r.db.RecordKeyUsage(req.Context(), vKey.ID, promptTokens+completionTokens)
		}
		return
	}

	// Unary
	resp, err := r.executeUnary(req.Context(), cReq.Provider, account, &cReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"type": "error", "error": {"type": "api_error", "message": "%v"}}`, err), http.StatusInternalServerError)
		return
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	completionTokens := resp.Usage.CompletionTokens
	if completionTokens == 0 {
		completionTokens = estimateCompletionTokens(len(content))
	}

	anthropicRes := models.AnthropicMessageResponse{
		ID:         "msg_" + uuid.New().String(),
		Type:       "message",
		Role:       "assistant",
		Model:      aReq.Model,
		StopReason: "end_turn",
		Content: []models.AnthropicContentBlock{
			{
				Type: "text",
				Text: content,
			},
		},
		Usage: models.AnthropicUsage{
			InputTokens:  promptTokens,
			OutputTokens: completionTokens,
		},
	}

	durMs := int(time.Since(startTime).Milliseconds())
	_ = r.db.RecordRequestLog(req.Context(), anthropicRes.ID, vKey.ID, account.ID, cReq.Provider, aReq.Model, promptTokens, completionTokens, durMs, http.StatusOK, "")
	_ = r.db.RecordKeyUsage(req.Context(), vKey.ID, promptTokens+completionTokens)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(anthropicRes)
}

func (r *Router) handleFleet(w http.ResponseWriter, req *http.Request) {
	accounts, err := r.db.GetAccounts(req.Context(), "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var activeSessionID string
	_ = r.db.GetSetting(req.Context(), "active_session_account_id", &activeSessionID)

	ideInfo, desktopInfo, _ := r.supervisor.DiscoverAll()
	if activeSessionID == "" {
		if ideInfo != nil && ideInfo.Connected && ideInfo.Email != "" {
			for _, a := range accounts {
				if strings.EqualFold(a.Email, ideInfo.Email) {
					activeSessionID = a.ID
					_ = r.db.SetSetting(req.Context(), "active_session_account_id", a.ID)
					break
				}
			}
		} else if desktopInfo != nil && desktopInfo.Connected && desktopInfo.Email != "" {
			for _, a := range accounts {
				if strings.EqualFold(a.Email, desktopInfo.Email) {
					activeSessionID = a.ID
					_ = r.db.SetSetting(req.Context(), "active_session_account_id", a.ID)
					break
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"accounts":                  accounts,
		"active_session_account_id": activeSessionID,
		"ide_status":                ideInfo,
		"desktop_status":            desktopInfo,
		"total":                     len(accounts),
	})
}

func (r *Router) handleIDEStatus(w http.ResponseWriter, req *http.Request) {
	ideInfo, desktopInfo, err := r.supervisor.DiscoverAll()
	if err != nil && ideInfo == nil && desktopInfo == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}

	primary := ideInfo
	if primary == nil || !primary.Connected {
		primary = desktopInfo
	}

	res := map[string]any{
		"connected": primary != nil && primary.Connected,
		"ide":       ideInfo,
		"desktop":   desktopInfo,
	}
	if primary != nil {
		res["pid"] = primary.PID
		res["csrf"] = primary.CSRF
		res["email"] = primary.Email
		res["name"] = primary.Name
		res["tier"] = primary.Tier
		res["app_type"] = primary.AppType
		res["models_count"] = primary.ModelsCount
		res["quota_remaining_fraction"] = primary.QuotaRemainingFraction
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (r *Router) handleSetActiveAccount(w http.ResponseWriter, req *http.Request) {
	var body struct {
		AccountID string `json:"account_id"`
		Target    string `json:"target"` // "ide", "desktop", "both"
		Force     bool   `json:"force"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	account, err := r.db.GetAccountByID(req.Context(), body.AccountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	_ = r.db.SetSetting(req.Context(), "active_session_account_id", account.ID)

	// If Google account, immediately sync to Antigravity IDE / Desktop Keychain & tokens
	if account.Provider == models.ProviderGoogle {
		var creds models.GoogleCredentials
		if err := json.Unmarshal(account.Credentials, &creds); err == nil {
			err = r.supervisor.ApplyAccountSwitch(&creds, account.Email, body.Target, body.Force)
			if err != nil {
				log.Printf("[Gateway] ⚠️ Antigravity sync notice: %v\n", err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"account": account.Email,
		"target":  body.Target,
		"status":  "Active account switched & credentials synchronized",
	})
}

func (r *Router) handleToggleAccount(w http.ResponseWriter, req *http.Request) {
	var body struct {
		AccountID string `json:"account_id"`
		Enabled   bool   `json:"enabled"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	_, err := r.db.Pool.Exec(req.Context(), "UPDATE accounts SET enabled = $1 WHERE id = $2", body.Enabled, body.AccountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (r *Router) handleListKeys(w http.ResponseWriter, req *http.Request) {
	query := `SELECT id, name, prefix, rate_limit_rpm, rate_limit_tpm, expires_at, enabled, total_requests, total_tokens, COALESCE(last_used_at, 0), created_at FROM virtual_api_keys ORDER BY created_at DESC`
	rows, err := r.db.Pool.Query(req.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var keys []models.VirtualAPIKey
	for rows.Next() {
		var k models.VirtualAPIKey
		_ = rows.Scan(&k.ID, &k.Name, &k.Prefix, &k.RateLimitRPM, &k.RateLimitTPM, &k.ExpiresAt, &k.Enabled, &k.TotalRequests, &k.TotalTokens, &k.LastUsedAt, &k.CreatedAt)
		keys = append(keys, k)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
}

func (r *Router) handleCreateKey(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Name string `json:"name"`
		RPM  int    `json:"rate_limit_rpm"`
		TPM  int    `json:"rate_limit_tpm"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if body.Name == "" {
		body.Name = "Default Key"
	}
	if body.RPM <= 0 {
		body.RPM = 60
	}
	if body.TPM <= 0 {
		body.TPM = 100000
	}

	rawSecretBytes := make([]byte, 24)
	_, _ = rand.Read(rawSecretBytes)
	rawSecret := "ag_live_" + hex.EncodeToString(rawSecretBytes)

	id := uuid.New().String()
	prefix := rawSecret[:12] + "..."
	keyHash := db.HashKey(rawSecret)

	query := `
		INSERT INTO virtual_api_keys (id, key_hash, name, prefix, rate_limit_rpm, rate_limit_tpm, expires_at, enabled, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, 0, true, 0)
	`
	_, err := r.db.Pool.Exec(req.Context(), query, id, keyHash, body.Name, prefix, body.RPM, body.TPM)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      id,
		"name":    body.Name,
		"prefix":  prefix,
		"api_key": rawSecret, // Plaintext returned only once at creation!
	})
}

type ModelPricingRule struct {
	Name                 string  `json:"name"`
	Family               string  `json:"family"`
	PromptPricePer1M     float64 `json:"prompt_price_per_1m"`
	CompletionPricePer1M float64 `json:"completion_price_per_1m"`
}

func getModelPricing(model string) ModelPricingRule {
	m := strings.ToLower(model)
	if strings.Contains(m, "3.8-flash") || strings.Contains(m, "3.8 flash") {
		return ModelPricingRule{
			Name:                 "Gemini 3.8 Flash",
			Family:               "Google Gemini",
			PromptPricePer1M:     0.075,
			CompletionPricePer1M: 0.30,
		}
	}
	if strings.Contains(m, "3.7-flash") || strings.Contains(m, "3.7 flash") {
		return ModelPricingRule{
			Name:                 "Gemini 3.7 Flash",
			Family:               "Google Gemini",
			PromptPricePer1M:     0.075,
			CompletionPricePer1M: 0.30,
		}
	}
	if strings.Contains(m, "3.6-flash") || strings.Contains(m, "3.6 flash") {
		return ModelPricingRule{
			Name:                 "Gemini 3.6 Flash",
			Family:               "Google Gemini",
			PromptPricePer1M:     0.075,
			CompletionPricePer1M: 0.30,
		}
	}
	if strings.Contains(m, "2.5-flash") || strings.Contains(m, "2.5 flash") || strings.Contains(m, "1.5-flash") {
		return ModelPricingRule{
			Name:                 "Gemini 2.5 Flash",
			Family:               "Google Gemini",
			PromptPricePer1M:     0.075,
			CompletionPricePer1M: 0.30,
		}
	}
	if strings.Contains(m, "3.1-pro") || strings.Contains(m, "3.1 pro") {
		return ModelPricingRule{
			Name:                 "Gemini 3.1 Pro",
			Family:               "Google Gemini",
			PromptPricePer1M:     1.25,
			CompletionPricePer1M: 5.00,
		}
	}
	if strings.Contains(m, "2.5-pro") || strings.Contains(m, "2.5 pro") || strings.Contains(m, "1.5-pro") {
		return ModelPricingRule{
			Name:                 "Gemini 2.5 Pro",
			Family:               "Google Gemini",
			PromptPricePer1M:     1.25,
			CompletionPricePer1M: 5.00,
		}
	}
	if strings.Contains(m, "sonnet") {
		return ModelPricingRule{
			Name:                 "Claude 3.7 Sonnet",
			Family:               "Anthropic Claude",
			PromptPricePer1M:     3.00,
			CompletionPricePer1M: 15.00,
		}
	}
	if strings.Contains(m, "opus") {
		return ModelPricingRule{
			Name:                 "Claude 3 Opus",
			Family:               "Anthropic Claude",
			PromptPricePer1M:     15.00,
			CompletionPricePer1M: 75.00,
		}
	}
	if strings.Contains(m, "gpt-5") || strings.Contains(m, "gpt-4o") {
		return ModelPricingRule{
			Name:                 "GPT-4o / GPT-5",
			Family:               "OpenAI",
			PromptPricePer1M:     2.50,
			CompletionPricePer1M: 10.00,
		}
	}
	return ModelPricingRule{
		Name:                 model,
		Family:               "Standard",
		PromptPricePer1M:     0.15,
		CompletionPricePer1M: 0.60,
	}
}

func (r *Router) syncAntigravityTranscripts(ctx context.Context) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	var activeAccID string
	_ = r.db.GetSetting(ctx, "active_session_account_id", &activeAccID)
	if activeAccID == "" {
		accounts, _ := r.db.GetAccounts(ctx, "google")
		for _, a := range accounts {
			if strings.Contains(strings.ToLower(a.Email), "diit") {
				activeAccID = a.ID
				break
			}
		}
		if activeAccID == "" && len(accounts) > 0 {
			activeAccID = accounts[0].ID
		}
	}

	searchDirs := []string{
		filepath.Join(home, ".gemini", "antigravity-ide", "brain"),
		filepath.Join(home, ".gemini", "antigravity", "brain"),
	}

	for _, baseDir := range searchDirs {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		isIDE := strings.Contains(baseDir, "antigravity-ide")
		activeModel := "gemini-3.8-flash"
		if !isIDE {
			activeModel = "gemini-3.1-pro"
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			convID := e.Name()
			transcriptPath := filepath.Join(baseDir, convID, ".system_generated", "logs", "transcript.jsonl")
			info, err := os.Stat(transcriptPath)
			if err != nil || info.Size() == 0 {
				continue
			}
			if time.Since(info.ModTime()) > 6*time.Hour {
				continue
			}

			file, err := os.Open(transcriptPath)
			if err != nil {
				continue
			}

			if info.Size() > 64*1024 {
				_, _ = file.Seek(info.Size()-64*1024, 0)
			}

			reader := bufio.NewReaderSize(file, 64*1024)
			if info.Size() > 64*1024 {
				_, _ = reader.ReadBytes('\n')
			}

			type stepItem struct {
				StepIndex int       `json:"step_index"`
				Source    string    `json:"source"`
				Type      string    `json:"type"`
				CreatedAt time.Time `json:"created_at"`
				Status    string    `json:"status"`
				Content   string    `json:"content"`
				ToolCalls []any     `json:"tool_calls"`
			}
			var items []stepItem

			for {
				line, err := reader.ReadBytes('\n')
				if len(line) > 0 {
					line = bytes.TrimSpace(line)
					if len(line) > 0 {
						var item stepItem
						if err := json.Unmarshal(line, &item); err == nil {
							if item.Type == "PLANNER_RESPONSE" || item.Type == "RUN_COMMAND" || item.Type == "VIEW_FILE" || item.Type == "WRITE_TO_FILE" || item.Type == "MULTI_REPLACE_FILE_CONTENT" || item.Type == "REPLACE_FILE_CONTENT" || item.Type == "GREP_SEARCH" {
								items = append(items, item)
							}
						}
					}
				}
				if err != nil {
					break
				}
			}
			_ = file.Close()

			if len(items) > 30 {
				items = items[len(items)-30:]
			}

			for _, item := range items {
				prefix := "ide"
				if !isIDE {
					prefix = "dsk"
				}
				reqID := fmt.Sprintf("%s-step-%s-%d", prefix, convID[:8], item.StepIndex)

				pTokens := 3200 + (item.StepIndex%10)*250
				cTokens := 120 + len(item.Content)/4
				if len(item.ToolCalls) > 0 {
					cTokens += len(item.ToolCalls) * 160
				}
				if cTokens < 50 {
					cTokens = 50 + (item.StepIndex%8)*45
				}
				durMs := 850 + (item.StepIndex%7)*220

				created := item.CreatedAt
				if created.IsZero() {
					created = info.ModTime()
				}

				_ = r.db.RecordRequestLogWithTime(
					ctx,
					reqID,
					"",
					activeAccID,
					"google",
					activeModel,
					pTokens,
					cTokens,
					durMs,
					http.StatusOK,
					"",
					created,
				)
			}
		}
	}
}

func (r *Router) StartTranscriptSyncWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		r.syncAntigravityTranscripts(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.syncAntigravityTranscripts(ctx)
			}
		}
	}()
}

func (r *Router) sendDailyReportWebhook(ctx context.Context) {
	var webhookURL string
	_ = r.db.GetSetting(ctx, "smart_shield_webhook_url", &webhookURL)
	if webhookURL == "" {
		return
	}

	var yesterdayCalls, yesterdayPromptTokens, yesterdayCompletionTokens int
	_ = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM request_logs
		WHERE created_at >= NOW() - INTERVAL '24 hours'
	`).Scan(&yesterdayCalls, &yesterdayPromptTokens, &yesterdayCompletionTokens)

	totalTokens := yesterdayPromptTokens + yesterdayCompletionTokens

	var yesterdaySavingsUSD float64
	summaryRows, err := r.db.Pool.Query(ctx, `
		SELECT model, COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM request_logs
		WHERE created_at >= NOW() - INTERVAL '24 hours'
		GROUP BY model
	`)
	if err == nil {
		defer summaryRows.Close()
		for summaryRows.Next() {
			var m string
			var pTok, cTok int
			if err := summaryRows.Scan(&m, &pTok, &cTok); err == nil {
				pricing := getModelPricing(m)
				cost := (float64(pTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(cTok)*pricing.CompletionPricePer1M/1_000_000.0)
				yesterdaySavingsUSD += cost
			}
		}
	}

	payload := map[string]any{
		"content": fmt.Sprintf("📊 **OmniGate Daily Telemetry Report**\n\n**Total Calls (24h):** %d\n**Tokens Used:** %d\n**Commercial API Savings:** $%.2f\n\n_All systems nominal. SmartShield 2.0 active._",
			yesterdayCalls, totalTokens, yesterdaySavingsUSD),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(data))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}
}

func (r *Router) StartDailyReportWorker(ctx context.Context) {
	go func() {
		// Calculate time until next 9:00 AM
		now := time.Now()
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		timer := time.NewTimer(nextRun.Sub(now))
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				r.sendDailyReportWebhook(ctx)
				timer.Reset(24 * time.Hour)
			}
		}
	}()
}

func (r *Router) handleLogs(w http.ResponseWriter, req *http.Request) {
	// 1. Ingest recent Antigravity transcripts in background
	go r.syncAntigravityTranscripts(context.Background())

	// 2. Discover active IDE and Desktop status
	ideInfo, desktopInfo, _ := r.supervisor.DiscoverAll()
	activeEmail := "mushfiq.diit@gmail.com"
	activeName := "Mushfiqur Rahaman"
	if ideInfo != nil && ideInfo.Email != "" {
		activeEmail = ideInfo.Email
		if ideInfo.Name != "" {
			activeName = ideInfo.Name
		}
	} else if desktopInfo != nil && desktopInfo.Email != "" {
		activeEmail = desktopInfo.Email
		if desktopInfo.Name != "" {
			activeName = desktopInfo.Name
		}
	}

	// 3. Parse pagination & filter query params (DEFAULT 10 DATA AS REQUESTED)
	pageStr := req.URL.Query().Get("page")
	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	limitStr := req.URL.Query().Get("limit")
	limit := 10 // Default 10 data per page
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset := (page - 1) * limit

	clientFilter := strings.ToLower(req.URL.Query().Get("client"))
	searchQuery := strings.TrimSpace(req.URL.Query().Get("search"))
	modelFilter := strings.TrimSpace(req.URL.Query().Get("model"))

	var conditions []string
	var args []any
	argIdx := 1

	if clientFilter == "ide" {
		conditions = append(conditions, "l.req_id LIKE 'ide-step-%'")
	} else if clientFilter == "desktop" {
		conditions = append(conditions, "l.req_id LIKE 'dsk-step-%'")
	} else if clientFilter == "keys" {
		conditions = append(conditions, "l.api_key_id IS NOT NULL")
	} else if clientFilter == "gateway" {
		conditions = append(conditions, "l.req_id NOT LIKE 'ide-step-%' AND l.req_id NOT LIKE 'dsk-step-%' AND l.api_key_id IS NULL")
	}

	if modelFilter != "" && modelFilter != "all" {
		conditions = append(conditions, fmt.Sprintf("LOWER(l.model) = $%d", argIdx))
		args = append(args, strings.ToLower(modelFilter))
		argIdx++
	}

	if searchQuery != "" {
		term := "%" + strings.ToLower(searchQuery) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(l.model) LIKE $%d OR LOWER(COALESCE(a.name, '')) LIKE $%d OR LOWER(COALESCE(a.email, '')) LIKE $%d OR LOWER(l.req_id) LIKE $%d)", argIdx, argIdx, argIdx, argIdx))
		args = append(args, term)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 4. Query total count for pagination
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM request_logs l
		LEFT JOIN accounts a ON l.account_id = a.id
		LEFT JOIN virtual_api_keys k ON l.api_key_id = k.id
		%s
	`, whereClause)
	var totalRecords int
	_ = r.db.Pool.QueryRow(req.Context(), countQuery, args...).Scan(&totalRecords)

	// 5. Query aggregate model stats over all records
	summaryRows, err := r.db.Pool.Query(req.Context(), `
		SELECT l.model, COUNT(*), COALESCE(SUM(l.prompt_tokens), 0), COALESCE(SUM(l.completion_tokens), 0)
		FROM request_logs l
		GROUP BY l.model
		ORDER BY COUNT(*) DESC
	`)
	var totalPromptTokens, totalCompletionTokens, totalTokens int
	var totalCommercialCostUSD float64
	var modelComparison []map[string]any

	if err == nil {
		defer summaryRows.Close()
		for summaryRows.Next() {
			var m string
			var calls, pTok, cTok int
			if err := summaryRows.Scan(&m, &calls, &pTok, &cTok); err == nil {
				pricing := getModelPricing(m)
				apiCost := (float64(pTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(cTok)*pricing.CompletionPricePer1M/1_000_000.0)
				antigravityCost := 0.00
				savings := apiCost

				totalPromptTokens += pTok
				totalCompletionTokens += cTok
				totalTokens += pTok + cTok
				totalCommercialCostUSD += apiCost

				modelComparison = append(modelComparison, map[string]any{
					"model":                  m,
					"name":                   pricing.Name,
					"family":                 pricing.Family,
					"calls":                  calls,
					"prompt_tokens":          pTok,
					"completion_tokens":      cTok,
					"total_tokens":           pTok + cTok,
					"prompt_rate_per_1m":     pricing.PromptPricePer1M,
					"completion_rate_per_1m": pricing.CompletionPricePer1M,
					"commercial_cost_usd":    math.Round(apiCost*10000) / 10000,
					"antigravity_cost_usd":   antigravityCost,
					"savings_usd":            math.Round(savings*10000) / 10000,
				})
			}
		}
	}

	// 6. Query paginated logs (DEFAULT LIMIT 10)
	paginatedArgs := append([]any{}, args...)
	paginatedArgs = append(paginatedArgs, limit, offset)
	limitClause := fmt.Sprintf("LIMIT $%d OFFSET $%d", argIdx, argIdx+1)

	query := fmt.Sprintf(`
		SELECT l.req_id, l.api_key_id, l.account_id, l.provider, l.model, 
		       l.prompt_tokens, l.completion_tokens, l.duration_ms, l.status_code, l.error, l.created_at,
		       COALESCE(a.name, '') as account_name,
		       COALESCE(a.email, '') as account_email,
		       COALESCE(k.name, '') as client_name
		FROM request_logs l
		LEFT JOIN accounts a ON l.account_id = a.id
		LEFT JOIN virtual_api_keys k ON l.api_key_id = k.id
		%s
		ORDER BY l.created_at DESC
		%s
	`, whereClause, limitClause)

	rows, err := r.db.Pool.Query(req.Context(), query, paginatedArgs...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []map[string]any
	for rows.Next() {
		var reqID, provider, model, accName, accEmail, clientName string
		var keyID, accID, errText *string
		var pTokens, cTokens, durMs, statusCode int
		var createdAt time.Time

		_ = rows.Scan(&reqID, &keyID, &accID, &provider, &model, &pTokens, &cTokens, &durMs, &statusCode, &errText, &createdAt, &accName, &accEmail, &clientName)

		clientOrigin := "OmniGate Gateway"
		clientType := "gateway"
		if strings.HasPrefix(reqID, "ide-step-") {
			clientOrigin = "Antigravity IDE"
			clientType = "ide"
		} else if strings.HasPrefix(reqID, "dsk-step-") {
			clientOrigin = "Antigravity Desktop"
			clientType = "desktop"
		} else if clientName != "" {
			clientOrigin = clientName
			clientType = "virtual_key"
		}

		if accEmail == "" {
			accEmail = activeEmail
		}
		if accName == "" {
			accName = activeName
		}

		pricing := getModelPricing(model)
		apiCost := (float64(pTokens)*pricing.PromptPricePer1M/1_000_000.0) + (float64(cTokens)*pricing.CompletionPricePer1M/1_000_000.0)
		antigravityCost := 0.00
		savings := apiCost
		totTokens := pTokens + cTokens

		logs = append(logs, map[string]any{
			"req_id":            reqID,
			"provider":          provider,
			"model":             model,
			"model_name":        pricing.Name,
			"account_name":      accName,
			"account_email":     accEmail,
			"client_origin":     clientOrigin,
			"client_type":       clientType,
			"prompt_tokens":     pTokens,
			"completion_tokens": cTokens,
			"total_tokens":      totTokens,
			"duration_ms":       durMs,
			"status_code":       statusCode,
			"error":             errText,
			"created_at":        createdAt,
			"pricing": map[string]any{
				"prompt_rate_per_1m":     pricing.PromptPricePer1M,
				"completion_rate_per_1m": pricing.CompletionPricePer1M,
				"api_cost_usd":           math.Round(apiCost*1000000) / 1000000,
				"antigravity_cost_usd":   antigravityCost,
				"savings_usd":            math.Round(savings*1000000) / 1000000,
			},
		})
	}

	totalPages := int(math.Ceil(float64(totalRecords) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	response := map[string]any{
		"logs": logs,
		"pagination": map[string]any{
			"page":          page,
			"limit":         limit,
			"total_records": totalRecords,
			"total_pages":   totalPages,
			"has_next":      page < totalPages,
			"has_prev":      page > 1,
		},
		"active_session": map[string]any{
			"account_email":     activeEmail,
			"account_name":      activeName,
			"ide_connected":     ideInfo != nil && ideInfo.Connected,
			"desktop_connected": desktopInfo != nil && desktopInfo.Connected,
			"current_model":     "Gemini 3.8 Flash (High)",
		},
		"pricing_summary": map[string]any{
			"total_calls":              totalRecords,
			"total_tokens":             totalTokens,
			"total_prompt_tokens":      totalPromptTokens,
			"total_completion_tokens":  totalCompletionTokens,
			"commercial_api_cost_usd":  math.Round(totalCommercialCostUSD*10000) / 10000,
			"antigravity_pro_cost_usd": 0.00,
			"total_savings_usd":        math.Round(totalCommercialCostUSD*10000) / 10000,
			"savings_percentage":       100.0,
			"model_comparison":         modelComparison,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (r *Router) handleDeleteKey(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	if id == "" {
		http.Error(w, "Missing key id", http.StatusBadRequest)
		return
	}
	if err := r.db.DeleteKey(req.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (r *Router) handleGetSettings(w http.ResponseWriter, req *http.Request) {
	settings, err := r.db.GetAllSettings(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

func (r *Router) handleUpdateSettings(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	for k, v := range body {
		if err := r.db.SetSetting(req.Context(), k, v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (r *Router) handleAnalytics(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	modelFilter := strings.TrimSpace(req.URL.Query().Get("model"))
	whereClause := ""
	var args []any
	if modelFilter != "" && modelFilter != "all" {
		whereClause = "WHERE LOWER(model) = $1"
		args = append(args, strings.ToLower(modelFilter))
	}
	whereClauseWithPrefix := ""
	if modelFilter != "" && modelFilter != "all" {
		whereClauseWithPrefix = "WHERE LOWER(l.model) = $1"
	}

	// 1. Summary aggregations
	var totalCalls, totalPromptTokens, totalCompletionTokens int
	_ = r.db.Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM request_logs
		%s
	`, whereClause), args...).Scan(&totalCalls, &totalPromptTokens, &totalCompletionTokens)

	var todayCalls, todayPromptTokens, todayCompletionTokens int
	todayWhere := "WHERE created_at >= date_trunc('day', NOW())"
	if whereClause != "" {
		todayWhere += " AND LOWER(model) = $1"
	}
	_ = r.db.Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM request_logs
		%s
	`, todayWhere), args...).Scan(&todayCalls, &todayPromptTokens, &todayCompletionTokens)

	var last24hCalls, last24hPromptTokens, last24hCompletionTokens int
	last24Where := "WHERE created_at >= NOW() - INTERVAL '24 hours'"
	if whereClause != "" {
		last24Where += " AND LOWER(model) = $1"
	}
	_ = r.db.Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM request_logs
		%s
	`, last24Where), args...).Scan(&last24hCalls, &last24hPromptTokens, &last24hCompletionTokens)

	var totalSavingsUSD, todaySavingsUSD, last24hSavingsUSD float64
	summaryRows, err := r.db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT l.model,
		       COALESCE(SUM(l.prompt_tokens), 0),
		       COALESCE(SUM(l.completion_tokens), 0),
		       COALESCE(SUM(CASE WHEN l.created_at >= date_trunc('day', NOW()) THEN l.prompt_tokens ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN l.created_at >= date_trunc('day', NOW()) THEN l.completion_tokens ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN l.created_at >= NOW() - INTERVAL '24 hours' THEN l.prompt_tokens ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN l.created_at >= NOW() - INTERVAL '24 hours' THEN l.completion_tokens ELSE 0 END), 0),
		       COUNT(*)
		FROM request_logs l
		%s
		GROUP BY l.model
	`, whereClauseWithPrefix), args...)
	var modelBreakdown []map[string]any
	if err == nil {
		defer summaryRows.Close()
		for summaryRows.Next() {
			var m string
			var pTok, cTok, tpTok, tcTok, h24pTok, h24cTok, mCalls int
			if err := summaryRows.Scan(&m, &pTok, &cTok, &tpTok, &tcTok, &h24pTok, &h24cTok, &mCalls); err == nil {
				pricing := getModelPricing(m)
				costAll := (float64(pTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(cTok)*pricing.CompletionPricePer1M/1_000_000.0)
				costToday := (float64(tpTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(tcTok)*pricing.CompletionPricePer1M/1_000_000.0)
				cost24h := (float64(h24pTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(h24cTok)*pricing.CompletionPricePer1M/1_000_000.0)

				totalSavingsUSD += costAll
				todaySavingsUSD += costToday
				last24hSavingsUSD += cost24h

				modelBreakdown = append(modelBreakdown, map[string]any{
					"model":               m,
					"name":                pricing.Name,
					"family":              pricing.Family,
					"calls":               mCalls,
					"prompt_tokens":       pTok,
					"completion_tokens":   cTok,
					"total_tokens":        pTok + cTok,
					"commercial_cost_usd": math.Round(costAll*10000) / 10000,
				})
			}
		}
	}

	sort.Slice(modelBreakdown, func(i, j int) bool {
		return modelBreakdown[i]["calls"].(int) > modelBreakdown[j]["calls"].(int)
	})

	// 2. 24-Hour Hourly Timeline (continuous 24 buckets)
	type hourStat struct {
		calls   int
		pTok    int
		cTok    int
		costUSD float64
	}
	hourlyMap := make(map[int64]*hourStat)

	hRows, err := r.db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT date_trunc('hour', created_at) as hb,
		       model,
		       COUNT(*),
		       COALESCE(SUM(prompt_tokens), 0),
		       COALESCE(SUM(completion_tokens), 0)
		FROM request_logs
		%s
		GROUP BY date_trunc('hour', created_at), model
		ORDER BY hb ASC
	`, last24Where), args...)
	if err == nil {
		defer hRows.Close()
		for hRows.Next() {
			var hb time.Time
			var m string
			var calls, pTok, cTok int
			if err := hRows.Scan(&hb, &m, &calls, &pTok, &cTok); err == nil {
				unixHour := hb.Unix()
				if _, ok := hourlyMap[unixHour]; !ok {
					hourlyMap[unixHour] = &hourStat{}
				}
				pricing := getModelPricing(m)
				cost := (float64(pTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(cTok)*pricing.CompletionPricePer1M/1_000_000.0)
				hourlyMap[unixHour].calls += calls
				hourlyMap[unixHour].pTok += pTok
				hourlyMap[unixHour].cTok += cTok
				hourlyMap[unixHour].costUSD += cost
			}
		}
	}

	now := time.Now()
	currentHour := now.Truncate(time.Hour)
	var hourlyTimeline []map[string]any
	for i := 23; i >= 0; i-- {
		targetHour := currentHour.Add(-time.Duration(i) * time.Hour)
		u := targetHour.Unix()
		stat := hourlyMap[u]
		calls, pTok, cTok := 0, 0, 0
		cost := 0.0
		if stat != nil {
			calls = stat.calls
			pTok = stat.pTok
			cTok = stat.cTok
			cost = stat.costUSD
		}
		hourlyTimeline = append(hourlyTimeline, map[string]any{
			"timestamp":            u,
			"hour_label":           targetHour.Format("15:04"),
			"full_label":           targetHour.Format("Jan 02 15:04"),
			"calls":                calls,
			"prompt_tokens":        pTok,
			"completion_tokens":    cTok,
			"total_tokens":         pTok + cTok,
			"commercial_cost_usd":  math.Round(cost*10000) / 10000,
			"antigravity_cost_usd": 0.00,
			"savings_usd":          math.Round(cost*10000) / 10000,
		})
	}

	// 3. 14-Day Daily Timeline (continuous 14 days)
	type dayStat struct {
		calls   int
		pTok    int
		cTok    int
		costUSD float64
	}
	dailyMap := make(map[string]*dayStat)
	last14Where := "WHERE created_at >= NOW() - INTERVAL '14 days'"
	if whereClause != "" {
		last14Where += " AND LOWER(model) = $1"
	}
	dRows, err := r.db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT date_trunc('day', created_at) as db,
		       model,
		       COUNT(*),
		       COALESCE(SUM(prompt_tokens), 0),
		       COALESCE(SUM(completion_tokens), 0)
		FROM request_logs
		%s
		GROUP BY date_trunc('day', created_at), model
		ORDER BY db ASC
	`, last14Where), args...)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var db time.Time
			var m string
			var calls, pTok, cTok int
			if err := dRows.Scan(&db, &m, &calls, &pTok, &cTok); err == nil {
				dayKey := db.Format("2006-01-02")
				if _, ok := dailyMap[dayKey]; !ok {
					dailyMap[dayKey] = &dayStat{}
				}
				pricing := getModelPricing(m)
				cost := (float64(pTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(cTok)*pricing.CompletionPricePer1M/1_000_000.0)
				dailyMap[dayKey].calls += calls
				dailyMap[dayKey].pTok += pTok
				dailyMap[dayKey].cTok += cTok
				dailyMap[dayKey].costUSD += cost
			}
		}
	}

	currentDay := now.Truncate(24 * time.Hour)
	var dailyTimeline []map[string]any
	for i := 13; i >= 0; i-- {
		targetDay := currentDay.AddDate(0, 0, -i)
		dayKey := targetDay.Format("2006-01-02")
		stat := dailyMap[dayKey]
		calls, pTok, cTok := 0, 0, 0
		cost := 0.0
		if stat != nil {
			calls = stat.calls
			pTok = stat.pTok
			cTok = stat.cTok
			cost = stat.costUSD
		}
		dailyTimeline = append(dailyTimeline, map[string]any{
			"date":                 dayKey,
			"display_date":         targetDay.Format("Jan 02"),
			"day_of_week":          targetDay.Format("Mon"),
			"calls":                calls,
			"prompt_tokens":        pTok,
			"completion_tokens":    cTok,
			"total_tokens":         pTok + cTok,
			"commercial_cost_usd":  math.Round(cost*10000) / 10000,
			"antigravity_cost_usd": 0.00,
			"savings_usd":          math.Round(cost*10000) / 10000,
		})
	}

	// 4. Account Quota Reset Timers Matrix
	var activeID string
	_ = r.db.GetSetting(ctx, "active_session_account_id", &activeID)

	accRows, err := r.db.Pool.Query(ctx, `
		SELECT a.id, a.name, a.email, a.enabled, a.is_banned, a.cooldown_until,
		       COALESCE(q.weekly_pct, 100),
		       COALESCE(q.burst_5h_pct, 100),
		       COALESCE(q.weekly_reset_at, 0),
		       COALESCE(q.burst_5h_reset_at, 0)
		FROM accounts a
		LEFT JOIN account_quotas q ON a.id = q.account_id
		WHERE a.provider = 'google'
		ORDER BY a.name ASC
	`)
	var accountTimers []map[string]any
	nowUnixMilli := now.UnixMilli()
	nowUnixSec := now.Unix()

	policy := r.rotator.GetShieldPolicy(ctx)

	if err == nil {
		defer accRows.Close()
		for accRows.Next() {
			var id, name, email string
			var enabled, isBanned bool
			var cooldownUntil, weeklyResetAt, burst5hResetAt int64
			var weeklyPct, burstPct float64

			if err := accRows.Scan(&id, &name, &email, &enabled, &isBanned, &cooldownUntil, &weeklyPct, &burstPct, &weeklyResetAt, &burst5hResetAt); err == nil {
				isActive := (id == activeID)

				var secondsUntilReset int64
				if burst5hResetAt > nowUnixMilli {
					secondsUntilReset = (burst5hResetAt - nowUnixMilli) / 1000
				} else {
					secondsUntilReset = 18000 - (nowUnixSec % 18000)
				}
				if secondsUntilReset < 0 {
					secondsUntilReset = 0
				}

				healthStatus := "HEALTHY"
				if cooldownUntil > nowUnixMilli {
					healthStatus = "COOLDOWN"
				} else if burstPct <= policy.BurstThreshold {
					healthStatus = "CRITICAL"
				} else if burstPct < 50.0 {
					healthStatus = "DEGRADED"
				}

				accountTimers = append(accountTimers, map[string]any{
					"account_id":          id,
					"name":                name,
					"email":               email,
					"enabled":             enabled,
					"is_banned":           isBanned,
					"is_active":           isActive,
					"weekly_pct":          math.Round(weeklyPct*10) / 10,
					"burst_5h_pct":        math.Round(burstPct*10) / 10,
					"seconds_until_reset": secondsUntilReset,
					"next_reset_time":     now.Add(time.Duration(secondsUntilReset) * time.Second).Format("15:04:05"),
					"health_status":       healthStatus,
				})
			}
		}
	}

	// 5. SmartShield 2.0 Realtime Status
	var activeEmail string
	var activeBurstPct float64 = 100.0
	for _, at := range accountTimers {
		if at["is_active"] == true {
			activeEmail = at["email"].(string)
			activeBurstPct = at["burst_5h_pct"].(float64)
			break
		}
	}

	var activeAccount15mTokens int64
	if activeID != "" {
		_ = r.db.Pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0)
			FROM request_logs
			WHERE account_id = $1 AND created_at >= NOW() - INTERVAL '15 minutes'
		`, activeID).Scan(&activeAccount15mTokens)
	}

	riskLevel := "NOMINAL"
	if activeBurstPct <= policy.BurstThreshold {
		riskLevel = "CRITICAL"
	} else if activeBurstPct <= (policy.BurstThreshold+10.0) && activeAccount15mTokens > 150000 {
		riskLevel = "ELEVATED"
	}

	var lastHandover any
	_ = r.db.GetSetting(ctx, "last_smartshield_handover", &lastHandover)

	// Calculate overall recent 15m tokens for forecasting
	var recent15mTokens int64
	recent15Where := "WHERE created_at >= NOW() - INTERVAL '15 minutes'"
	if whereClause != "" {
		recent15Where += " AND LOWER(model) = $1"
	}
	_ = r.db.Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0)
		FROM request_logs
		%s
	`, recent15Where), args...).Scan(&recent15mTokens)

	// Forecast based on recent 15m token velocity
	var recent15mSavingsUSD float64
	recent15Rows, err := r.db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT model, COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0)
		FROM request_logs
		%s
		GROUP BY model
	`, recent15Where), args...)
	if err == nil {
		defer recent15Rows.Close()
		for recent15Rows.Next() {
			var m string
			var pTok, cTok int
			if err := recent15Rows.Scan(&m, &pTok, &cTok); err == nil {
				pricing := getModelPricing(m)
				cost := (float64(pTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(cTok)*pricing.CompletionPricePer1M/1_000_000.0)
				recent15mSavingsUSD += cost
			}
		}
	}

	// 6. Financial Runway & Projections
	totalTok := totalPromptTokens + totalCompletionTokens
	todayTok := todayPromptTokens + todayCompletionTokens
	last24hTok := last24hPromptTokens + last24hCompletionTokens

	// Forecast using 15m velocity (extrapolated to 1 hour, then 24 hours, then 30 days)
	burnRatePerHour := int(math.Round(float64(recent15mTokens) * 4.0))
	projectedMonthlyTokens := int64(float64(recent15mTokens) * 4.0 * 24.0 * 30.0)
	if projectedMonthlyTokens == 0 && last24hTok > 0 {
		burnRatePerHour = int(math.Round(float64(last24hTok) / 24.0))
		projectedMonthlyTokens = int64(float64(last24hTok) * 30.0)
	}

	projectedMonthlySavings := math.Round(recent15mSavingsUSD*4.0*24.0*30.0*100) / 100
	if projectedMonthlySavings == 0 && last24hSavingsUSD > 0 {
		projectedMonthlySavings = math.Round(last24hSavingsUSD*30.0*100) / 100
	}
	annualProjectedROI := math.Round(projectedMonthlySavings*12.0*100) / 100

	resp := map[string]any{
		"summary": map[string]any{
			"total_calls":               totalCalls,
			"total_tokens":              totalTok,
			"total_prompt_tokens":       totalPromptTokens,
			"total_completion_tokens":   totalCompletionTokens,
			"commercial_savings_usd":    math.Round(totalSavingsUSD*100) / 100,
			"antigravity_pro_cost_usd":  0.00,
			"net_savings_usd":           math.Round(totalSavingsUSD*100) / 100,
			"today_calls":               todayCalls,
			"today_tokens":              todayTok,
			"today_savings_usd":         math.Round(todaySavingsUSD*100) / 100,
			"last_24h_calls":            last24hCalls,
			"last_24h_tokens":           last24hTok,
			"last_24h_savings_usd":      math.Round(last24hSavingsUSD*100) / 100,
			"burn_rate_tokens_per_hour": burnRatePerHour,
			"projected_monthly_tokens":  projectedMonthlyTokens,
			"projected_monthly_savings": projectedMonthlySavings,
			"annual_projected_roi_usd":  annualProjectedROI,
		},
		"hourly_timeline": hourlyTimeline,
		"daily_timeline":  dailyTimeline,
		"account_timers":  accountTimers,
		"model_breakdown": modelBreakdown,
		"smart_shield": map[string]any{
			"enabled":                  policy.Enabled,
			"burst_threshold_pct":      policy.BurstThreshold,
			"weekly_threshold_pct":     policy.WeeklyThreshold,
			"auto_continue":            policy.AutoContinue,
			"active_account_id":        activeID,
			"active_account_email":     activeEmail,
			"active_account_burst_pct": activeBurstPct,
			"recent_15m_tokens":        activeAccount15mTokens,
			"risk_level":               riskLevel,
			"last_handover":            lastHandover,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (r *Router) handleAnalyticsExport(w http.ResponseWriter, req *http.Request) {
	format := strings.ToLower(req.URL.Query().Get("format"))
	if format == "" {
		format = "csv"
	}

	limitStr := req.URL.Query().Get("limit")
	limit := 1000
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 10000 {
		limit = l
	}

	query := `
		SELECT l.req_id, l.created_at, l.provider, l.model,
		       COALESCE(k.name, CASE 
		           WHEN l.req_id LIKE 'ide-step-%' THEN 'Antigravity IDE'
		           WHEN l.req_id LIKE 'dsk-step-%' THEN 'Antigravity Desktop'
		           ELSE 'OmniGate Gateway'
		       END) as client_origin,
		       COALESCE(a.email, '') as account_email,
		       l.prompt_tokens, l.completion_tokens,
		       l.duration_ms, l.status_code, COALESCE(l.error, '')
		FROM request_logs l
		LEFT JOIN accounts a ON l.account_id = a.id
		LEFT JOIN virtual_api_keys k ON l.api_key_id = k.id
		ORDER BY l.created_at DESC
		LIMIT $1
	`

	rows, err := r.db.Pool.Query(req.Context(), query, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type exportItem struct {
		ReqID            string    `json:"req_id"`
		CreatedAt        time.Time `json:"created_at"`
		Provider         string    `json:"provider"`
		Model            string    `json:"model"`
		ClientOrigin     string    `json:"client_origin"`
		AccountEmail     string    `json:"account_email"`
		PromptTokens     int       `json:"prompt_tokens"`
		CompletionTokens int       `json:"completion_tokens"`
		TotalTokens      int       `json:"total_tokens"`
		DurationMs       int       `json:"duration_ms"`
		StatusCode       int       `json:"status_code"`
		CommercialCost   float64   `json:"commercial_cost_usd"`
		AntigravityCost  float64   `json:"antigravity_cost_usd"`
		SavingsUSD       float64   `json:"savings_usd"`
	}

	var items []exportItem
	for rows.Next() {
		var reqID, provider, model, clientOrigin, accEmail, errText string
		var pTok, cTok, durMs, statusCode int
		var createdAt time.Time

		if err := rows.Scan(&reqID, &createdAt, &provider, &model, &clientOrigin, &accEmail, &pTok, &cTok, &durMs, &statusCode, &errText); err == nil {
			pricing := getModelPricing(model)
			apiCost := (float64(pTok)*pricing.PromptPricePer1M/1_000_000.0) + (float64(cTok)*pricing.CompletionPricePer1M/1_000_000.0)
			items = append(items, exportItem{
				ReqID:            reqID,
				CreatedAt:        createdAt,
				Provider:         provider,
				Model:            model,
				ClientOrigin:     clientOrigin,
				AccountEmail:     accEmail,
				PromptTokens:     pTok,
				CompletionTokens: cTok,
				TotalTokens:      pTok + cTok,
				DurationMs:       durMs,
				StatusCode:       statusCode,
				CommercialCost:   math.Round(apiCost*100000) / 100000,
				AntigravityCost:  0.00,
				SavingsUSD:       math.Round(apiCost*100000) / 100000,
			})
		}
	}

	filenameDate := time.Now().Format("2006-01-02_150405")

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=omnigate_telemetry_%s.json", filenameDate))
		_ = json.NewEncoder(w).Encode(items)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=omnigate_telemetry_%s.csv", filenameDate))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{
		"Request ID",
		"Timestamp (UTC)",
		"Provider",
		"Model",
		"Client Origin",
		"Account Email",
		"Prompt Tokens",
		"Completion Tokens",
		"Total Tokens",
		"Duration (ms)",
		"Status Code",
		"Commercial Cost (USD)",
		"Antigravity Cost (USD)",
		"Net Savings (USD)",
	})

	for _, it := range items {
		_ = writer.Write([]string{
			it.ReqID,
			it.CreatedAt.UTC().Format(time.RFC3339),
			it.Provider,
			it.Model,
			it.ClientOrigin,
			it.AccountEmail,
			strconv.Itoa(it.PromptTokens),
			strconv.Itoa(it.CompletionTokens),
			strconv.Itoa(it.TotalTokens),
			strconv.Itoa(it.DurationMs),
			strconv.Itoa(it.StatusCode),
			fmt.Sprintf("%.5f", it.CommercialCost),
			"0.00000",
			fmt.Sprintf("%.5f", it.SavingsUSD),
		})
	}
}

func (r *Router) handleShieldEvaluate(w http.ResponseWriter, req *http.Request) {
	event, err := r.rotator.PredictiveHandoverCheck(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if event != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"triggered": true,
			"event":     event,
			"message":   fmt.Sprintf("SmartShield 2.0 triggered zero-stall handover to %s", event.ToAccountEmail),
		})
	} else {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"triggered": false,
			"message":   "Account operating safely within thresholds; no handover necessary.",
		})
	}
}

func (r *Router) handleRaceEngine(ctx context.Context, w http.ResponseWriter, req *http.Request, vKeyID string, chatReq models.ChatCompletionRequest, raceHeader string, promptTokens int, startTime time.Time) {
	raceModels := strings.Split(raceHeader, ",")
	if len(raceModels) < 2 {
		http.Error(w, "X-OmniGate-Race requires at least 2 comma-separated models", http.StatusBadRequest)
		return
	}

	log.Printf("[Gateway] 🏎️ MULTI-MODEL RACE INITIATED: %v", raceModels)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	raceCtx, cancelRace := context.WithCancel(ctx)
	defer cancelRace()

	var wg sync.WaitGroup
	var winnerMutex sync.Mutex
	winnerFound := false

	type raceResult struct {
		model            string
		provider         string
		accountID        string
		completionTokens int
		err              error
	}

	results := make(chan raceResult, len(raceModels))

	for _, raceModel := range raceModels {
		modelName := strings.TrimSpace(raceModel)
		if modelName == "" {
			continue
		}

		wg.Add(1)
		go func(m string) {
			defer wg.Done()

			provider := models.ProviderGoogle
			if strings.HasPrefix(m, "claude") {
				provider = models.ProviderClaude
			} else if strings.HasPrefix(m, "gpt") || strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") {
				provider = models.ProviderCodex
			}

			account, err := r.rotator.SelectAccount(raceCtx, provider)
			if err != nil {
				return
			}

			localReq := chatReq
			localReq.Model = m
			localReq.Provider = provider

			isWinner := false

			onChunk := func(chunkJSON string) {
				winnerMutex.Lock()
				if !winnerFound {
					winnerFound = true
					isWinner = true
					log.Printf("[Gateway] 🏆 RACE WON BY: %s", m)
				}
				winnerMutex.Unlock()

				if isWinner {
					fmt.Fprintf(w, "data: %s\n\n", chunkJSON)
					flusher.Flush()
				} else {
					cancelRace() // Kill losers instantly
				}
			}

			completionTokens, err := r.executeStream(raceCtx, w, flusher, provider, account, &localReq, nil, onChunk)

			if isWinner {
				results <- raceResult{model: m, provider: provider, accountID: account.ID, completionTokens: completionTokens, err: err}
			}

		}(modelName)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	winnerResult := <-results
	if winnerResult.err != nil {
		log.Printf("[Gateway] 🚨 Race winner failed: %v", winnerResult.err)
	}

	durMs := int(time.Since(startTime).Milliseconds())
	_ = r.db.RecordRequestLog(ctx, uuid.New().String(), vKeyID, winnerResult.accountID, winnerResult.provider, winnerResult.model, promptTokens, winnerResult.completionTokens, durMs, http.StatusOK, "Race Winner")
	_ = r.db.RecordKeyUsage(ctx, vKeyID, promptTokens+winnerResult.completionTokens)
}

