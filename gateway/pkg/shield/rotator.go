package shield

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/antigravity/gateway/pkg/antigravity"
	"github.com/antigravity/gateway/pkg/db"
	"github.com/antigravity/gateway/pkg/models"
	"github.com/antigravity/gateway/pkg/providers"
)

type Rotator struct {
	db         *db.DB
	supervisor *antigravity.Supervisor
	googleCl   *providers.GoogleClient
	claudeCl   *providers.ClaudeClient
	codexCl    *providers.CodexClient
	mu         sync.RWMutex
}

func NewRotator(database *db.DB, sup *antigravity.Supervisor) *Rotator {
	return &Rotator{
		db:         database,
		supervisor: sup,
		googleCl:   providers.NewGoogleClient(),
		claudeCl:   providers.NewClaudeClient(),
		codexCl:    providers.NewCodexClient(),
	}
}

type Policy struct {
	Enabled         bool    `json:"enabled"`
	BurstThreshold  float64 `json:"burst_threshold"`
	WeeklyThreshold float64 `json:"weekly_threshold"`
	AutoContinue    bool    `json:"auto_continue"`
}

func (r *Rotator) GetShieldPolicy(ctx context.Context) Policy {
	p := Policy{
		Enabled:         true,
		BurstThreshold:  15.0,
		WeeklyThreshold: 10.0,
		AutoContinue:    true,
	}

	settings, err := r.db.GetAllSettings(ctx)
	if err != nil {
		return p
	}

	if v, ok := settings["smart_shield_enabled"]; ok {
		if b, ok := v.(bool); ok {
			p.Enabled = b
		}
	}

	parseNum := func(keys ...string) (float64, bool) {
		for _, k := range keys {
			if v, ok := settings[k]; ok {
				switch val := v.(type) {
				case float64:
					return val, true
				case int:
					return float64(val), true
				case string:
					var f float64
					if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
						return f, true
					}
				}
			}
		}
		return 0, false
	}

	if b, ok := parseNum("smart_shield_burst_threshold", "burst_threshold_pct"); ok {
		p.BurstThreshold = b
	}
	if w, ok := parseNum("smart_shield_weekly_threshold", "weekly_threshold_pct"); ok {
		p.WeeklyThreshold = w
	}
	if v, ok := settings["smart_shield_auto_continue"]; ok {
		if b, ok := v.(bool); ok {
			p.AutoContinue = b
		}
	}

	return p
}

// SelectAccount returns the healthiest, non-cooled-down account for a provider
func (r *Rotator) SelectAccount(ctx context.Context, provider string) (*models.Account, error) {
	accounts, err := r.db.GetAccounts(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %w", err)
	}

	policy := r.GetShieldPolicy(ctx)
	now := time.Now().UnixMilli()

	// Proactive Smart Shield quota evaluation
	if policy.Enabled {
		for i := range accounts {
			a := &accounts[i]
			if !a.Enabled || a.IsBanned || a.CooldownUntil > now {
				continue
			}
			if a.Quotas != nil {
				// 1. Check weekly quota threshold
				if a.Quotas.WeeklyPct <= policy.WeeklyThreshold {
					log.Printf("[SmartShield] 🛡️ Account %s weekly quota is critically low (%.1f%% <= %.1f%%). Setting 24h quarantine.\n",
						a.Email, a.Quotas.WeeklyPct, policy.WeeklyThreshold)
					_ = r.db.SetAccountCooldown(ctx, a.ID, 86400, fmt.Sprintf("Smart Shield: weekly quota low (%.1f%% <= %.1f%%)", a.Quotas.WeeklyPct, policy.WeeklyThreshold))
					a.CooldownUntil = now + (86400 * 1000)
					continue
				}

				// 2. Check 5-hour burst quota threshold
				if a.Quotas.Burst5hPct <= policy.BurstThreshold {
					log.Printf("[SmartShield] 🛡️ Account %s burst quota is low (%.1f%% <= %.1f%%). Setting 30m cooling period.\n",
						a.Email, a.Quotas.Burst5hPct, policy.BurstThreshold)
					_ = r.db.SetAccountCooldown(ctx, a.ID, 1800, fmt.Sprintf("Smart Shield: 5-hour burst quota low (%.1f%% <= %.1f%%)", a.Quotas.Burst5hPct, policy.BurstThreshold))
					a.CooldownUntil = now + (1800 * 1000)
					continue
				}
			}
		}
	}

	var candidates []models.Account
	for _, a := range accounts {
		if !a.Enabled || a.IsBanned {
			continue
		}
		if a.CooldownUntil > now {
			continue
		}
		candidates = append(candidates, a)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("all %s accounts are currently in cooldown or below Smart Shield thresholds", provider)
	}

	// For Google, check if active_session_account_id is preferred and still healthy
	if provider == models.ProviderGoogle {
		var activeID string
		_ = r.db.GetSetting(ctx, "active_session_account_id", &activeID)
		if activeID != "" {
			for _, cand := range candidates {
				if cand.ID == activeID {
					// Verify active session account has adequate quota if Smart Shield enabled
					if policy.Enabled && cand.Quotas != nil && (cand.Quotas.Burst5hPct <= policy.BurstThreshold || cand.Quotas.WeeklyPct <= policy.WeeklyThreshold) {
						log.Printf("[SmartShield] ⚠️ Active session account %s quota is below threshold; rotating to healthiest alternative.\n", cand.Email)
						break
					}
					return &cand, nil
				}
			}
		}
	}

	// Sort candidates by highest remaining quota
	sort.Slice(candidates, func(i, j int) bool {
		q1 := candidates[i].Quotas
		q2 := candidates[j].Quotas
		if q1 == nil || q2 == nil {
			return false
		}
		// Prefer accounts with higher burst and weekly remaining %
		score1 := (q1.Burst5hPct * 0.6) + (q1.WeeklyPct * 0.4)
		score2 := (q2.Burst5hPct * 0.6) + (q2.WeeklyPct * 0.4)
		return score1 > score2
	})

	return &candidates[0], nil
}

// HandleQuotaExceeded marks account into cooldown for the gateway API pool
func (r *Rotator) HandleQuotaExceeded(ctx context.Context, account *models.Account, reason string, cooldownSeconds int) error {
	log.Printf("[SmartShield] ⚠️ Account %s (%s) exceeded quota. Setting pool cooldown (%ds): %s\n",
		account.Email, account.ID, cooldownSeconds, reason)

	if cooldownSeconds <= 0 {
		cooldownSeconds = 300 // default 5m
	}

	return r.db.SetAccountCooldown(ctx, account.ID, cooldownSeconds, reason)
}

// StartQuotaSyncWorker runs background loop updating quotas in PostgreSQL
func (r *Rotator) StartQuotaSyncWorker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		// Run initial check after 5 seconds
		time.Sleep(5 * time.Second)
		r.syncAllQuotas(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.syncAllQuotas(ctx)
			}
		}
	}()
}

func (r *Rotator) syncAllQuotas(ctx context.Context) {
	accounts, err := r.db.GetAccounts(ctx, models.ProviderGoogle)
	if err != nil {
		return
	}

	for _, a := range accounts {
		if !a.Enabled {
			continue
		}
		var creds models.GoogleCredentials
		if err := json.Unmarshal(a.Credentials, &creds); err != nil {
			continue
		}

		weekly, burst5h, claudeWeekly, claude5h, err := r.googleCl.FetchQuotaSummary(ctx, &creds)
		if err == nil {
			_ = r.db.UpdateQuotasAll(ctx, a.ID, weekly, burst5h, claudeWeekly, claude5h, 0, 0)
			// Also update credentials if access token was refreshed
			_ = r.db.UpdateAccountCredentials(ctx, a.ID, creds)
			log.Printf("[SmartShield] ✅ Synced quotas for %s: weekly=%.1f%%, 5h=%.1f%%, claude_weekly=%.1f%%, claude_5h=%.1f%%\n",
				a.Email, weekly, burst5h, claudeWeekly, claude5h)
		} else {
			log.Printf("[SmartShield] ⚠️ Quota fetch failed for %s: %v\n", a.Email, err)
		}
	}

	// Ensure Claude and Codex accounts have valid quota records in account_quotas
	claudeAccounts, _ := r.db.GetAccounts(ctx, models.ProviderClaude)
	for _, ca := range claudeAccounts {
		_ = r.db.UpdateQuotasAll(ctx, ca.ID, 100, 100, 100, 100, 0, 0)
	}

	codexAccounts, _ := r.db.GetAccounts(ctx, models.ProviderCodex)
	for _, cdx := range codexAccounts {
		_ = r.db.UpdateQuotasAll(ctx, cdx.ID, 100, 100, 100, 100, 0, 0)
	}

	// SmartShield 2.0: Evaluate predictive zero-stall handover
	_, _ = r.PredictiveHandoverCheck(ctx)
}

// HandoverEvent stores metadata about zero-stall account switches
type HandoverEvent struct {
	Timestamp        int64   `json:"timestamp"`
	FromAccountEmail string  `json:"from_account_email"`
	ToAccountEmail   string  `json:"to_account_email"`
	Reason           string  `json:"reason"`
	PreviousBurstPct float64 `json:"previous_burst_pct"`
	NewBurstPct      float64 `json:"new_burst_pct"`
	Status           string  `json:"status"`
}

func (r *Rotator) triggerWebhook(ctx context.Context, event *HandoverEvent) {
	var webhookURL string
	_ = r.db.GetSetting(ctx, "smart_shield_webhook_url", &webhookURL)
	if webhookURL == "" {
		return
	}

	payload := map[string]any{
		"content": fmt.Sprintf("⚡ **SmartShield 2.0 Zero-Stall Handover**\n\n**From:** %s (Burst: %.1f%%)\n**To:** %s (Burst: %.1f%%)\n**Reason:** %s",
			event.FromAccountEmail, event.PreviousBurstPct,
			event.ToAccountEmail, event.NewBurstPct,
			event.Reason),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Fire and forget webhook
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
}

func (r *Rotator) PredictiveHandoverCheck(ctx context.Context) (*HandoverEvent, error) {
	policy := r.GetShieldPolicy(ctx)
	if !policy.Enabled || !policy.AutoContinue {
		return nil, nil
	}

	var activeID string
	_ = r.db.GetSetting(ctx, "active_session_account_id", &activeID)
	if activeID == "" {
		return nil, nil
	}

	activeAcc, err := r.db.GetAccountByID(ctx, activeID)
	if err != nil || activeAcc == nil || activeAcc.Quotas == nil {
		return nil, nil
	}

	// 1. Calculate recent token velocity in last 15 minutes
	var recentTokens int64
	_ = r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0)
		FROM request_logs
		WHERE account_id = $1 AND created_at >= NOW() - INTERVAL '15 minutes'
	`, activeID).Scan(&recentTokens)

	// Threshold evaluation: If burst is critically low, or below threshold + rapid burn
	isCriticallyLow := activeAcc.Quotas.Burst5hPct <= policy.BurstThreshold
	isPreemptiveRisk := recentTokens > 150000 && activeAcc.Quotas.Burst5hPct <= (policy.BurstThreshold+10.0)

	if !isCriticallyLow && !isPreemptiveRisk {
		return nil, nil // Current active account is healthy
	}

	// 2. Find the healthiest standby account
	accounts, err := r.db.GetAccounts(ctx, models.ProviderGoogle)
	if err != nil || len(accounts) <= 1 {
		return nil, nil
	}

	now := time.Now().UnixMilli()
	var bestCandidate *models.Account
	var bestScore float64 = -1

	for i := range accounts {
		cand := &accounts[i]
		if cand.ID == activeID || !cand.Enabled || cand.IsBanned || cand.CooldownUntil > now {
			continue
		}
		if cand.Quotas == nil {
			continue
		}
		score := (cand.Quotas.Burst5hPct * 0.7) + (cand.Quotas.WeeklyPct * 0.3)
		if score > bestScore && cand.Quotas.Burst5hPct > policy.BurstThreshold+15 {
			bestScore = score
			bestCandidate = cand
		}
	}

	if bestCandidate == nil {
		log.Printf("[SmartShield 2.0] ⚠️ No healthier standby Google account available for handover (all in cooldown or low quota)\n")
		return nil, nil
	}

	// 3. Execute zero-stall account switch on IDE & Desktop
	var nextCreds models.GoogleCredentials
	if err := json.Unmarshal(bestCandidate.Credentials, &nextCreds); err != nil {
		return nil, err
	}

	reason := fmt.Sprintf("Zero-Stall Handover: 5h burst quota low (%.1f%% <= %.1f%%)", activeAcc.Quotas.Burst5hPct, policy.BurstThreshold)
	if isPreemptiveRisk && !isCriticallyLow {
		reason = fmt.Sprintf("Predictive Handover: High velocity burn (%dk/15m) with burst quota at %.1f%%", recentTokens/1000, activeAcc.Quotas.Burst5hPct)
	}

	err = r.supervisor.ApplyAccountSwitch(&nextCreds, bestCandidate.Email, "both", false)
	if err != nil {
		log.Printf("[SmartShield 2.0] ❌ Failed to execute zero-stall account switch: %v\n", err)
		return nil, err
	}

	// 4. Update active session setting in database
	_ = r.db.SetSetting(ctx, "active_session_account_id", bestCandidate.ID)

	event := &HandoverEvent{
		Timestamp:        time.Now().UnixMilli(),
		FromAccountEmail: activeAcc.Email,
		ToAccountEmail:   bestCandidate.Email,
		Reason:           reason,
		PreviousBurstPct: activeAcc.Quotas.Burst5hPct,
		NewBurstPct:      bestCandidate.Quotas.Burst5hPct,
		Status:           "COMPLETED",
	}

	_ = r.db.SetSetting(ctx, "last_smartshield_handover", event)
	log.Printf("[SmartShield 2.0] ⚡ Zero-Stall Handover Executed! Preemptively switched active session from %s (%.1f%%) to %s (%.1f%%)\n",
		activeAcc.Email, activeAcc.Quotas.Burst5hPct, bestCandidate.Email, bestCandidate.Quotas.Burst5hPct)

	r.triggerWebhook(ctx, event)

	return event, nil
}
