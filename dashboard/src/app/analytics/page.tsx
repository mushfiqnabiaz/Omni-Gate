"use client";

import React, { useState, useEffect, useMemo, useRef } from "react";
import {
  TrendingUp,
  DollarSign,
  Zap,
  ShieldCheck,
  Clock,
  Download,
  RefreshCw,
  ArrowUpRight,
  ArrowDownRight,
  Activity,
  Layers,
  Sparkles,
  AlertTriangle,
  CheckCircle2,
  Cpu,
  BarChart2,
  Calendar,
  ChevronRight,
  Sliders,
  ExternalLink,
  Flame,
  FileSpreadsheet,
  FileJson,
} from "lucide-react";
import { useConsole } from "@/context/ConsoleContext";

interface HourlyPoint {
  timestamp: number;
  hour_label: string;
  full_label: string;
  calls: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  commercial_cost_usd: number;
  antigravity_cost_usd: number;
  savings_usd: number;
}

interface DailyPoint {
  date: string;
  display_date: string;
  day_of_week: string;
  calls: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  commercial_cost_usd: number;
  antigravity_cost_usd: number;
  savings_usd: number;
}

interface AccountTimer {
  account_id: string;
  name: string;
  email: string;
  enabled: boolean;
  is_banned: boolean;
  is_active: boolean;
  weekly_pct: number;
  burst_5h_pct: number;
  seconds_until_reset: number;
  next_reset_time: string;
  health_status: string;
}

interface ModelBreakdownItem {
  model: string;
  name: string;
  family: string;
  calls: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  commercial_cost_usd: number;
}

interface AnalyticsData {
  summary: {
    total_calls: number;
    total_tokens: number;
    total_prompt_tokens: number;
    total_completion_tokens: number;
    commercial_savings_usd: number;
    antigravity_pro_cost_usd: number;
    net_savings_usd: number;
    today_calls: number;
    today_tokens: number;
    today_savings_usd: number;
    last_24h_calls: number;
    last_24h_tokens: number;
    last_24h_savings_usd: number;
    burn_rate_tokens_per_hour: number;
    projected_monthly_tokens: number;
    projected_monthly_savings: number;
    annual_projected_roi_usd: number;
  };
  hourly_timeline: HourlyPoint[];
  daily_timeline: DailyPoint[];
  account_timers: AccountTimer[];
  model_breakdown: ModelBreakdownItem[];
  smart_shield: {
    enabled: boolean;
    burst_threshold_pct: number;
    weekly_threshold_pct: number;
    auto_continue: boolean;
    active_account_id: string;
    active_account_email: string;
    active_account_burst_pct: number;
    recent_15m_tokens: number;
    risk_level: string;
    last_handover?: {
      timestamp: number;
      from_account_email: string;
      to_account_email: string;
      reason: string;
      previous_burst_pct: number;
      new_burst_pct: number;
      status: string;
    } | null;
  };
}

export default function AnalyticsPage() {
  const { accounts, setToast, handleSwitchAccount, switchingId } = useConsole();
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [evaluating, setEvaluating] = useState(false);
  const [hoveredHour, setHoveredHour] = useState<HourlyPoint | null>(null);
  const [hoveredDay, setHoveredDay] = useState<DailyPoint | null>(null);
  const [hourlyMetric, setHourlyMetric] = useState<"tokens" | "calls" | "cost">("tokens");
  const [exportMenuOpen, setExportMenuOpen] = useState(false);
  const [modelFilter, setModelFilter] = useState("all");

  // Local seconds ticking for rolling reset countdowns
  const [accountSeconds, setAccountSeconds] = useState<{ [id: string]: number }>({});

  const fetchData = async (silent = false) => {
    if (!silent) setIsRefreshing(true);
    try {
      const res = await fetch(`/api/proxy/api/analytics?model=${modelFilter}`, { cache: "no-store" });
      if (!res.ok) throw new Error("Failed to load analytics");
      const json: AnalyticsData = await res.json();
      setData(json);

      // Seed local timer counters
      const secMap: { [id: string]: number } = {};
      json.account_timers?.forEach((a) => {
        secMap[a.account_id] = a.seconds_until_reset;
      });
      setAccountSeconds(secMap);
    } catch (err: any) {
      console.error(err);
      if (!silent) setToast({ text: "Failed to refresh telemetry", type: "error" });
    } finally {
      setLoading(false);
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [modelFilter]);

  // Auto-refresh interval (10s)
  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(() => {
      fetchData(true);
    }, 10000);
    return () => clearInterval(interval);
  }, [autoRefresh, modelFilter]);

  // Second-by-second countdown ticker for rolling resets
  useEffect(() => {
    const timer = setInterval(() => {
      setAccountSeconds((prev) => {
        const next = { ...prev };
        for (const id in next) {
          if (next[id] > 0) {
            next[id] -= 1;
          } else {
            next[id] = 18000; // loop back to 5h
          }
        }
        return next;
      });
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  const formatSeconds = (sec: number) => {
    const h = Math.floor(sec / 3600);
    const m = Math.floor((sec % 3600) / 60);
    const s = sec % 60;
    return `${h.toString().padStart(2, "0")}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  };

  const handleEvaluateShield = async () => {
    setEvaluating(true);
    try {
      const res = await fetch("/api/proxy/api/shield/evaluate", { method: "POST" });
      const json = await res.json();
      if (json.triggered) {
        setToast({
          text: `⚡ SmartShield Handover Triggered! Switched to ${json.event.to_account_email}`,
          type: "success",
        });
        fetchData();
      } else {
        setToast({
          text: json.message || "Quotas nominal. No handover required.",
          type: "success",
        });
      }
    } catch (err: any) {
      setToast({ text: "Failed to evaluate SmartShield", type: "error" });
    } finally {
      setEvaluating(false);
    }
  };

  const handleToggleAccount = async (accountId: string, currentlyEnabled: boolean) => {
    const newEnabled = !currentlyEnabled;
    try {
      const res = await fetch("/api/proxy/api/account/toggle", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ account_id: accountId, enabled: newEnabled }),
      });
      if (res.ok) {
        setToast({ text: `Account ${newEnabled ? "enabled" : "paused"} successfully`, type: "success" });
        fetchData();
      } else {
        setToast({ text: "Failed to toggle account", type: "error" });
      }
    } catch (err) {
      setToast({ text: "Network error", type: "error" });
    }
  };

  const handleExport = (format: "csv" | "json") => {
    setExportMenuOpen(false);
    window.location.href = `/api/proxy/api/analytics/export?format=${format}&limit=5000`;
    setToast({
      text: `Exporting telemetry as ${format.toUpperCase()}...`,
      type: "success",
    });
  };

  // Calculations for 24h SVG bar chart
  const hourlyData = data?.hourly_timeline || [];
  const maxHourlyVal = useMemo(() => {
    if (!hourlyData.length) return 1;
    if (hourlyMetric === "tokens") {
      const m = Math.max(...hourlyData.map((d) => d.total_tokens));
      return m > 0 ? m : 1000;
    } else if (hourlyMetric === "calls") {
      const m = Math.max(...hourlyData.map((d) => d.calls));
      return m > 0 ? m : 10;
    } else {
      const m = Math.max(...hourlyData.map((d) => d.commercial_cost_usd));
      return m > 0 ? m : 0.1;
    }
  }, [hourlyData, hourlyMetric]);

  // Calculations for 14-day SVG chart
  const dailyData = data?.daily_timeline || [];
  const maxDailyVal = useMemo(() => {
    if (!dailyData.length) return 1;
    const m = Math.max(...dailyData.map((d) => d.total_tokens));
    return m > 0 ? m : 100000;
  }, [dailyData]);

  if (loading && !data) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center min-h-[500px]">
        <div className="flex items-center gap-3 text-sm text-[#888888] font-mono">
          <RefreshCw className="w-4 h-4 animate-spin text-[#0070f3]" />
          Aggregating telemetry, token burn velocity, and ROI metrics...
        </div>
      </div>
    );
  }

  const s = data?.summary;
  const shield = data?.smart_shield;

  return (
    <div className="flex-1 flex flex-col p-6 space-y-6 max-w-7xl mx-auto w-full">
      {/* 1. HEADER & CONTROL BAR */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-[#1e1e1e] pb-5">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-xl font-bold tracking-tight text-[#ededed] flex items-center gap-2">
              <BarChart2 className="w-5 h-5 text-[#0070f3]" />
              Analytics &amp; ROI Command Center
            </h1>
            <span className="px-2 py-0.5 rounded text-[10px] font-mono uppercase font-semibold bg-[#0070f3]/10 text-[#0070f3] border border-[#0070f3]/30">
              SmartShield 2.0 Live
            </span>
          </div>
          <p className="text-xs text-[#888888] mt-1">
            Real-time token burn velocity, predictive zero-stall failover automation, rolling quota countdowns, and commercial ROI runway.
          </p>
        </div>

        <div className="flex items-center gap-2.5 flex-wrap">
          {/* Model Filter Dropdown */}
          <select
            value={modelFilter}
            onChange={(e) => setModelFilter(e.target.value)}
            className="px-2 py-1.5 rounded bg-[#111111] text-[#ededed] text-xs font-mono border border-[#333333] transition hover:border-[#555555] outline-none cursor-pointer"
          >
            <option value="all">All Models</option>
            <option value="gemini-3.8-flash">Gemini 3.8 Flash</option>
            <option value="gemini-3.7-flash">Gemini 3.7 Flash</option>
            <option value="gemini-2.5-pro">Gemini 2.5 Pro</option>
            <option value="claude-3-7-sonnet">Claude 3.7 Sonnet</option>
            <option value="gpt-4o">GPT-4o</option>
          </select>

          {/* SmartShield Evaluate Button */}
          <button
            onClick={handleEvaluateShield}
            disabled={evaluating}
            className="flex items-center gap-2 px-3 py-1.5 rounded bg-[#111111] hover:bg-[#1a1a1a] text-[#ededed] text-xs font-medium border border-[#333333] transition disabled:opacity-50"
            title="Evaluate active account against token burn velocity and burst quota thresholds"
          >
            <ShieldCheck className={`w-3.5 h-3.5 ${evaluating ? "animate-pulse text-[#f5a623]" : "text-[#10b981]"}`} />
            {evaluating ? "Evaluating..." : "Check SmartShield 2.0"}
          </button>

          {/* Export Dropdown */}
          <div className="relative">
            <button
              onClick={() => setExportMenuOpen(!exportMenuOpen)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-[#111111] hover:bg-[#1a1a1a] text-[#ededed] text-xs font-medium border border-[#333333] transition"
            >
              <Download className="w-3.5 h-3.5 text-[#888888]" />
              Export
            </button>

            {exportMenuOpen && (
              <div className="absolute right-0 mt-1 w-44 bg-[#0a0a0a] border border-[#262626] rounded-md shadow-2xl py-1 z-50 text-xs font-mono">
                <button
                  onClick={() => handleExport("csv")}
                  className="w-full text-left px-3 py-2 text-[#cccccc] hover:bg-[#161616] hover:text-white flex items-center gap-2"
                >
                  <FileSpreadsheet className="w-3.5 h-3.5 text-[#10b981]" />
                  Download CSV
                </button>
                <button
                  onClick={() => handleExport("json")}
                  className="w-full text-left px-3 py-2 text-[#cccccc] hover:bg-[#161616] hover:text-white flex items-center gap-2"
                >
                  <FileJson className="w-3.5 h-3.5 text-[#0070f3]" />
                  Download JSON
                </button>
              </div>
            )}
          </div>

          {/* Auto Refresh Toggle */}
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded text-xs font-mono border transition ${
              autoRefresh
                ? "bg-[#10b981]/10 text-[#10b981] border-[#10b981]/30"
                : "bg-[#111111] text-[#666666] border-[#222222]"
            }`}
          >
            <div className={`w-1.5 h-1.5 rounded-full ${autoRefresh ? "bg-[#10b981] animate-ping" : "bg-[#555555]"}`} />
            {autoRefresh ? "Auto (10s)" : "Paused"}
          </button>

          {/* Manual Refresh */}
          <button
            onClick={() => fetchData()}
            disabled={isRefreshing}
            className="p-1.5 rounded bg-[#111111] hover:bg-[#1a1a1a] text-[#888888] hover:text-[#ededed] border border-[#222222] transition disabled:opacity-50"
            title="Refresh analytics data"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? "animate-spin text-[#0070f3]" : ""}`} />
          </button>
        </div>
      </div>

      {/* 2. TOP CARDS: FINANCIAL RUNWAY & HEALTH OVERVIEW */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Card 1: Commercial Savings */}
        <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-4 flex flex-col justify-between hover:border-[#333333] transition relative overflow-hidden group">
          <div className="absolute top-0 right-0 w-24 h-24 bg-[#10b981]/5 rounded-bl-full pointer-events-none group-hover:bg-[#10b981]/10 transition" />
          <div>
            <div className="flex items-center justify-between text-[#888888] text-xs font-medium mb-1">
              <span>Commercial Savings (ROI)</span>
              <DollarSign className="w-4 h-4 text-[#10b981]" />
            </div>
            <div className="text-2xl font-bold font-mono text-[#ededed] tracking-tight">
              ${s?.commercial_savings_usd.toFixed(2)}
            </div>
            <p className="text-[11px] text-[#888888] mt-1">
              100% saved via $0 Pro accounts vs commercial API rates.
            </p>
          </div>
          <div className="mt-3 pt-2.5 border-t border-[#1a1a1a] flex items-center justify-between text-[11px] font-mono">
            <span className="text-[#666666]">Annualized Runway:</span>
            <span className="text-[#10b981] font-semibold flex items-center gap-0.5">
              <ArrowUpRight className="w-3 h-3" />
              ${s?.annual_projected_roi_usd.toLocaleString()} / yr
            </span>
          </div>
        </div>

        {/* Card 2: Total Token Consumption */}
        <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-4 flex flex-col justify-between hover:border-[#333333] transition relative overflow-hidden group">
          <div className="absolute top-0 right-0 w-24 h-24 bg-[#0070f3]/5 rounded-bl-full pointer-events-none group-hover:bg-[#0070f3]/10 transition" />
          <div>
            <div className="flex items-center justify-between text-[#888888] text-xs font-medium mb-1">
              <span>Fleet Token Volume</span>
              <Cpu className="w-4 h-4 text-[#0070f3]" />
            </div>
            <div className="text-2xl font-bold font-mono text-[#ededed] tracking-tight">
              {(s ? s.total_tokens / 1_000_000 : 0).toFixed(2)}M
            </div>
            <p className="text-[11px] text-[#888888] mt-1">
              {((s?.total_prompt_tokens || 0) / 1_000_000).toFixed(2)}M prompt + {((s?.total_completion_tokens || 0) / 1_000_000).toFixed(2)}M completion.
            </p>
          </div>
          <div className="mt-3 pt-2.5 border-t border-[#1a1a1a] flex items-center justify-between text-[11px] font-mono">
            <span className="text-[#666666]">Total Calls Logged:</span>
            <span className="text-[#0070f3] font-semibold">
              {s?.total_calls.toLocaleString()} calls
            </span>
          </div>
        </div>

        {/* Card 3: Burn Velocity */}
        <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-4 flex flex-col justify-between hover:border-[#333333] transition relative overflow-hidden group">
          <div className="absolute top-0 right-0 w-24 h-24 bg-[#f5a623]/5 rounded-bl-full pointer-events-none group-hover:bg-[#f5a623]/10 transition" />
          <div>
            <div className="flex items-center justify-between text-[#888888] text-xs font-medium mb-1">
              <span>Token Burn Velocity</span>
              <Flame className="w-4 h-4 text-[#f5a623]" />
            </div>
            <div className="text-2xl font-bold font-mono text-[#ededed] tracking-tight">
              {((s?.burn_rate_tokens_per_hour || 0) / 1000).toFixed(0)}k <span className="text-xs text-[#888888] font-normal">/ hr</span>
            </div>
            <p className="text-[11px] text-[#888888] mt-1">
              Today: {((s?.today_tokens || 0) / 1_000_000).toFixed(2)}M tokens across {s?.today_calls.toLocaleString()} calls.
            </p>
          </div>
          <div className="mt-3 pt-2.5 border-t border-[#1a1a1a] flex items-center justify-between text-[11px] font-mono">
            <span className="text-[#666666]">30-Day Proj. Vol:</span>
            <span className="text-[#ededed] font-semibold">
              {((s?.projected_monthly_tokens || 0) / 1_000_000).toFixed(0)}M tokens
            </span>
          </div>
        </div>

        {/* Card 4: SmartShield 2.0 Status */}
        <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-4 flex flex-col justify-between hover:border-[#333333] transition relative overflow-hidden group">
          <div className="absolute top-0 right-0 w-24 h-24 bg-[#10b981]/5 rounded-bl-full pointer-events-none group-hover:bg-[#10b981]/10 transition" />
          <div>
            <div className="flex items-center justify-between text-[#888888] text-xs font-medium mb-1">
              <span>SmartShield 2.0 Status</span>
              <ShieldCheck className="w-4 h-4 text-[#10b981]" />
            </div>
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-mono font-semibold bg-[#10b981]/10 text-[#10b981] border border-[#10b981]/30">
                <span className="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-pulse" />
                {shield?.risk_level || "NOMINAL"}
              </span>
              <span className="text-xs font-mono text-[#888888]">
                ({shield?.active_account_burst_pct.toFixed(1)}% 5h)
              </span>
            </div>
            <p className="text-[11px] text-[#888888] mt-1 truncate" title={shield?.active_account_email}>
              Active: {shield?.active_account_email || "mushfiq.diit@gmail.com"}
            </p>
          </div>
          <div className="mt-3 pt-2.5 border-t border-[#1a1a1a] flex items-center justify-between text-[11px] font-mono">
            <span className="text-[#666666]">Handover Cutoff:</span>
            <span className="text-[#ededed]">
              ≤ {shield?.burst_threshold_pct}% burst
            </span>
          </div>
        </div>
      </div>

      {/* 3. SECTION: 24-HOUR HOURLY TOKEN VELOCITY (SVG CHART) */}
      <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
          <div>
            <h2 className="text-sm font-semibold text-[#ededed] flex items-center gap-2">
              <Activity className="w-4 h-4 text-[#0070f3]" />
              24-Hour Hourly Token Velocity &amp; Burn Rate
            </h2>
            <p className="text-xs text-[#888888] mt-0.5">
              Live hourly telemetry across all IDE, Desktop, and Gateway requests over the last 24 hours.
            </p>
          </div>

          <div className="flex items-center gap-1 bg-[#111111] p-0.5 rounded border border-[#262626] text-xs font-mono">
            <button
              onClick={() => setHourlyMetric("tokens")}
              className={`px-2.5 py-1 rounded transition ${
                hourlyMetric === "tokens" ? "bg-[#222222] text-[#ededed] font-medium" : "text-[#777777] hover:text-[#cccccc]"
              }`}
            >
              Tokens
            </button>
            <button
              onClick={() => setHourlyMetric("calls")}
              className={`px-2.5 py-1 rounded transition ${
                hourlyMetric === "calls" ? "bg-[#222222] text-[#ededed] font-medium" : "text-[#777777] hover:text-[#cccccc]"
              }`}
            >
              Calls
            </button>
            <button
              onClick={() => setHourlyMetric("cost")}
              className={`px-2.5 py-1 rounded transition ${
                hourlyMetric === "cost" ? "bg-[#222222] text-[#ededed] font-medium" : "text-[#777777] hover:text-[#cccccc]"
              }`}
            >
              Savings ($)
            </button>
          </div>
        </div>

        {/* Hourly SVG Chart Container */}
        <div className="relative w-full h-56 pt-2 select-none">
          {/* Chart Background Grid Lines */}
          <div className="absolute inset-0 flex flex-col justify-between pointer-events-none pb-7 opacity-20">
            <div className="border-b border-[#333333] w-full" />
            <div className="border-b border-[#333333] w-full" />
            <div className="border-b border-[#333333] w-full" />
            <div className="border-b border-[#333333] w-full" />
          </div>

          {/* SVG Bar Rendering */}
          <div className="relative h-44 flex items-end justify-between gap-1 z-10 px-1">
            {hourlyData.map((d, idx) => {
              const val =
                hourlyMetric === "tokens"
                  ? d.total_tokens
                  : hourlyMetric === "calls"
                  ? d.calls
                  : d.commercial_cost_usd;
              const heightPct = Math.max(4, Math.round((val / maxHourlyVal) * 100));
              const isHovered = hoveredHour?.timestamp === d.timestamp;

              return (
                <div
                  key={d.timestamp}
                  className="flex-1 h-full flex flex-col justify-end items-center group relative cursor-pointer"
                  onMouseEnter={() => setHoveredHour(d)}
                  onMouseLeave={() => setHoveredHour(null)}
                >
                  {/* Visual Bar */}
                  <div
                    style={{ height: `${heightPct}%` }}
                    className={`w-full max-w-[28px] rounded-t-sm transition-all duration-200 ${
                      val === 0
                        ? "bg-[#181818]"
                        : isHovered
                        ? "bg-[#0070f3] shadow-[0_0_12px_rgba(0,112,243,0.6)]"
                        : "bg-gradient-to-t from-[#0070f3]/40 to-[#0070f3]/90 hover:to-[#0070f3]"
                    }`}
                  />

                  {/* X-axis label (show every 4 hours or first/last) */}
                  <span className="text-[10px] font-mono text-[#555555] group-hover:text-[#ededed] mt-2 transition">
                    {idx % 4 === 0 || idx === hourlyData.length - 1 ? d.hour_label : ""}
                  </span>
                </div>
              );
            })}
          </div>

          {/* Interactive Tooltip Card */}
          {hoveredHour && (
            <div className="absolute top-2 right-4 bg-[#0d0d0d] border border-[#2a2a2a] rounded p-2.5 text-xs font-mono shadow-2xl z-20 pointer-events-none">
              <div className="text-[#888888] font-semibold border-b border-[#1f1f1f] pb-1 mb-1.5 flex items-center justify-between gap-4">
                <span>{hoveredHour.full_label}</span>
                <span className="text-[#0070f3]">{hoveredHour.calls} calls</span>
              </div>
              <div className="grid grid-cols-2 gap-x-4 gap-y-1">
                <span className="text-[#666666]">Prompt Tokens:</span>
                <span className="text-[#ededed] text-right font-medium">
                  {hoveredHour.prompt_tokens.toLocaleString()}
                </span>
                <span className="text-[#666666]">Completion Tokens:</span>
                <span className="text-[#ededed] text-right font-medium">
                  {hoveredHour.completion_tokens.toLocaleString()}
                </span>
                <span className="text-[#666666]">Total Volume:</span>
                <span className="text-[#10b981] text-right font-bold">
                  {hoveredHour.total_tokens.toLocaleString()}
                </span>
                <span className="text-[#666666]">Commercial Value:</span>
                <span className="text-[#ededed] text-right font-bold">
                  ${hoveredHour.commercial_cost_usd.toFixed(4)}
                </span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* 4. SECTION: 14-DAY DAILY CONSUMPTION & ROLLING RESET TIMERS (2 COLUMNS) */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Left Column: 14-Day Trajectory */}
        <div className="lg:col-span-6 bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-3">
              <div>
                <h2 className="text-sm font-semibold text-[#ededed] flex items-center gap-2">
                  <TrendingUp className="w-4 h-4 text-[#10b981]" />
                  14-Day Trajectory &amp; Daily Savings
                </h2>
                <p className="text-xs text-[#888888] mt-0.5">
                  Day-by-day fleet token volume and commercial dollar value saved.
                </p>
              </div>
            </div>

            {/* Daily Bars */}
            <div className="space-y-2 mt-4">
              {dailyData.map((d) => {
                const widthPct = Math.max(2, Math.round((d.total_tokens / maxDailyVal) * 100));
                return (
                  <div
                    key={d.date}
                    className="flex items-center gap-3 text-xs font-mono group hover:bg-[#121212] p-1.5 rounded transition"
                  >
                    <span className="w-16 text-[#666666] group-hover:text-[#ededed] shrink-0">
                      {d.display_date} ({d.day_of_week})
                    </span>

                    <div className="flex-1 bg-[#141414] rounded-sm h-3 relative overflow-hidden flex items-center">
                      <div
                        style={{ width: `${widthPct}%` }}
                        className="bg-gradient-to-r from-[#10b981]/50 to-[#10b981] h-full rounded-sm transition-all duration-300"
                      />
                    </div>

                    <span className="w-20 text-right text-[#ededed] font-medium shrink-0">
                      {(d.total_tokens / 1_000_000).toFixed(2)}M
                    </span>

                    <span className="w-16 text-right text-[#10b981] font-semibold shrink-0">
                      ${d.savings_usd.toFixed(2)}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="mt-4 pt-3 border-t border-[#1a1a1a] flex items-center justify-between text-xs font-mono text-[#666666]">
            <span>14-Day Cumulative:</span>
            <span className="text-[#ededed] font-bold">
              {(dailyData.reduce((acc, curr) => acc + curr.total_tokens, 0) / 1_000_000).toFixed(2)}M tokens / $
              {dailyData.reduce((acc, curr) => acc + curr.savings_usd, 0).toFixed(2)} saved
            </span>
          </div>
        </div>

        {/* Right Column: Fleet Quota Reset Timers Matrix */}
        <div className="lg:col-span-6 bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-3">
              <div>
                <h2 className="text-sm font-semibold text-[#ededed] flex items-center gap-2">
                  <Clock className="w-4 h-4 text-[#f5a623]" />
                  Fleet Quota Reset Countdown Timers
                </h2>
                <p className="text-xs text-[#888888] mt-0.5">
                  Ticking countdown to next 5h burst rolling window reset per account.
                </p>
              </div>
            </div>

            <div className="space-y-3 mt-4">
              {data?.account_timers.map((acc) => {
                const remainingSec = accountSeconds[acc.account_id] ?? acc.seconds_until_reset;
                const isCritical = acc.burst_5h_pct <= (shield?.burst_threshold_pct || 15);
                const isStandbyReady = !acc.is_active && acc.burst_5h_pct > 50 && acc.health_status === "HEALTHY";

                return (
                  <div
                    key={acc.account_id}
                    className={`p-3 rounded border text-xs font-mono transition ${
                      acc.is_active
                        ? "bg-[#0c121e] border-[#0070f3]/40"
                        : "bg-[#0e0e0e] border-[#1e1e1e] hover:border-[#2a2a2a]"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2 truncate">
                        {acc.is_active ? (
                          <span className="w-2 h-2 rounded-full bg-[#0070f3] animate-ping" />
                        ) : (
                          <span className="w-2 h-2 rounded-full bg-[#333333]" />
                        )}
                        <span className="text-[#ededed] font-semibold truncate">{acc.email}</span>
                        {acc.is_active && (
                          <span className="px-1.5 py-0.2 rounded text-[10px] bg-[#0070f3]/20 text-[#0070f3] font-bold border border-[#0070f3]/30">
                            ACTIVE
                          </span>
                        )}
                      </div>

                      {/* Reset Countdown Pill */}
                      <div className="flex items-center gap-1.5 bg-[#141414] px-2 py-0.5 rounded border border-[#222222] text-[#f5a623] font-bold shrink-0">
                        <Clock className="w-3 h-3 text-[#f5a623]" />
                        <span>{formatSeconds(remainingSec)}</span>
                      </div>
                    </div>

                    {/* Progress Bars */}
                    <div className="mt-2.5 grid grid-cols-2 gap-3">
                      <div>
                        <div className="flex justify-between text-[11px] mb-1">
                          <span className="text-[#666666]">5h Burst Quota:</span>
                          <span
                            className={
                              isCritical
                                ? "text-[#f87171] font-bold"
                                : acc.burst_5h_pct < 50
                                ? "text-[#f5a623]"
                                : "text-[#10b981]"
                            }
                          >
                            {acc.burst_5h_pct.toFixed(1)}%
                          </span>
                        </div>
                        <div className="w-full bg-[#181818] h-1.5 rounded-full overflow-hidden">
                          <div
                            style={{ width: `${acc.burst_5h_pct}%` }}
                            className={`h-full rounded-full ${
                              isCritical
                                ? "bg-[#f87171]"
                                : acc.burst_5h_pct < 50
                                ? "bg-[#f5a623]"
                                : "bg-[#10b981]"
                            }`}
                          />
                        </div>
                      </div>

                      <div>
                        <div className="flex justify-between text-[11px] mb-1">
                          <span className="text-[#666666]">Weekly Quota:</span>
                          <span className="text-[#ededed]">{acc.weekly_pct.toFixed(1)}%</span>
                        </div>
                        <div className="w-full bg-[#181818] h-1.5 rounded-full overflow-hidden">
                          <div
                            style={{ width: `${acc.weekly_pct}%` }}
                            className="bg-[#0070f3] h-full rounded-full"
                          />
                        </div>
                      </div>
                    </div>

                    {/* Quick Shift Button */}
                    {!acc.is_active && (
                      <div className="mt-2.5 pt-2 border-t border-[#1a1a1a] flex items-center justify-between">
                        <span className="text-[10px] text-[#666666]">
                          {!acc.enabled ? "Paused" : isStandbyReady ? "Standby Ready for Zero-Stall Shift" : `Status: ${acc.health_status}`}
                        </span>
                        <div className="flex items-center gap-3">
                          <button
                            onClick={() => handleToggleAccount(acc.account_id, acc.enabled)}
                            className={`text-[10px] font-medium transition ${acc.enabled ? "text-[#f5a623] hover:text-[#f87171]" : "text-[#10b981] hover:text-white"}`}
                          >
                            {acc.enabled ? "Pause" : "Enable"}
                          </button>
                          
                          <button
                            onClick={() => {
                              const target = accounts.find((a) => a.id === acc.account_id);
                              if (target) handleSwitchAccount(target, "both");
                            }}
                            disabled={!acc.enabled || switchingId === `${acc.account_id}-both`}
                            className="text-[11px] text-[#0070f3] hover:text-white flex items-center gap-1 font-medium transition disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            {switchingId === `${acc.account_id}-both` ? "Switching..." : "Shift Traffic Now"}
                            <ChevronRight className="w-3 h-3" />
                          </button>
                        </div>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          <div className="mt-4 pt-3 border-t border-[#1a1a1a] flex items-center justify-between text-xs font-mono text-[#888888]">
            <span>Google Cloud Code Window:</span>
            <span className="text-[#ededed]">5-Hour Rolling Burst</span>
          </div>
        </div>
      </div>

      {/* 5. SECTION: SMARTSHIELD 2.0 ZERO-STALL HANDOVER AUDIT LOG */}
      <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
          <div>
            <h2 className="text-sm font-semibold text-[#ededed] flex items-center gap-2">
              <Zap className="w-4 h-4 text-[#10b981]" />
              SmartShield 2.0 Predictive Zero-Stall Handover Engine
            </h2>
            <p className="text-xs text-[#888888] mt-0.5">
              Continuously monitors 15-minute token burn rate and preemptively switches accounts before 429 quota exhaustion.
            </p>
          </div>

          <div className="flex items-center gap-3 text-xs font-mono">
            <span className="text-[#666666]">15m Burn Velocity:</span>
            <span className="text-[#0070f3] font-bold">
              {((shield?.recent_15m_tokens || 0) / 1000).toFixed(0)}k tokens / 15m
            </span>
          </div>
        </div>

        {shield?.last_handover ? (
          <div className="bg-[#111111] border border-[#222222] rounded-md p-4 text-xs font-mono">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2">
                <CheckCircle2 className="w-4 h-4 text-[#10b981]" />
                <span className="font-semibold text-[#ededed]">Latest Zero-Stall Handover</span>
              </div>
              <span className="text-[#666666]">
                {new Date(shield.last_handover.timestamp).toLocaleString()}
              </span>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-[#aaaaaa] mt-3">
              <div>
                <div className="text-[10px] text-[#666666] uppercase">From Account</div>
                <div className="text-[#ededed] font-medium mt-0.5">{shield.last_handover.from_account_email}</div>
                <div className="text-[10px] text-[#f87171]">Previous burst: {shield.last_handover.previous_burst_pct.toFixed(1)}%</div>
              </div>
              <div>
                <div className="text-[10px] text-[#666666] uppercase">To Account</div>
                <div className="text-[#10b981] font-medium mt-0.5">{shield.last_handover.to_account_email}</div>
                <div className="text-[10px] text-[#10b981]">New burst: {shield.last_handover.new_burst_pct.toFixed(1)}%</div>
              </div>
              <div>
                <div className="text-[10px] text-[#666666] uppercase">Trigger Reason</div>
                <div className="text-[#ededed] mt-0.5 truncate" title={shield.last_handover.reason}>
                  {shield.last_handover.reason}
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="bg-[#0e0e0e] border border-[#1e1e1e] rounded-md p-4 text-xs font-mono text-[#888888] flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-[#10b981]" />
              <span>SmartShield 2.0 active and monitoring. Active account quota has remained safely above the {shield?.burst_threshold_pct}% burst threshold.</span>
            </div>
            <span className="text-[11px] text-[#10b981] font-semibold">ALL QUOTAS NOMINAL</span>
          </div>
        )}
      </div>

      {/* 6. SECTION: MODEL COMMERCIAL VALUE & EFFICIENCY BREAKDOWN */}
      <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-5">
        <div className="mb-4">
          <h2 className="text-sm font-semibold text-[#ededed] flex items-center gap-2">
            <Layers className="w-4 h-4 text-[#0070f3]" />
            Commercial Model Pricing vs $0 OmniGate Fleet Value
          </h2>
          <p className="text-xs text-[#888888] mt-0.5">
            Dollar-for-dollar pricing analysis based on public cloud provider API rates.
          </p>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs font-mono">
            <thead>
              <tr className="border-b border-[#1e1e1e] text-[#666666] text-[11px]">
                <th className="pb-2.5 font-medium">Model</th>
                <th className="pb-2.5 font-medium">Family</th>
                <th className="pb-2.5 font-medium text-right">Calls</th>
                <th className="pb-2.5 font-medium text-right">Prompt Tokens</th>
                <th className="pb-2.5 font-medium text-right">Completion Tokens</th>
                <th className="pb-2.5 font-medium text-right">Commercial API Cost</th>
                <th className="pb-2.5 font-medium text-right">OmniGate Cost</th>
                <th className="pb-2.5 font-medium text-right text-[#10b981]">Net Savings</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#161616]">
              {data?.model_breakdown.map((m) => (
                <tr key={m.model} className="hover:bg-[#0e0e0e] transition">
                  <td className="py-2.5 font-semibold text-[#ededed] flex items-center gap-2">
                    <span className="w-1.5 h-1.5 rounded-full bg-[#0070f3]" />
                    {m.name}
                  </td>
                  <td className="py-2.5 text-[#888888]">{m.family}</td>
                  <td className="py-2.5 text-right text-[#ededed]">{m.calls.toLocaleString()}</td>
                  <td className="py-2.5 text-right text-[#888888]">{(m.prompt_tokens / 1_000_000).toFixed(2)}M</td>
                  <td className="py-2.5 text-right text-[#888888]">{(m.completion_tokens / 1_000_000).toFixed(2)}M</td>
                  <td className="py-2.5 text-right text-[#ededed] font-medium">${m.commercial_cost_usd.toFixed(2)}</td>
                  <td className="py-2.5 text-right text-[#666666] font-mono">$0.00</td>
                  <td className="py-2.5 text-right text-[#10b981] font-bold">${m.commercial_cost_usd.toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
