<div align="center">
  <img src="https://raw.githubusercontent.com/lucide-icons/lucide/main/icons/shield-check.svg" width="80" height="80" alt="OmniGate Logo" />
  <h1 align="center">OmniGate</h1>
  <p align="center">
    <strong>The Predictive, Zero-Stall AI Gateway & Fleet Manager</strong>
  </p>
  <p align="center">
    <a href="#features">Features</a> •
    <a href="#architecture">Architecture</a> •
    <a href="#installation">Installation</a> •
    <a href="#dashboard">Dashboard</a>
  </p>
</div>

---

**OmniGate** is an ultra-high-performance, OpenAI-compatible API Gateway designed to manage fleets of AI accounts (Google Cloud Code, Claude, Codex) and provide 100% uptime for intensive IDE sessions and background workloads. 

By utilizing **SmartShield 2.0**, OmniGate monitors your 15-minute token burst velocity and preemptively rotates your active session to a healthy standby account *before* a 429 Quota Exhaustion occurs, ensuring a true **zero-stall** experience.

## ✨ Key Features

- 🛡️ **SmartShield 2.0 Engine**: Predictive zero-stall handover. Automatically swaps active accounts seamlessly across your IDE and Desktop when quotas get dangerously low.
- ⚡ **Sub-100ms Semantic Caching**: SHA-256 hashed prompt interception. Repeated massive codebase contexts are cached in-memory and streamed back instantly at exactly $0 cost.
- 📊 **ROI Analytics Command Center**: A stunning, glassmorphic Next.js dashboard providing real-time views on token burn, 14-day history, fleet health, and commercial API cost savings.
- 🔔 **Telemetry Webhooks**: Instant Slack/Discord notifications for preemptive account handovers, plus daily 9:00 AM automated ROI and token summaries.
- 🔑 **Virtual API Keys (Multi-Tenancy)**: Issue distinct `ag_live_` keys to your team or scripts. Set strict Rate-Per-Minute (RPM) and daily token boundaries per key.
- 🔀 **Universal Compatibility**: Exposes a standard OpenAI `/v1/chat/completions` endpoint, allowing you to use OmniGate as a drop-in replacement for any AI agent or IDE (Cursor, VSCode, etc).

## 🏗️ Architecture Stack

* **Core Gateway**: High-concurrency Go (Golang) daemon using `chi` router and atomic RWMutex state management.
* **Database**: PostgreSQL for persistent telemetry, request logging, and Virtual API Key management.
* **Dashboard**: Next.js 14 (App Router), TailwindCSS, Lucide Icons, featuring a responsive dark-mode glassmorphic design.

## 🚀 Installation & Quick Start

### 1. Prerequisites
- Go 1.21+
- Node.js 18+
- PostgreSQL (Local or Docker)

### 2. Setup the Gateway
```bash
# Clone the repository
git clone https://github.com/yourusername/omnigate.git
cd omnigate/gateway

# Setup environment variables
export DATABASE_URL="postgres://user:pass@127.0.0.1:5432/omnigate?sslmode=disable"
export PORT="8050"

# Build and run the daemon
go build -o ./bin/gateway ./cmd/server
./bin/gateway
```

### 3. Launch the Command Center
```bash
cd ../dashboard

# Install dependencies
npm install

# Start the dashboard in development mode
npm run dev -- -p 3000
```
*Visit `http://localhost:3000` to access the Analytics Dashboard.*

## 📸 Dashboard Preview

*(Add your screenshots here! Highlight the Analytics page, Model Filtering, and the SmartShield configuration topology.)*

## 🛣️ Roadmap

- [x] Predictive Quota Handovers
- [x] Sub-100ms Semantic Caching
- [x] Webhook Telemetry
- [ ] Cross-Provider Transparent Fallback (Google -> Claude -> OpenAI)
- [ ] Redis integration for distributed prompt caching
- [ ] Docker Compose one-click deployment

## 🤝 Contributing

Contributions are always welcome! Feel free to open an Issue if you have feature requests or submit a Pull Request. 

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
