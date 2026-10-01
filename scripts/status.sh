#!/bin/bash

echo "📊 OmniGate Status Check:"
echo "-----------------------------------"

echo -n "PostgreSQL (:5432): "
if nc -z 127.0.0.1 5432 2>/dev/null; then
    echo "🟢 ONLINE"
else
    echo "🔴 OFFLINE"
fi

echo -n "Gateway    (:8050): "
GW_HEALTH=$(curl -s http://localhost:8050/health 2>/dev/null || true)
if [[ -n "$GW_HEALTH" ]]; then
    echo "🟢 ONLINE ($GW_HEALTH)"
else
    echo "🔴 OFFLINE"
fi

echo -n "Dashboard  (:3000): "
DASH_STATUS=$(curl -sI http://localhost:3000/overview 2>/dev/null | head -n 1 || true)
if [[ -n "$DASH_STATUS" ]]; then
    echo "🟢 ONLINE ($DASH_STATUS)"
else
    echo "🔴 OFFLINE"
fi
echo "-----------------------------------"
