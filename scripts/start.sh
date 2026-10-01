#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "🚀 Starting OmniGate Services..."

# 1. Check PostgreSQL
if ! nc -z 127.0.0.1 5432 2>/dev/null; then
    echo "⚠️  PostgreSQL is not listening on 127.0.0.1:5432. Starting dev-postgres..."
    docker start dev-postgres 2>/dev/null || (cd "$DIR" && docker compose up -d)
fi

# 2. Start Go Gateway
echo "▶️  Launching OmniGate Go Gateway on :8050..."
pkill -f "omnigate/gateway/bin/gateway" 2>/dev/null || true
nohup "$DIR/gateway/bin/gateway" > "$DIR/gateway/gateway.log" 2>&1 &
echo "   Gateway PID: $!"

# 3. Start Next.js Dashboard
echo "▶️  Launching OmniGate Web Console on :3000..."
pkill -f "next-server" 2>/dev/null || true
(
    cd "$DIR/dashboard"
    nohup ./node_modules/.bin/next start -p 3000 > "$DIR/dashboard/dashboard.log" 2>&1 &
)
echo "   Dashboard started in background"

sleep 2
echo "✅ OmniGate services launched!"
echo "   • Console: http://localhost:3000"
echo "   • Gateway: http://localhost:8050"
