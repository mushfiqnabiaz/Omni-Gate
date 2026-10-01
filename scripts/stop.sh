#!/bin/bash

echo "🛑 Stopping OmniGate Services..."

pkill -f "omnigate/gateway/bin/gateway" 2>/dev/null && echo "   • Gateway stopped" || echo "   • Gateway was not running"
pkill -f "next-server" 2>/dev/null && echo "   • Dashboard stopped" || echo "   • Dashboard was not running"

echo "✅ OmniGate services stopped."
