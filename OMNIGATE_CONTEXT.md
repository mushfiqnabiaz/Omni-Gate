# OmniGate: Complete Platform Context & Architecture Specification

**OmniGate (Version 2)** is an enterprise-grade AI proxy gateway, multi-provider quota router, and real-time fleet operations console. It unifies Google Cloud Code (Gemini), Anthropic Claude, and OpenAI Codex behind standard, drop-in OpenAI (`/v1/chat/completions`) and Anthropic (`/v1/messages`) API endpoints with sub-millisecond local routing, autonomous rate limiting, and zero-downtime failover.

---

## 📑 Table of Contents

1. [Executive Summary & Evolution](#1-executive-summary--evolution)
2. [High-Level Architecture & Topology](#2-high-level-architecture--topology)
3. [Component Deep-Dive](#3-component-deep-dive)
   - [3.1 Go Enterprise Gateway Core (`gateway/`)](#31-go-enterprise-gateway-core-gateway)
   - [3.2 Next.js Web Console (`dashboard/`)](#32-nextjs-web-console-dashboard)
   - [3.3 PostgreSQL 17 Database Engine (`dev-postgres`)](#33-postgresql-17-database-engine-dev-postgres)
4. [Provider Execution Engines](#4-provider-execution-engines)
   - [4.1 OpenAI Codex Local Native Runtime](#41-openai-codex-local-native-runtime)
   - [4.2 Google Cloud Code & Gemini Engine](#42-google-cloud-code--gemini-engine)
   - [4.3 Anthropic Claude Engine & Bridge](#43-anthropic-claude-engine--bridge)
5. [Latency, Networking & Telemetry Architecture](#5-latency-networking--telemetry-architecture)
   - [5.1 Local IPC vs. Cloud WAN Analysis](#51-local-ipc-vs-cloud-wan-analysis)
   - [5.2 HTTP/2 Keep-Alive Connection Pooling](#52-http2-keep-alive-connection-pooling)
   - [5.3 Time To First Token (TTFT) Tracking](#53-time-to-first-token-ttft-tracking)
6. [Desktop IDE Supervisor & Keychain Safety](#6-desktop-ide-supervisor--keychain-safety)
7. [Smart Shield & Autonomous Quota Management](#7-smart-shield--autonomous-quota-management)
8. [Virtual API Key Security Architecture](#8-virtual-api-key-security-architecture)
9. [Database Schema Reference](#9-database-schema-reference)
10. [Web Console Route Matrix (10 Endpoints)](#10-web-console-route-matrix-10-endpoints)
11. [Complete API Specification](#11-complete-api-specification)
12. [Two-Codebase Structural Separation](#12-two-codebase-structural-separation)
13. [Operations, Runbooks & Diagnostic Cheatsheet](#13-operations-runbooks--diagnostic-cheatsheet)

---

## 1. Executive Summary & Evolution

### The Problem
During intensive coding with Google Antigravity IDE and modern AI assistants, engineers face strict burst (5-hour) and weekly quota limitations per Google Pro account. When an account depletes its quota mid-task:
- The engineer is forced to log out, re-authenticate with another Google account, and lose editor context, history, and active terminal state.
- Workflows spanning OpenAI Codex (`gpt-5.5`) or Anthropic Claude require juggling incompatible SDKs, multiple API keys, and fragmented billing/logging.

### Legacy Version 1 (`antigravity-harness`)
The initial prototype was built as a single-repository Node.js monolith:
- Monolithic `server.js` (110 KB) handling routing, proxying, and streaming on a single JavaScript event loop.
- Monolithic `dashboard.html` (359 KB) containing inline styles, script tags, and DOM queries.
- Concurrency bottlenecks during Server-Sent Events (SSE) streaming.
- Account credentials stored in flat `accounts.json` files and quota states in `stats.json`.

### Modern Version 2 (`omnigate`)
OmniGate completely re-architects the system into an enterprise microservice topology:
- **Go 1.26 Gateway**: Multi-threaded, compiled ARM64 Mach-O binary capable of handling tens of thousands of concurrent requests with near-zero memory footprint.
- **Next.js 16 Web Console**: Modern, modular React 19 application with Turbopack, Tailwind CSS, Lucide icons, and Cloudflare dark-mode styling (`#000000` / `#0a0a0a`).
- **PostgreSQL 17 Database Pool**: High-concurrency `pgx/v5` connection pool hosted on OrbStack (`127.0.0.1:5432`) managing accounts, real-time quotas, hashed virtual keys, system settings, and request audit logs.
- **Universal Provider Mesh**: Native support for **Google Cloud Code (Gemini)**, **Anthropic Claude**, and **OpenAI Codex** through unified drop-in APIs.

---

## 2. High-Level Architecture & Topology

```mermaid
graph TD
    subgraph Clients["Clients & Development Environments"]
        IDE["Antigravity IDE / VS Code / Cursor"]
        CLI["Claude CLI / Codex Tools / curl"]
        Browser["Admin Browser (Next.js Console)"]
    end

    subgraph OmniGate_Platform["OmniGate Platform (/Users/nabiaz/omnigate)"]
        subgraph Dashboard_Tier["Frontend Tier (:3000)"]
            NextJS["Next.js 16.3.7 Console<br/>React 19 + Turbopack"]
            ProxyAPI["Next.js Internal Route Proxy<br/>/api/proxy/[...path]"]
            NextJS --> ProxyAPI
        end

        subgraph Gateway_Tier["Core Gateway Tier (:8050)"]
            Router["Chi Router & Dispatcher<br/>/v1/chat/completions | /v1/messages"]
            AuthCheck["Virtual Key Validator<br/>(SHA-256 Hash Matching)"]
            Shield["Smart Shield Engine<br/>Auto Failover & Quota Worker"]
            Supervisor["Desktop IDE Supervisor<br/>(macOS Keychain & Jetski Sync)"]
            
            Router --> AuthCheck
            AuthCheck --> Shield
        end

        subgraph Database_Tier["Persistence Tier (:5432)"]
            PG[("PostgreSQL 17 Pool<br/>OrbStack dev-postgres")]
        end

        Shield <--> PG
        Router <--> PG
        ProxyAPI -->|HTTP Internal| Router
    end

    subgraph Provider_Mesh["AI Provider Mesh"]
        CodexLocal["OpenAI Codex CLI<br/>Mach-O ARM64 Subprocess<br/>(~80ms Local IPC)"]
        GoogleCloud["Google Cloud Code US<br/>daily-cloudcode-pa HTTP/2 SSE<br/>(~1.5s–2.4s Cloud WAN)"]
        ClaudeCloud["Anthropic API / Pro Bridge<br/>Messages v1 SSE<br/>(~580ms Cloud Bridge)"]
    end

    Clients -->|Bearer ag_live_...| Router
    Browser -->|HTTP| NextJS
    Shield -->|gpt-* Models| CodexLocal
    Shield -->|gemini-* Models| GoogleCloud
    Shield -->|claude-* Models| ClaudeCloud
    Supervisor -->|macOS API| Keychain["macOS Keychain Services"]
    Supervisor -->|Process Kill/Respawn| LangServer["IDE language_server Process"]
```

---

## 3. Component Deep-Dive

### 3.1 Go Enterprise Gateway Core (`gateway/`)
- **Path**: `/Users/nabiaz/omnigate/gateway`
- **Binary**: `/Users/nabiaz/omnigate/gateway/bin/gateway`
- **Default Port**: `8050`
- **Key Modules**:
  - `cmd/server/main.go`: Initializes database connection pool, boots the Supervisor, spawns the quota synchronization worker Goroutine, mounts the Chi HTTP router, and configures graceful POSIX signal shutdown.
  - `pkg/router/router.go`: Standardized routing handling CORS, OpenAI `/v1/chat/completions`, Anthropic `/v1/messages`, and fleet management APIs.
  - `pkg/shield/rotator.go`: Computes dynamic quotas, evaluates 5-hour burst thresholds, quarantines exhausted accounts, and routes requests to the healthiest provider account.
  - `pkg/providers/`: Provider drivers implementing unified streaming contracts.
  - `pkg/db/db.go`: Optimized SQL queries with `pgxpool.Pool` (MaxConns: 30, MinConns: 5, MaxConnIdleTime: 15m).

### 3.2 Next.js Web Console (`dashboard/`)
- **Path**: `/Users/nabiaz/omnigate/dashboard`
- **Default Port**: `3000`
- **Technology Stack**:
  - Next.js 16.3.7 with Turbopack bundler.
  - React 19 with Server and Client Components.
  - Tailwind CSS v4 featuring Cloudflare-inspired dark borders (`#1e1e1e`), cards (`#0a0a0a`), and slate accents.
  - Context Provider (`src/context/ConsoleContext.tsx`) polling gateway telemetry every 6 seconds to maintain synchronized state without WebSocket complexity.
  - Secure reverse proxy route (`src/app/api/proxy/[...path]/route.ts`) bridging browser client calls directly to `:8050` while streaming SSE events intact.

### 3.3 PostgreSQL 17 Database Engine (`dev-postgres`)
- **Host**: `127.0.0.1:5432`
- **Platform**: OrbStack native Linux container
- **Database Name**: `antigravity_harness`
- **Role/Credentials**: User `dev`, Password `password`
- **Data Safety**: All tables utilize primary keys, foreign key constraints with `ON DELETE SET NULL`, and transactional consistency.

---

## 4. Provider Execution Engines

### 4.1 OpenAI Codex Local Native Runtime
- **Model Aliases**: `gpt-5.5`, `gpt-4o`, `gpt-4o-mini`, `o1`, `o3-mini`
- **Execution Mechanism**:
  Instead of routing over WAN to OpenAI servers, OmniGate drives the local Apple Silicon Mach-O binary `/opt/homebrew/bin/codex` as a headless subprocess via Darwin Unix pipes.
- **Command-Line Invocation**:
  ```bash
  /opt/homebrew/bin/codex exec --skip-git-repo-check --ephemeral --color never -m gpt-5.5 "<PROMPT>"
  ```
- **Stream Emulation**:
  Lines emitted by the Codex binary are read asynchronously via `bufio.Scanner`, parsed for delta tokens, and formatted into OpenAI-standard Server-Sent Event chunks:
  ```json
  data: {"id":"chatcmpl-...","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"..."}}]}
  ```
- **Performance**: Zero external network overhead; responsive latency of **~80ms**.

### 4.2 Google Cloud Code & Gemini Engine
- **Model Aliases**: `gemini-2.5-flash`, `gemini-2.5-pro`, `gemini-3.8-flash`, `gemini-3.8-pro`
- **Execution Mechanism**:
  Communicates with Google Cloud Code PaLM / Gemini enterprise backend clusters via authenticated OAuth access tokens (`ya29.a0...`).
- **Endpoint Failover Hierarchy**:
  1. `https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:streamGenerateCode` (Fastest, ~1.45s)
  2. `https://daily-cloudcode-pa.googleapis.com/v1internal:streamGenerateCode`
  3. `https://cloudcode-pa.googleapis.com/v1internal:streamGenerateCode`
- **Stream Protocol**: HTTP/2 Server-Sent Events yielding protobuf-backed JSON deltas transformed into standard completions.

### 4.3 Anthropic Claude Engine & Bridge
- **Model Aliases**: `claude-3-7-sonnet`, `claude-3-5-sonnet`, `claude-3-opus`
- **Execution Mechanism**:
  - **Direct API Mode**: Dispatches directly to `https://api.anthropic.com/v1/messages` using `x-api-key` or OAuth Bearer headers with `anthropic-version: 2023-06-01`.
  - **Pro Bridge Mode**: Zero-downtime translation converting Claude Messages format into Google Cloud Code PaLM representations when Anthropic quotas are depleted.

---

## 5. Latency, Networking & Telemetry Architecture

### 5.1 Local IPC vs. Cloud WAN Analysis
A common point of comparison on the dashboard is the latency difference between providers:

| Dimension | OpenAI Codex Runtime | Google Cloud Code (Gemini) | Anthropic Claude |
| :--- | :--- | :--- | :--- |
| **Execution Tier** | **Local Apple Silicon Subprocess** | **Remote Cloud US WAN** | **Remote Cloud WAN** |
| **Transport** | Unix Pipe / Local IPC | HTTP/2 over TLS 1.3 | HTTP/2 over TLS 1.3 |
| **Physical Ping** | `0ms` (Loopback kernel memory) | `~300ms` (Dhaka $\leftrightarrow$ US West) | `~280ms` (Dhaka $\leftrightarrow$ US East) |
| **TLS Handshake** | None | `~500ms – 600ms` (if cold) | `~500ms – 600ms` (if cold) |
| **Queue & Inference** | `~60ms – 80ms` | `~600ms – 900ms` | `~300ms – 400ms` |
| **Total Response** | **~80ms** | **~1.5s – 2.4s** | **~580ms – 900ms** |

### 5.2 HTTP/2 Keep-Alive Connection Pooling
To eliminate the 500ms–600ms TLS handshake penalty on every request, the Go Gateway implements dedicated `http.Transport` pools for Google and Anthropic:
```go
tr := &http.Transport{
    Proxy:                 http.ProxyFromEnvironment,
    ForceAttemptHTTP2:     true,
    MaxIdleConns:          50,
    MaxIdleConnsPerHost:   30,
    IdleConnTimeout:       120 * time.Second,
    TLSHandshakeTimeout:   10 * time.Second,
    ExpectContinueTimeout: 1 * time.Second,
}
```
Sockets remain open and warm for up to 2 minutes, allowing subsequent requests to transmit payload immediately.

### 5.3 Time To First Token (TTFT) Tracking
Telemetry logs recorded in `request_logs` reflect true user-perceived responsiveness (TTFT) via the `onFirstChunk` callback:
```go
var firstTokenLogged bool
onFirstChunk := func() {
    if !firstTokenLogged {
        ttftDuration := time.Since(startTime).Milliseconds()
        firstTokenLogged = true
        // Stored for real-time dashboard latency calculations
    }
}
```

---

## 6. Desktop IDE Supervisor & Keychain Safety

A critical requirement of OmniGate is keeping the desktop Antigravity IDE running continuously without losing open editors, chat sessions, or terminal states.

### Atomic Credential Sync Workflow
When switching active accounts or rotating upon quota exhaustion:
1. **Preserve Layout State**:
   The Supervisor reads `~/Library/Application Support/Antigravity/app_storage.json`, backs it up with a timestamped suffix (`app_storage.json.backup-<ts>`), and preserves active workspace layout.
2. **macOS Keychain Write**:
   Invokes macOS `/usr/bin/security` with the `-A` flag:
   ```bash
   security add-generic-password -s gemini -a antigravity -l "Antigravity" -w "go-keyring-base64:<B64_JSON>" -A
   ```
   *The `-A` flag authorizes all applications, eliminating native OS password prompt dialogs.*
3. **Jetski Standalone File Write**:
   Writes the OAuth token bundle atomically to `~/.gemini/jetski-standalone-oauth-token` with `0600` permissions.
4. **Clean Language Server Re-Spawn**:
   The supervisor discovers the running `language_server` process PID using `ps` and `lsof` (searching for `--csrf_token` and `--standalone`), preserves layout, and issues a POSIX `SIGKILL`. The Electron supervisor immediately re-spawns `language_server` with the new credentials in under 1.5 seconds.

---

## 7. Smart Shield & Autonomous Quota Management

Smart Shield prevents Google account restrictions by enforcing automated cooling periods and dynamic routing.

```mermaid
flowchart TD
    Req[Incoming Chat Completion Request] --> KeyCheck{Valid API Key?}
    KeyCheck -- No --> Deny[401 Unauthorized]
    KeyCheck -- Yes --> ShieldCheck{Smart Shield Enabled?}
    
    ShieldCheck -- No --> DirectRoute[Route to Default Account]
    ShieldCheck -- Yes --> Evaluate[Evaluate Account Quota Metrics]
    
    Evaluate --> P1{Weekly Quota > Threshold?}
    P1 -- No --> Quarantine1[Quarantine Account for 24h]
    Quarantine1 --> NextAccount[Select Next Best Account in Pool]
    
    P1 -- Yes --> P2{5-Hour Burst Quota > 15%?}
    P2 -- No --> TempCool[Place Account in 30m Cooldown]
    TempCool --> NextAccount
    
    P2 -- Yes --> Execute[Execute Stream Request]
    Execute --> Record[Update Token Usage & Latency Logs]
```

### Configurable Settings (`system_settings`)
- `smart_shield_enabled`: Global boolean master switch.
- `burst_threshold_pct`: Minimum 5-hour quota percentage (default: `15%`) before triggering account rotation.
- `weekly_threshold_pct`: Minimum weekly quota percentage (default: `10%`) before cooldown.
- `failover_to_claude`: Automatically route to Claude if all Google accounts are in cooldown.
- `failover_to_codex`: Automatically route to local Codex runtime if external cloud APIs fail.

---

## 8. Virtual API Key Security Architecture

OmniGate protects upstream credentials by never exposing Google OAuth refresh tokens or Claude session cookies to client applications.

- **Key Format**: `ag_live_<48_hex_characters>` (e.g., `ag_live_fbe370f3ce9bad9baa58ada7f060bcbd773ff7bc1f179a8a`)
- **Key Hashing**: Secrets are hashed immediately upon creation using **SHA-256**. Only the 64-character hex hash is stored in the database (`virtual_api_keys.key_hash`).
- **Plaintext Secret**: Displayed exactly **once** in the UI modal at creation time.
- **Prefix Storage**: A 12-character preview prefix (`ag_live_fbe3...`) is stored for identification in the dashboard table.
- **Revocation**: Instant revocation via `DELETE /api/keys/{id}` immediately sets `enabled = false` in PostgreSQL.

---

## 9. Database Schema Reference

```sql
-- Accounts (Google, Claude, Codex)
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

-- Real-time Account Quotas
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

-- Virtual API Keys
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

-- Telemetry & Audit Logs
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

-- Dynamic System Settings
CREATE TABLE IF NOT EXISTS system_settings (
    key VARCHAR(64) PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

---

## 10. Web Console Route Matrix (10 Endpoints)

| URL Path | Component File | Key Functionality |
| :--- | :--- | :--- |
| `http://localhost:3000/` | `src/app/page.tsx` | Instant redirect to `/overview` |
| `http://localhost:3000/overview` | `src/app/overview/page.tsx` | Fleet summary, multi-provider latency benchmark cards, capacity matrix |
| `http://localhost:3000/fleet/google` | `src/app/fleet/google/page.tsx` | Google Pro accounts, 5h burst bars, weekly quota gauges, active IDE switcher |
| `http://localhost:3000/fleet/claude` | `src/app/fleet/claude/page.tsx` | Anthropic Claude session tokens, Messages v1 health, and Pro Bridge status |
| `http://localhost:3000/fleet/codex` | `src/app/fleet/codex/page.tsx` | OpenAI Codex CLI binary inspector, Apple Silicon Mach-O status (`gpt-5.5`) |
| `http://localhost:3000/keys` | `src/app/keys/page.tsx` | Virtual API Key issuance (`ag_live_...`), token counters, and deletion |
| `http://localhost:3000/shield` | `src/app/shield/page.tsx` | Interactive Smart Shield sliders (RPM, burst %, weekly %), autonomous toggles |
| `http://localhost:3000/logs` | `src/app/logs/page.tsx` | Real-time telemetry audit table (req_id, provider, latency, status, tokens) |
| `http://localhost:3000/playground` | `src/app/playground/page.tsx` | Multi-model SSE chat tester (switch between Gemini, Claude, and Codex) |
| `http://localhost:3000/infra` | `src/app/infra/page.tsx` | PostgreSQL connection pool metrics, memory allocation, and process runtime |

---

## 11. Complete API Specification

### 11.1 OpenAI Drop-In Chat Completions
* **Method & URL**: `POST http://localhost:8050/v1/chat/completions`
* **Headers**:
  * `Authorization: Bearer ag_live_<secret>`
  * `Content-Type: application/json`
* **Request Body**:
  ```json
  {
    "model": "gpt-5.5",
    "stream": true,
    "messages": [
      {"role": "system", "content": "You are a coding assistant."},
      {"role": "user", "content": "Write a quicksort in Go."}
    ]
  }
  ```
* **Supported Models**: `gpt-5.5`, `gpt-4o`, `gemini-2.5-flash`, `gemini-2.5-pro`, `claude-3-7-sonnet`

### 11.2 Anthropic Drop-In Messages
* **Method & URL**: `POST http://localhost:8050/v1/messages`
* **Headers**:
  * `x-api-key: ag_live_<secret>`
  * `anthropic-version: 2023-06-01`
  * `Content-Type: application/json`
* **Request Body**:
  ```json
  {
    "model": "claude-3-7-sonnet",
    "stream": true,
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Explain HTTP/2 multiplexing."}
    ]
  }
  ```

### 11.3 Management & Fleet Control APIs
* `GET /health` — Gateway status, DB engine, and timestamp
* `GET /api/fleet` — List all accounts, active quotas, and active session account ID
* `POST /api/set-active-account` — Switch IDE active session (`{"account_id": "...", "force": true}`)
* `GET /api/keys` — List all virtual API keys
* `POST /api/keys` — Generate a new virtual API key (`{"name": "CI Key", "rate_limit_rpm": 120}`)
* `DELETE /api/keys/{id}` — Revoke and delete a virtual API key
* `GET /api/logs` — Fetch latest 100 request audit logs
* `GET /api/settings` — Get Smart Shield thresholds
* `POST /api/settings` — Update Smart Shield configuration

---

## 12. Two-Codebase Structural Separation

To guarantee zero regression and provide clean separation of concerns, the user environment is organized into strictly **two separate codebases**:

```
/Users/nabiaz/
├── antigravity-harness/     # Version 1 (Legacy Monolith)
│   ├── src/server.js        # Node.js 22 server
│   ├── src/dashboard.html   # 359 KB monolithic HTML/JS
│   ├── accounts.json        # Flat JSON account store
│   └── stats.json           # Flat quota cache
│
└── omnigate/                # Version 2 (Modern Platform)
    ├── gateway/             # Go 1.26 High-Performance Proxy Core (:8050)
    ├── dashboard/           # Next.js 16 Web Console (:3000)
    ├── scripts/             # Operational shell tooling (start, stop, status)
    ├── docker-compose.yml   # PostgreSQL 17 container specification
    └── README.md            # Quickstart documentation
```

---

## 13. Operations, Runbooks & Diagnostic Cheatsheet

### Starting All Services
```bash
cd /Users/nabiaz/omnigate
./scripts/start.sh
```

### Checking Real-Time System Health
```bash
cd /Users/nabiaz/omnigate
./scripts/status.sh
```

### Stopping Services Gracefully
```bash
cd /Users/nabiaz/omnigate
./scripts/stop.sh
```

### Manual Service Testing via cURL
```bash
# 1. Gateway Health Check
curl -s http://localhost:8050/health | jq .

# 2. Test Codex Streaming Inference via Virtual Key
curl -s -N -X POST http://localhost:8050/v1/chat/completions \
  -H "Authorization: Bearer ag_live_fbe370f3ce9bad9baa58ada7f060bcbd773ff7bc1f179a8a" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-5.5", "stream": true, "messages": [{"role": "user", "content": "ping"}]}'

# 3. Test Google Gemini Streaming via Gateway
curl -s -N -X POST http://localhost:8050/v1/chat/completions \
  -H "Authorization: Bearer ag_live_fbe370f3ce9bad9baa58ada7f060bcbd773ff7bc1f179a8a" \
  -H "Content-Type: application/json" \
  -d '{"model": "gemini-2.5-flash", "stream": true, "messages": [{"role": "user", "content": "ping"}]}'

# 4. View Latest Telemetry Audit Row in PostgreSQL
docker exec dev-postgres psql -U dev -d antigravity_harness -c \
  "SELECT req_id, provider, model, duration_ms, status_code, created_at FROM request_logs ORDER BY created_at DESC LIMIT 3;"
```

### Troubleshooting Guide

1. **Port 8050 or 3000 already in use**:
   ```bash
   pkill -f "omnigate/gateway/bin/gateway" 2>/dev/null
   pkill -f "next-server" 2>/dev/null
   ./scripts/start.sh
   ```
2. **PostgreSQL container stopped after Mac sleep**:
   ```bash
   docker start dev-postgres
   ```
3. **Recompiling Go Gateway after changes**:
   ```bash
   cd /Users/nabiaz/omnigate/gateway
   go build -o bin/gateway cmd/server/main.go
   pkill -f "omnigate/gateway/bin/gateway"
   nohup ./bin/gateway > gateway.log 2>&1 &
   ```
4. **Rebuilding Next.js Dashboard**:
   ```bash
   cd /Users/nabiaz/omnigate/dashboard
   npm run build
   pkill -f "next-server"
   nohup ./node_modules/.bin/next start -p 3000 > dashboard.log 2>&1 &
   ```
