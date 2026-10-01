package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antigravity/gateway/pkg/models"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, connString string) (*DB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse db config error: %w", err)
	}

	config.MaxConns = 30
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool error: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db error: %w", err)
	}

	log.Println("[DB] ✅ Connected to PostgreSQL database pool")
	d := &DB{Pool: pool}
	if err := d.InitSchema(ctx); err != nil {
		log.Printf("[DB] ⚠️ Schema initialization notice: %v\n", err)
	}
	return d, nil
}

func (d *DB) InitSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS accounts (
		id VARCHAR(64) PRIMARY KEY,
		provider VARCHAR(32) NOT NULL,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) NOT NULL,
		auth_type VARCHAR(32) NOT NULL,
		credentials JSONB NOT NULL,
		plan_type VARCHAR(32) DEFAULT 'pro',
		enabled BOOLEAN DEFAULT true,
		is_banned BOOLEAN DEFAULT false,
		cooldown_until BIGINT DEFAULT 0,
		cooldown_reason TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS account_quotas (
		account_id VARCHAR(64) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
		weekly_pct DOUBLE PRECISION NOT NULL DEFAULT 100,
		burst_5h_pct DOUBLE PRECISION NOT NULL DEFAULT 100,
		claude_weekly_pct DOUBLE PRECISION NOT NULL DEFAULT 100,
		claude_5h_pct DOUBLE PRECISION NOT NULL DEFAULT 100,
		weekly_reset_at BIGINT DEFAULT 0,
		burst_5h_reset_at BIGINT DEFAULT 0,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS virtual_api_keys (
		id VARCHAR(64) PRIMARY KEY,
		key_hash VARCHAR(128) NOT NULL UNIQUE,
		name VARCHAR(255) NOT NULL,
		prefix VARCHAR(16) NOT NULL,
		rate_limit_rpm INT NOT NULL DEFAULT 60,
		rate_limit_tpm INT NOT NULL DEFAULT 100000,
		expires_at BIGINT NOT NULL DEFAULT 0,
		enabled BOOLEAN NOT NULL DEFAULT true,
		total_requests BIGINT NOT NULL DEFAULT 0,
		total_tokens BIGINT NOT NULL DEFAULT 0,
		last_used_at BIGINT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS request_logs (
		req_id VARCHAR(64) PRIMARY KEY,
		api_key_id VARCHAR(64) REFERENCES virtual_api_keys(id) ON DELETE SET NULL,
		account_id VARCHAR(64) REFERENCES accounts(id) ON DELETE SET NULL,
		provider VARCHAR(32) NOT NULL,
		model VARCHAR(64) NOT NULL,
		prompt_tokens INT DEFAULT 0,
		completion_tokens INT DEFAULT 0,
		duration_ms INT NOT NULL,
		status_code INT NOT NULL,
		error TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_request_logs_model ON request_logs(model);
	CREATE INDEX IF NOT EXISTS idx_request_logs_account_id ON request_logs(account_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_request_logs_req_id ON request_logs(req_id);

	CREATE TABLE IF NOT EXISTS system_settings (
		key VARCHAR(64) PRIMARY KEY,
		value JSONB NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	INSERT INTO system_settings (key, value, updated_at)
	VALUES 
		('smart_shield_enabled', 'true'::jsonb, NOW()),
		('smart_shield_burst_threshold', '15'::jsonb, NOW()),
		('smart_shield_weekly_threshold', '10'::jsonb, NOW()),
		('smart_shield_auto_continue', 'true'::jsonb, NOW())
	ON CONFLICT (key) DO NOTHING;
	`
	_, err := d.Pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("execute init schema error: %w", err)
	}
	log.Println("[DB] ✅ Database schema verified and initialized")
	return nil
}

func (d *DB) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}

// Accounts Operations
func (d *DB) GetAccounts(ctx context.Context, provider string) ([]models.Account, error) {
	query := `
		SELECT a.id, a.provider, a.name, a.email, a.auth_type, a.credentials, 
		       a.plan_type, a.enabled, a.is_banned, a.cooldown_until, a.cooldown_reason, 
		       a.created_at, a.updated_at,
		       COALESCE(q.weekly_pct, 100), COALESCE(q.burst_5h_pct, 100),
		       COALESCE(q.claude_weekly_pct, 100), COALESCE(q.claude_5h_pct, 100),
		       COALESCE(q.weekly_reset_at, 0), COALESCE(q.burst_5h_reset_at, 0)
		FROM accounts a
		LEFT JOIN account_quotas q ON a.id = q.account_id
	`
	var rows pgx.Rows
	var err error

	if provider != "" {
		query += " WHERE a.provider = $1 ORDER BY a.name ASC"
		rows, err = d.Pool.Query(ctx, query, provider)
	} else {
		query += " ORDER BY a.provider, a.name ASC"
		rows, err = d.Pool.Query(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var a models.Account
		var q models.AccountQuotas
		q.AccountID = a.ID

		err := rows.Scan(
			&a.ID, &a.Provider, &a.Name, &a.Email, &a.AuthType, &a.Credentials,
			&a.PlanType, &a.Enabled, &a.IsBanned, &a.CooldownUntil, &a.CooldownReason,
			&a.CreatedAt, &a.UpdatedAt,
			&q.WeeklyPct, &q.Burst5hPct, &q.ClaudeWeeklyPct, &q.Claude5hPct,
			&q.WeeklyResetAt, &q.Burst5hResetAt,
		)
		if err != nil {
			return nil, err
		}
		a.Quotas = &q
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (d *DB) GetAccountByID(ctx context.Context, id string) (*models.Account, error) {
	query := `
		SELECT a.id, a.provider, a.name, a.email, a.auth_type, a.credentials, 
		       a.plan_type, a.enabled, a.is_banned, a.cooldown_until, a.cooldown_reason, 
		       a.created_at, a.updated_at,
		       COALESCE(q.weekly_pct, 100), COALESCE(q.burst_5h_pct, 100),
		       COALESCE(q.claude_weekly_pct, 100), COALESCE(q.claude_5h_pct, 100),
		       COALESCE(q.weekly_reset_at, 0), COALESCE(q.burst_5h_reset_at, 0)
		FROM accounts a
		LEFT JOIN account_quotas q ON a.id = q.account_id
		WHERE a.id = $1
	`
	row := d.Pool.QueryRow(ctx, query, id)
	var a models.Account
	var q models.AccountQuotas
	err := row.Scan(
		&a.ID, &a.Provider, &a.Name, &a.Email, &a.AuthType, &a.Credentials,
		&a.PlanType, &a.Enabled, &a.IsBanned, &a.CooldownUntil, &a.CooldownReason,
		&a.CreatedAt, &a.UpdatedAt,
		&q.WeeklyPct, &q.Burst5hPct, &q.ClaudeWeeklyPct, &q.Claude5hPct,
		&q.WeeklyResetAt, &q.Burst5hResetAt,
	)
	if err != nil {
		return nil, err
	}
	a.Quotas = &q
	return &a, nil
}

func (d *DB) SetAccountCooldown(ctx context.Context, id string, durationSeconds int, reason string) error {
	until := time.Now().UnixMilli() + int64(durationSeconds*1000)
	query := `
		UPDATE accounts 
		SET cooldown_until = $1, cooldown_reason = $2, updated_at = NOW() 
		WHERE id = $3
	`
	_, err := d.Pool.Exec(ctx, query, until, reason, id)
	return err
}

func (d *DB) UpdateAccountCredentials(ctx context.Context, id string, creds any) error {
	bytes, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	query := `UPDATE accounts SET credentials = $1, updated_at = NOW() WHERE id = $2`
	_, err = d.Pool.Exec(ctx, query, bytes, id)
	return err
}

func (d *DB) UpdateQuotasAll(ctx context.Context, accountID string, weekly, burst5h, claudeWeekly, claude5h float64, wReset, bReset int64) error {
	query := `
		INSERT INTO account_quotas (account_id, weekly_pct, burst_5h_pct, claude_weekly_pct, claude_5h_pct, weekly_reset_at, burst_5h_reset_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (account_id) DO UPDATE SET
			weekly_pct = EXCLUDED.weekly_pct,
			burst_5h_pct = EXCLUDED.burst_5h_pct,
			claude_weekly_pct = EXCLUDED.claude_weekly_pct,
			claude_5h_pct = EXCLUDED.claude_5h_pct,
			weekly_reset_at = EXCLUDED.weekly_reset_at,
			burst_5h_reset_at = EXCLUDED.burst_5h_reset_at,
			updated_at = NOW()
	`
	_, err := d.Pool.Exec(ctx, query, accountID, weekly, burst5h, claudeWeekly, claude5h, wReset, bReset)
	return err
}

func (d *DB) UpdateQuotas(ctx context.Context, accountID string, weekly, burst5h float64, wReset, bReset int64) error {
	return d.UpdateQuotasAll(ctx, accountID, weekly, burst5h, 100, 100, wReset, bReset)
}

// Virtual API Keys
func HashKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

func (d *DB) ValidateVirtualKey(ctx context.Context, rawKey string) (*models.VirtualAPIKey, error) {
	keyHash := HashKey(rawKey)
	query := `
		SELECT id, name, prefix, rate_limit_rpm, rate_limit_tpm, expires_at, enabled,
		       total_requests, total_tokens, COALESCE(last_used_at, 0), created_at
		FROM virtual_api_keys
		WHERE key_hash = $1
	`
	var k models.VirtualAPIKey
	row := d.Pool.QueryRow(ctx, query, keyHash)
	err := row.Scan(
		&k.ID, &k.Name, &k.Prefix, &k.RateLimitRPM, &k.RateLimitTPM, &k.ExpiresAt,
		&k.Enabled, &k.TotalRequests, &k.TotalTokens, &k.LastUsedAt, &k.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if !k.Enabled {
		return nil, fmt.Errorf("api key is disabled")
	}
	if k.ExpiresAt > 0 && k.ExpiresAt < time.Now().UnixMilli() {
		return nil, fmt.Errorf("api key has expired")
	}
	return &k, nil
}

func (d *DB) RecordKeyUsage(ctx context.Context, keyID string, tokens int) error {
	now := time.Now().UnixMilli()
	query := `
		UPDATE virtual_api_keys
		SET total_requests = total_requests + 1, total_tokens = total_tokens + $1, last_used_at = $2
		WHERE id = $3
	`
	_, err := d.Pool.Exec(ctx, query, tokens, now, keyID)
	return err
}

func (d *DB) RecordRequestLog(ctx context.Context, reqID, keyID, accountID, provider, model string, pTokens, cTokens, durMs, status int, errText string) error {
	return d.RecordRequestLogWithTime(ctx, reqID, keyID, accountID, provider, model, pTokens, cTokens, durMs, status, errText, time.Now())
}

func (d *DB) RecordRequestLogWithTime(ctx context.Context, reqID, keyID, accountID, provider, model string, pTokens, cTokens, durMs, status int, errText string, createdAt time.Time) error {
	var keyPtr, accPtr *string
	if keyID != "" {
		keyPtr = &keyID
	}
	if accountID != "" {
		accPtr = &accountID
	}
	var errPtr *string
	if errText != "" {
		errPtr = &errText
	}

	query := `
		INSERT INTO request_logs (req_id, api_key_id, account_id, provider, model, prompt_tokens, completion_tokens, duration_ms, status_code, error, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (req_id) DO NOTHING
	`
	_, err := d.Pool.Exec(ctx, query, reqID, keyPtr, accPtr, provider, model, pTokens, cTokens, durMs, status, errPtr, createdAt)
	return err
}

// System Settings
func (d *DB) GetSetting(ctx context.Context, key string, target any) error {
	query := `SELECT value FROM system_settings WHERE key = $1`
	var val json.RawMessage
	err := d.Pool.QueryRow(ctx, query, key).Scan(&val)
	if err != nil {
		return err
	}
	return json.Unmarshal(val, target)
}

func (d *DB) SetSetting(ctx context.Context, key string, val any) error {
	bytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO system_settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`
	_, err = d.Pool.Exec(ctx, query, key, bytes)
	return err
}

func (d *DB) GetAllSettings(ctx context.Context) (map[string]any, error) {
	query := `SELECT key, value FROM system_settings`
	rows, err := d.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]any)
	for rows.Next() {
		var key string
		var raw json.RawMessage
		if err := rows.Scan(&key, &raw); err == nil {
			var parsed any
			if err := json.Unmarshal(raw, &parsed); err == nil {
				settings[key] = parsed
			} else {
				settings[key] = string(raw)
			}
		}
	}
	return settings, nil
}

func (d *DB) DeleteKey(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM virtual_api_keys WHERE id = $1`, id)
	return err
}
