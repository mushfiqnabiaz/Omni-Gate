"use client";

import React, { useState, useEffect, useRef } from "react";
import { useConsole, RequestLog } from "@/context/ConsoleContext";
import {
  Search,
  RefreshCw,
  Activity,
  Sparkles,
  Laptop,
  Monitor,
  Key,
  ShieldCheck,
  TrendingDown,
  Coins,
  Cpu,
  Clock,
  CheckCircle2,
  AlertCircle,
  ExternalLink,
  ChevronRight,
  ChevronLeft,
  ChevronsLeft,
  ChevronsRight,
  ChevronDown,
  ChevronUp,
  Zap,
  Pause,
  Play,
  Copy,
  Check,
  X,
  SlidersHorizontal,
  Info,
} from "lucide-react";

export default function CallLoggerPage() {
  const {
    requestLogs,
    pricingSummary,
    pagination,
    ideStatus,
    desktopStatus,
    fetchLogs,
    loading,
  } = useConsole();

  // State
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10); // Default 10 data as requested!
  const [selectedClient, setSelectedClient] = useState<string>("all");
  const [filterQuery, setFilterQuery] = useState("");
  const [selectedModel, setSelectedModel] = useState<string>("all");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [showMatrix, setShowMatrix] = useState(true);
  const [inspectLog, setInspectLog] = useState<RequestLog | null>(null);
  const [copiedId, setCopiedId] = useState(false);

  // Debounced search query
  const [debouncedQuery, setDebouncedQuery] = useState("");
  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedQuery(filterQuery);
      setCurrentPage(1); // reset to page 1 on search
    }, 300);
    return () => clearTimeout(handler);
  }, [filterQuery]);

  // Fetch logs whenever page, limit, client, model, or search changes
  useEffect(() => {
    fetchLogs({
      page: currentPage,
      limit: pageSize,
      client: selectedClient,
      search: debouncedQuery,
      model: selectedModel,
    });
  }, [currentPage, pageSize, selectedClient, selectedModel, debouncedQuery]);

  // Real-time live polling every 1.5 seconds if autoRefresh is enabled
  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(() => {
      fetchLogs({
        page: currentPage,
        limit: pageSize,
        client: selectedClient,
        search: debouncedQuery,
        model: selectedModel,
      });
    }, 1500);
    return () => clearInterval(interval);
  }, [autoRefresh, currentPage, pageSize, selectedClient, selectedModel, debouncedQuery]);

  const activeEmail = ideStatus?.email || desktopStatus?.email || "mushfiq.diit@gmail.com";
  const activeModel = "Gemini 3.8 Flash (High)";

  // Pagination helpers
  const totalRecords = pagination?.total_records || requestLogs.length;
  const totalPages = pagination?.total_pages || Math.ceil(totalRecords / pageSize) || 1;
  const startItem = totalRecords === 0 ? 0 : (currentPage - 1) * pageSize + 1;
  const endItem = Math.min(currentPage * pageSize, totalRecords);

  const handleCopyReqId = (id: string) => {
    navigator.clipboard.writeText(id);
    setCopiedId(true);
    setTimeout(() => setCopiedId(false), 2000);
  };

  return (
    <div className="space-y-6 max-w-6xl mx-auto pb-16">
      {/* 1. TOP HEADER & TELEMETRY CONTROLS */}
      <div className="flex flex-col md:flex-row md:items-center justify-between pb-6 border-b border-[#1e1e1e] gap-4">
        <div>
          <div className="flex items-center gap-2.5 mb-1.5 flex-wrap">
            <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
              <Activity className="h-6 w-6 text-[#0070f3]" />
              Realtime Call Logger & Pricing Intelligence
            </h1>
            <div
              className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-mono border ${
                autoRefresh
                  ? "bg-[#061e12] border-[#0e4429] text-[#10b981]"
                  : "bg-[#201505] border-[#442c0a] text-[#f59e0b]"
              }`}
            >
              <span className="flex h-1.5 w-1.5 relative">
                {autoRefresh && (
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-[#10b981] opacity-75"></span>
                )}
                <span
                  className={`relative inline-flex rounded-full h-1.5 w-1.5 ${
                    autoRefresh ? "bg-[#10b981]" : "bg-[#f59e0b]"
                  }`}
                ></span>
              </span>
              <span>{autoRefresh ? "LIVE STREAMING (1.5s)" : "STREAM PAUSED"}</span>
            </div>
          </div>
          <p className="text-xs text-[#a1a1a1] font-mono">
            Capturing active account and model calls in real time. Token consumption pricing benchmarked against official Google Cloud & Gemini API rates.
          </p>
        </div>

        {/* Live Controls & Active Account Pill */}
        <div className="flex items-center gap-2 flex-wrap">
          <div className="p-2 rounded-md bg-[#0a0a0a] border border-[#1e1e1e] text-xs font-mono flex items-center gap-3">
            <div className="flex items-center gap-2">
              <span className="text-[#666666]">ACTIVE:</span>
              <span className="text-white font-semibold flex items-center gap-1.5">
                <span className="h-2 w-2 rounded-full bg-[#10b981]"></span>
                {activeEmail}
              </span>
            </div>
            <div className="h-3 w-px bg-[#262626]" />
            <div className="flex items-center gap-1.5 text-[#0070f3]">
              <Zap className="h-3 w-3" />
              <span>{activeModel}</span>
            </div>
          </div>

          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`h-9 px-3 rounded-md text-xs font-mono inline-flex items-center gap-1.5 border transition cursor-pointer ${
              autoRefresh
                ? "bg-[#0a0a0a] hover:bg-[#141414] border-[#222222] text-[#a1a1a1] hover:text-white"
                : "bg-[#0070f3] hover:bg-[#0060df] border-[#0070f3] text-white font-medium"
            }`}
            title={autoRefresh ? "Pause Live Polling" : "Resume Live Polling"}
          >
            {autoRefresh ? <Pause className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5 fill-white" />}
            <span>{autoRefresh ? "Pause" : "Resume"}</span>
          </button>

          <button
            onClick={() =>
              fetchLogs({
                page: currentPage,
                limit: pageSize,
                client: selectedClient,
                search: debouncedQuery,
                model: selectedModel,
              })
            }
            className="h-9 w-9 rounded-md bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-[#707070] hover:text-white transition flex items-center justify-center cursor-pointer"
            title="Force Refresh"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin text-[#0070f3]" : ""}`} />
          </button>
        </div>
      </div>

      {/* 2. EXECUTIVE FINANCIAL & PERFORMANCE HUD (TOP 4 KPI CARDS) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
        {/* Card 1: Commercial Google Cloud API Equivalent */}
        <div className="vercel-card p-4 space-y-2">
          <div className="flex items-center justify-between text-xs text-[#707070] font-mono">
            <span>COMMERCIAL API RATE</span>
            <Coins className="h-4 w-4 text-[#f59e0b]" />
          </div>
          <div className="text-2xl font-bold font-mono tracking-tight text-[#f59e0b]">
            ${pricingSummary ? pricingSummary.commercial_api_cost_usd.toFixed(4) : "0.0000"}
          </div>
          <div className="text-[11px] text-[#707070] font-mono pt-1 border-t border-[#181818] flex items-center justify-between">
            <span>Official GCP Gemini Rate</span>
            <span>Vertex AI</span>
          </div>
        </div>

        {/* Card 2: Antigravity Pro Cost */}
        <div className="vercel-card p-4 space-y-2">
          <div className="flex items-center justify-between text-xs text-[#707070] font-mono">
            <span>ANTIGRAVITY PRO COST</span>
            <Sparkles className="h-4 w-4 text-[#0070f3]" />
          </div>
          <div className="text-2xl font-bold font-mono tracking-tight text-white">
            $0.0000
          </div>
          <div className="text-[11px] text-[#10b981] font-mono pt-1 border-t border-[#181818] flex items-center justify-between">
            <span className="flex items-center gap-1">
              <CheckCircle2 className="h-3 w-3" />
              <span>Zero Marginal Cost</span>
            </span>
            <span className="text-[10px] px-1.5 py-0.2 rounded bg-[#041a36] text-[#0070f3] border border-[#0a3069]">
              PRO TIER
            </span>
          </div>
        </div>

        {/* Card 3: Net Cumulative Pro Savings */}
        <div className="vercel-card p-4 space-y-2 bg-[#001408] border-[#0e4429]">
          <div className="flex items-center justify-between text-xs text-[#10b981] font-mono">
            <span>NET PRO SAVINGS</span>
            <TrendingDown className="h-4 w-4 text-[#10b981]" />
          </div>
          <div className="text-2xl font-bold font-mono tracking-tight text-[#10b981]">
            ${pricingSummary ? pricingSummary.total_savings_usd.toFixed(4) : "0.0000"}
          </div>
          <div className="text-[11px] text-[#10b981] font-mono pt-1 border-t border-[#0e4429] flex items-center justify-between">
            <span>Total Calls: {totalRecords.toLocaleString()}</span>
            <span className="px-1.5 py-0.2 rounded bg-[#061e12] border border-[#10b981]/30 font-bold">
              100% SAVED
            </span>
          </div>
        </div>

        {/* Card 4: Total Token Throughput */}
        <div className="vercel-card p-4 space-y-2">
          <div className="flex items-center justify-between text-xs text-[#707070] font-mono">
            <span>TOKEN CONSUMPTION</span>
            <Cpu className="h-4 w-4 text-[#0070f3]" />
          </div>
          <div className="text-2xl font-bold font-mono tracking-tight text-white">
            {pricingSummary ? (pricingSummary.total_tokens / 1000).toFixed(1) + "k" : "—"}
          </div>
          <div className="text-[11px] text-[#a1a1a1] font-mono flex items-center justify-between pt-1 border-t border-[#181818]">
            <span>In: {pricingSummary ? (pricingSummary.total_prompt_tokens / 1000).toFixed(1) + "k" : "—"}</span>
            <span>Out: {pricingSummary ? (pricingSummary.total_completion_tokens / 1000).toFixed(1) + "k" : "—"}</span>
          </div>
        </div>
      </div>

      {/* 3. OFFICIAL API PRICING & CONSUMPTION COMPARISON MATRIX (COLLAPSIBLE) */}
      <div className="vercel-card overflow-hidden">
        <button
          onClick={() => setShowMatrix(!showMatrix)}
          className="w-full p-4 flex items-center justify-between hover:bg-[#080808] transition text-left cursor-pointer border-b border-[#1e1e1e]"
        >
          <div className="flex items-center gap-2">
            <Coins className="h-4 w-4 text-[#0070f3]" />
            <h2 className="text-sm font-semibold text-white tracking-tight">
              Official Google Cloud / Gemini API Pricing & Consumption Benchmark Matrix
            </h2>
            <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-[#141414] text-[#707070] border border-[#222222]">
              {pricingSummary?.model_comparison?.length || 0} Models
            </span>
          </div>
          <div className="flex items-center gap-2 text-xs font-mono text-[#707070]">
            <span>{showMatrix ? "Collapse Matrix" : "Expand Matrix"}</span>
            {showMatrix ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          </div>
        </button>

        {showMatrix && (
          <div className="p-4 space-y-3">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs border-collapse font-mono">
                <thead>
                  <tr className="border-b border-[#1e1e1e] text-[#666666] uppercase text-[10px] bg-[#000000]">
                    <th className="py-2.5 px-3">Model Architecture</th>
                    <th className="py-2.5 px-3">Commercial Input / 1M</th>
                    <th className="py-2.5 px-3">Commercial Output / 1M</th>
                    <th className="py-2.5 px-3">Antigravity Pro Rate</th>
                    <th className="py-2.5 px-3">Logged Calls</th>
                    <th className="py-2.5 px-3">Tokens Consumed</th>
                    <th className="py-2.5 px-3">Commercial Cost</th>
                    <th className="py-2.5 px-3">Antigravity Cost</th>
                    <th className="py-2.5 px-3 text-right">Net Value Saved</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#1e1e1e]">
                  {pricingSummary?.model_comparison && pricingSummary.model_comparison.length > 0 ? (
                    pricingSummary.model_comparison.map((mc, idx) => (
                      <tr key={idx} className="hover:bg-[#0c0c0c] transition">
                        <td className="py-2.5 px-3">
                          <div className="font-semibold text-white">{mc.name}</div>
                          <div className="text-[10px] text-[#666666]">{mc.family}</div>
                        </td>
                        <td className="py-2.5 px-3 text-[#a1a1a1]">${mc.prompt_rate_per_1m.toFixed(3)}</td>
                        <td className="py-2.5 px-3 text-[#a1a1a1]">${mc.completion_rate_per_1m.toFixed(2)}</td>
                        <td className="py-2.5 px-3 text-[#10b981] font-semibold">$0.00 / 1M</td>
                        <td className="py-2.5 px-3 text-[#ededed]">{mc.calls.toLocaleString()}</td>
                        <td className="py-2.5 px-3 text-[#ededed]">{mc.total_tokens.toLocaleString()}</td>
                        <td className="py-2.5 px-3 text-[#f59e0b] font-medium">${mc.commercial_cost_usd.toFixed(4)}</td>
                        <td className="py-2.5 px-3 text-white font-medium">$0.0000</td>
                        <td className="py-2.5 px-3 text-right">
                          <span className="px-2 py-0.5 rounded text-[11px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429]">
                            +${mc.savings_usd.toFixed(4)}
                          </span>
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td colSpan={9} className="py-8 text-center text-[#666666]">
                        No model pricing comparison available.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
            <div className="text-[11px] text-[#666666] font-mono flex items-center justify-between pt-2 border-t border-[#1a1a1a]">
              <span>Pricing Formula: (Prompt Tokens &times; Input Rate) + (Output Tokens &times; Output Rate) / 1,000,000</span>
              <span>Updated: Official Google Cloud Pricing Catalog</span>
            </div>
          </div>
        )}
      </div>

      {/* 4. INTERACTIVE TOOLBAR & CONTROLS */}
      <div className="space-y-3">
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3">
          {/* Client Filter Tabs */}
          <div className="flex items-center gap-1.5 bg-[#0a0a0a] p-1 rounded-md border border-[#1e1e1e] overflow-x-auto">
            {[
              { id: "all", label: "All Calls" },
              { id: "ide", label: "Antigravity IDE", icon: Laptop },
              { id: "desktop", label: "Antigravity Desktop", icon: Monitor },
              { id: "keys", label: "Virtual Keys", icon: Key },
              { id: "gateway", label: "Gateway Proxy", icon: Activity },
            ].map((tab) => {
              const TabIcon = tab.icon;
              return (
                <button
                  key={tab.id}
                  onClick={() => {
                    setSelectedClient(tab.id);
                    setCurrentPage(1); // reset to page 1
                  }}
                  className={`px-3 py-1.5 rounded text-xs font-mono transition cursor-pointer flex items-center gap-1.5 whitespace-nowrap ${
                    selectedClient === tab.id
                      ? "bg-[#1f1f1f] text-white font-medium border border-[#333333]"
                      : "text-[#707070] hover:text-[#a1a1a1]"
                  }`}
                >
                  {TabIcon && <TabIcon className="h-3 w-3" />}
                  <span>{tab.label}</span>
                </button>
              );
            })}
          </div>

          {/* Search & Model Selector */}
          <div className="flex items-center gap-2 flex-wrap sm:flex-nowrap">
            {/* Search Box */}
            <div className="relative w-full sm:w-64">
              <Search className="h-3.5 w-3.5 absolute left-3 top-2.5 text-[#555555]" />
              <input
                type="text"
                placeholder="Search model, email, req_id..."
                value={filterQuery}
                onChange={(e) => setFilterQuery(e.target.value)}
                className="w-full h-8 pl-9 pr-3 bg-[#0a0a0a] border border-[#222222] rounded-md text-xs font-mono text-white placeholder-[#555555] focus:outline-none focus:border-[#0070f3]"
              />
            </div>

            {/* Page Size Selector (Default 10) */}
            <div className="flex items-center gap-1 text-xs font-mono text-[#707070] bg-[#0a0a0a] px-2.5 py-1 rounded-md border border-[#222222]">
              <span>Rows:</span>
              <select
                value={pageSize}
                onChange={(e) => {
                  setPageSize(Number(e.target.value));
                  setCurrentPage(1);
                }}
                className="bg-transparent text-white font-mono focus:outline-none cursor-pointer"
              >
                <option value={10} className="bg-black text-white">10 / page (Default)</option>
                <option value={25} className="bg-black text-white">25 / page</option>
                <option value={50} className="bg-black text-white">50 / page</option>
                <option value={100} className="bg-black text-white">100 / page</option>
              </select>
            </div>
          </div>
        </div>

        {/* 5. PAGINATED CALL STREAM TABLE (DEFAULT 10 DATA) */}
        <div className="vercel-card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs border-collapse font-mono">
              <thead>
                <tr className="border-b border-[#1e1e1e] text-[#666666] uppercase text-[10px] bg-[#000000]">
                  <th className="py-2.5 px-4">Time</th>
                  <th className="py-2.5 px-4">Origin Client</th>
                  <th className="py-2.5 px-4">Calling Account</th>
                  <th className="py-2.5 px-4">Active Model</th>
                  <th className="py-2.5 px-4">Tokens Consumed</th>
                  <th className="py-2.5 px-4">Commercial API Rate</th>
                  <th className="py-2.5 px-4">Antigravity Pro</th>
                  <th className="py-2.5 px-4">Latency</th>
                  <th className="py-2.5 px-4">Status</th>
                  <th className="py-2.5 px-3 text-right">Inspect</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#181818]">
                {requestLogs.length === 0 ? (
                  <tr>
                    <td colSpan={10} className="py-16 text-center text-[#666666]">
                      No calls matching current filters.
                    </td>
                  </tr>
                ) : (
                  requestLogs.map((l, idx) => {
                    const isIDE = l.client_type === "ide";
                    const isDesktop = l.client_type === "desktop";
                    const isKey = l.client_type === "virtual_key";

                    const clientBadgeClass = isIDE
                      ? "bg-[#041a36] text-[#0070f3] border-[#0a3069]"
                      : isDesktop
                      ? "bg-[#1e1035] text-[#a855f7] border-[#3b1d6b]"
                      : isKey
                      ? "bg-[#251805] text-[#f59e0b] border-[#4a2e0a]"
                      : "bg-[#141414] text-[#a1a1a1] border-[#262626]";

                    const ClientIcon = isIDE ? Laptop : isDesktop ? Monitor : isKey ? Key : Activity;

                    return (
                      <tr
                        key={idx}
                        onClick={() => setInspectLog(l)}
                        className="hover:bg-[#0c0c0c] transition cursor-pointer group"
                      >
                        {/* Time */}
                        <td className="py-2.5 px-4 text-[#707070] whitespace-nowrap">
                          {new Date(l.created_at).toLocaleTimeString([], {
                            hour: "2-digit",
                            minute: "2-digit",
                            second: "2-digit",
                          })}
                        </td>

                        {/* Client Origin */}
                        <td className="py-2.5 px-4 whitespace-nowrap">
                          <span
                            className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-semibold border ${clientBadgeClass}`}
                          >
                            <ClientIcon className="h-3 w-3" />
                            {l.client_origin || (isIDE ? "Antigravity IDE" : "Gateway")}
                          </span>
                        </td>

                        {/* Account Email & Name */}
                        <td className="py-2.5 px-4 whitespace-nowrap">
                          <div className="text-white font-medium flex items-center gap-1.5">
                            <span className="h-1.5 w-1.5 rounded-full bg-[#10b981]"></span>
                            <span>{l.account_email || activeEmail}</span>
                          </div>
                          <div className="text-[10px] text-[#666666]">{l.account_name}</div>
                        </td>

                        {/* Calling Model */}
                        <td className="py-2.5 px-4 whitespace-nowrap">
                          <span className="font-semibold text-white">
                            {l.model_name || l.model}
                          </span>
                        </td>

                        {/* Token Consumption */}
                        <td className="py-2.5 px-4 whitespace-nowrap">
                          <span className="text-white font-medium">
                            {(l.total_tokens || l.prompt_tokens + l.completion_tokens).toLocaleString()}
                          </span>{" "}
                          <span className="text-[#666666] text-[10px]">
                            ({l.prompt_tokens.toLocaleString()}p / {l.completion_tokens.toLocaleString()}c)
                          </span>
                        </td>

                        {/* Commercial API Cost */}
                        <td className="py-2.5 px-4 text-[#f59e0b] whitespace-nowrap font-medium">
                          ${l.pricing ? l.pricing.api_cost_usd.toFixed(6) : "0.000000"}
                        </td>

                        {/* Antigravity Cost & Net Savings */}
                        <td className="py-2.5 px-4 whitespace-nowrap">
                          <span className="text-white font-medium">$0.0000</span>{" "}
                          <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429] ml-1">
                            +{l.pricing ? l.pricing.savings_usd.toFixed(5) : "0.0000"}
                          </span>
                        </td>

                        {/* Latency */}
                        <td className="py-2.5 px-4 text-[#10b981] whitespace-nowrap">
                          {l.duration_ms}ms
                        </td>

                        {/* HTTP Status */}
                        <td className="py-2.5 px-4 whitespace-nowrap">
                          {l.status_code === 200 ? (
                            <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429]">
                              200 OK
                            </span>
                          ) : (
                            <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-[#260b0b] text-[#ef4444] border border-[#441111]">
                              {l.status_code}
                            </span>
                          )}
                        </td>

                        {/* Inspect Icon */}
                        <td className="py-2.5 px-3 text-right whitespace-nowrap">
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              setInspectLog(l);
                            }}
                            className="p-1 rounded text-[#555555] hover:text-white hover:bg-[#1a1a1a] transition"
                            title="Inspect Call Telemetry"
                          >
                            <ChevronRight className="h-3.5 w-3.5" />
                          </button>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>

          {/* 6. PAGINATION CONTROLLER (DEFAULT 10 DATA) */}
          <div className="p-3.5 border-t border-[#1e1e1e] bg-[#000000] flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs font-mono">
            {/* Range info */}
            <div className="text-[#707070]">
              Showing <span className="text-white font-semibold">{startItem}</span> -{" "}
              <span className="text-white font-semibold">{endItem}</span> of{" "}
              <span className="text-white font-semibold">{totalRecords.toLocaleString()}</span> calls
            </div>

            {/* Page navigation buttons */}
            <div className="flex items-center gap-1.5">
              {/* First Page */}
              <button
                onClick={() => setCurrentPage(1)}
                disabled={currentPage <= 1}
                className="h-7 w-7 rounded bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-[#a1a1a1] hover:text-white disabled:opacity-30 disabled:cursor-not-allowed flex items-center justify-center transition cursor-pointer"
                title="First Page"
              >
                <ChevronsLeft className="h-3.5 w-3.5" />
              </button>

              {/* Prev Page */}
              <button
                onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
                disabled={currentPage <= 1}
                className="h-7 px-2 rounded bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-[#a1a1a1] hover:text-white disabled:opacity-30 disabled:cursor-not-allowed flex items-center gap-1 transition cursor-pointer"
              >
                <ChevronLeft className="h-3.5 w-3.5" />
                <span>Prev</span>
              </button>

              {/* Page Number Pills */}
              <div className="flex items-center gap-1 px-1">
                {(() => {
                  const pages: (number | string)[] = [];
                  if (totalPages <= 5) {
                    for (let i = 1; i <= totalPages; i++) pages.push(i);
                  } else {
                    if (currentPage <= 3) {
                      pages.push(1, 2, 3, 4, "...", totalPages);
                    } else if (currentPage >= totalPages - 2) {
                      pages.push(1, "...", totalPages - 3, totalPages - 2, totalPages - 1, totalPages);
                    } else {
                      pages.push(1, "...", currentPage - 1, currentPage, currentPage + 1, "...", totalPages);
                    }
                  }

                  return pages.map((p, i) =>
                    typeof p === "number" ? (
                      <button
                        key={i}
                        onClick={() => setCurrentPage(p)}
                        className={`h-7 min-w-7 px-1.5 rounded text-xs font-mono transition cursor-pointer ${
                          currentPage === p
                            ? "bg-[#0070f3] text-white font-bold"
                            : "bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-[#a1a1a1] hover:text-white"
                        }`}
                      >
                        {p}
                      </button>
                    ) : (
                      <span key={i} className="px-1 text-[#555555]">
                        {p}
                      </span>
                    )
                  );
                })()}
              </div>

              {/* Next Page */}
              <button
                onClick={() => setCurrentPage(Math.min(totalPages, currentPage + 1))}
                disabled={currentPage >= totalPages}
                className="h-7 px-2 rounded bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-[#a1a1a1] hover:text-white disabled:opacity-30 disabled:cursor-not-allowed flex items-center gap-1 transition cursor-pointer"
              >
                <span>Next</span>
                <ChevronRight className="h-3.5 w-3.5" />
              </button>

              {/* Last Page */}
              <button
                onClick={() => setCurrentPage(totalPages)}
                disabled={currentPage >= totalPages}
                className="h-7 w-7 rounded bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-[#a1a1a1] hover:text-white disabled:opacity-30 disabled:cursor-not-allowed flex items-center justify-center transition cursor-pointer"
                title="Last Page"
              >
                <ChevronsRight className="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* 7. INSPECT CALL TELEMETRY MODAL */}
      {inspectLog && (
        <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-[#0a0a0a] border border-[#262626] rounded-lg max-w-xl w-full p-6 space-y-5 shadow-2xl font-mono text-xs">
            <div className="flex items-center justify-between pb-4 border-b border-[#1e1e1e]">
              <div className="flex items-center gap-2">
                <Activity className="h-4 w-4 text-[#0070f3]" />
                <h3 className="text-sm font-bold text-white tracking-tight">Call Telemetry & Pricing Breakdown</h3>
              </div>
              <button
                onClick={() => setInspectLog(null)}
                className="text-[#707070] hover:text-white p-1 rounded hover:bg-[#141414] transition cursor-pointer"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="space-y-4">
              {/* Request ID & Origin */}
              <div className="p-3 bg-[#111111] rounded border border-[#1e1e1e] space-y-2">
                <div className="flex items-center justify-between text-[#707070]">
                  <span>REQUEST ID</span>
                  <button
                    onClick={() => handleCopyReqId(inspectLog.req_id)}
                    className="flex items-center gap-1 text-[#0070f3] hover:underline cursor-pointer"
                  >
                    {copiedId ? <Check className="h-3 w-3 text-[#10b981]" /> : <Copy className="h-3 w-3" />}
                    <span>{copiedId ? "Copied" : "Copy"}</span>
                  </button>
                </div>
                <div className="text-white font-mono break-all text-[11px]">{inspectLog.req_id}</div>
              </div>

              {/* Grid Metadata */}
              <div className="grid grid-cols-2 gap-3 text-xs">
                <div className="p-3 bg-[#111111] rounded border border-[#1e1e1e] space-y-1">
                  <span className="text-[#666666] text-[10px]">CLIENT ORIGIN</span>
                  <div className="text-white font-semibold">{inspectLog.client_origin || "Antigravity IDE"}</div>
                </div>

                <div className="p-3 bg-[#111111] rounded border border-[#1e1e1e] space-y-1">
                  <span className="text-[#666666] text-[10px]">CALLING ACCOUNT</span>
                  <div className="text-white font-semibold truncate">{inspectLog.account_email || activeEmail}</div>
                </div>

                <div className="p-3 bg-[#111111] rounded border border-[#1e1e1e] space-y-1">
                  <span className="text-[#666666] text-[10px]">MODEL ARCHITECTURE</span>
                  <div className="text-white font-semibold">{inspectLog.model_name || inspectLog.model}</div>
                </div>

                <div className="p-3 bg-[#111111] rounded border border-[#1e1e1e] space-y-1">
                  <span className="text-[#666666] text-[10px]">LATENCY & STATUS</span>
                  <div className="text-[#10b981] font-semibold">
                    {inspectLog.duration_ms}ms &bull; HTTP {inspectLog.status_code}
                  </div>
                </div>
              </div>

              {/* Token & Pricing Breakdown */}
              <div className="p-3.5 bg-[#001408] rounded border border-[#0e4429] space-y-2">
                <div className="flex items-center justify-between text-[#10b981] font-semibold">
                  <span>FINANCIAL VALUE BREAKDOWN</span>
                  <span className="px-1.5 py-0.2 rounded bg-[#061e12] border border-[#10b981]/30 text-[10px]">
                    100% PRO SAVINGS
                  </span>
                </div>
                <div className="grid grid-cols-2 gap-2 text-[11px] text-[#ededed] pt-1">
                  <div>
                    <span className="text-[#a1a1a1]">Prompt Tokens: </span>
                    <span className="font-bold">{inspectLog.prompt_tokens.toLocaleString()}</span>
                  </div>
                  <div>
                    <span className="text-[#a1a1a1]">Output Tokens: </span>
                    <span className="font-bold">{inspectLog.completion_tokens.toLocaleString()}</span>
                  </div>
                  <div>
                    <span className="text-[#a1a1a1]">Commercial Rate: </span>
                    <span className="font-bold text-[#f59e0b]">
                      ${inspectLog.pricing?.api_cost_usd.toFixed(6) || "0.000000"}
                    </span>
                  </div>
                  <div>
                    <span className="text-[#a1a1a1]">Antigravity Pro: </span>
                    <span className="font-bold text-white">$0.0000</span>
                  </div>
                </div>
                <div className="pt-2 border-t border-[#0e4429] text-[11px] text-[#10b981] flex justify-between font-bold">
                  <span>Net Savings on this call:</span>
                  <span>+${inspectLog.pricing?.savings_usd.toFixed(6) || "0.000000"} USD</span>
                </div>
              </div>
            </div>

            <div className="pt-3 border-t border-[#1e1e1e] flex justify-end">
              <button
                onClick={() => setInspectLog(null)}
                className="h-8 px-4 rounded bg-[#1f1f1f] hover:bg-[#262626] text-white text-xs font-semibold transition cursor-pointer"
              >
                Close Inspector
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
