-- OmniGate PostgreSQL 17 Database Schema
-- Auto-executed on gateway boot and available for container bootstrapping

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

CREATE TABLE IF NOT EXISTS system_settings (
    key VARCHAR(64) PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Default system settings
INSERT INTO system_settings (key, value, updated_at)
VALUES 
    ('smart_shield_enabled', 'true'::jsonb, NOW()),
    ('smart_shield_burst_threshold', '15'::jsonb, NOW()),
    ('smart_shield_weekly_threshold', '10'::jsonb, NOW()),
    ('smart_shield_auto_continue', 'true'::jsonb, NOW())
ON CONFLICT (key) DO NOTHING;
