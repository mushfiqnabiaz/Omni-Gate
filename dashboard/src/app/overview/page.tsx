"use client";

import React from "react";
import Link from "next/link";
import { useConsole, Account } from "@/context/ConsoleContext";
import {
  ArrowRight,
  RefreshCw,
  Sparkles,
  CheckCircle2,
  AlertTriangle,
  Play,
  Laptop,
  Monitor,
  Activity,
  Key,
} from "lucide-react";

export default function OverviewPage() {
  const {
    accounts,
    activeSessionId,
    ideStatus,
    desktopStatus,
    virtualKeys,
    requestLogs,
    pricingSummary,
    handleSwitchAccount,
    switchingId,
  } = useConsole();

  const googleAccounts = accounts.filter((a) => a.provider === "google");
  const activeAccount = accounts.find((a) => a.id === activeSessionId) || googleAccounts[0];

  const ideEmail = ideStatus?.connected && ideStatus.email ? ideStatus.email.toLowerCase() : "";
  const desktopEmail = desktopStatus?.connected && desktopStatus.email ? desktopStatus.email.toLowerCase() : "";
  const isBothConnected = Boolean(ideStatus?.connected && desktopStatus?.connected);
  const isClientsMatched = Boolean(ideEmail && desktopEmail && ideEmail === desktopEmail);

  const googleLogs = requestLogs.filter((l) => l.provider === "google");
  const avgGoogleMs =
    googleLogs.length > 0
      ? Math.round(googleLogs.reduce((acc, l) => acc + (l.duration_ms || 0), 0) / googleLogs.length)
      : 320;

  const totalTokens = virtualKeys.reduce((acc, k) => acc + (k.total_tokens || 0), 0);
  const totalRequests = virtualKeys.reduce((acc, k) => acc + (k.total_requests || 0), 0);

  return (
    <div className="space-y-8 max-w-6xl mx-auto">
      {/* 1. PAGE HEADER (Vercel Style) */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-6 border-b border-[#1e1e1e] gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Overview</h1>
          <p className="text-xs text-[#a1a1a1] mt-1 font-mono">
            Antigravity Pro fleet management, dual-client synchronization, and virtual gateway dispatch.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Link
            href="/playground"
            className="h-8 px-3 rounded-md bg-white hover:bg-[#eaeaea] text-black text-xs font-semibold inline-flex items-center gap-1.5 transition cursor-pointer shadow-sm"
          >
            <Play className="h-3 w-3 fill-black" />
            <span>AI Playground</span>
          </Link>
          <Link
            href="/fleet/google"
            className="h-8 px-3 rounded-md bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-xs text-white inline-flex items-center gap-1.5 transition font-mono"
          >
            <span>Manage Fleet</span>
            <ArrowRight className="h-3 w-3 text-[#707070]" />
          </Link>
        </div>
      </div>

      {/* 2. METRIC CARDS GRID (Vercel Clean Engineering Cards) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Metric 1 */}
        <div className="vercel-card p-5 space-y-2">
          <span className="text-[10px] font-mono text-[#707070] uppercase font-semibold tracking-wider block">
            Fleet Pool Size
          </span>
          <div className="flex items-baseline justify-between">
            <span className="text-3xl font-bold text-white tracking-tight font-mono">
              {googleAccounts.length}
            </span>
            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-[#061e12] text-[#10b981] border border-[#0e4429]">
              ALL ACTIVE
            </span>
          </div>
          <p className="text-[11px] text-[#707070] font-mono pt-1">
            Google AI Pro accounts pooled
          </p>
        </div>

        {/* Metric 2 */}
        <div className="vercel-card p-5 space-y-2">
          <span className="text-[10px] font-mono text-[#707070] uppercase font-semibold tracking-wider block">
            Active Client Sync
          </span>
          <div className="flex items-baseline justify-between">
            <span className="text-base font-bold text-[#0070f3] font-mono truncate max-w-[130px]">
              {ideStatus?.connected ? ideStatus.email.split("@")[0] : activeAccount?.email?.split("@")[0]}
            </span>
            <span
              className={`text-[10px] font-mono px-1.5 py-0.2 rounded ${
                isClientsMatched
                  ? "bg-[#061e12] text-[#10b981] border border-[#0e4429]"
                  : "bg-[#201505] text-[#f59e0b] border border-[#442c0a]"
              }`}
            >
              {isClientsMatched ? "MATCHED" : "UNSYNCED"}
            </span>
          </div>
          <p className="text-[11px] text-[#707070] font-mono pt-1">
            IDE (PID {ideStatus?.pid || "—"}) &bull; Desktop (PID {desktopStatus?.pid || "—"})
          </p>
        </div>

        {/* Metric 3 */}
        <div className="vercel-card p-5 space-y-2">
          <span className="text-[10px] font-mono text-[#707070] uppercase font-semibold tracking-wider block">
            Tokens Consumed
          </span>
          <div className="flex items-baseline justify-between">
            <span className="text-3xl font-bold text-white tracking-tight font-mono">
              {pricingSummary ? (pricingSummary.total_tokens / 1000).toFixed(1) + "k" : "—"}
            </span>
            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-[#041a36] text-[#0070f3] border border-[#0a3069]">
              LIVE
            </span>
          </div>
          <p className="text-[11px] text-[#707070] font-mono pt-1">
            {pricingSummary?.total_calls || requestLogs.length} total calls recorded
          </p>
        </div>

        {/* Metric 4 */}
        <div className="vercel-card p-5 space-y-2 bg-[#001408] border-[#0e4429]">
          <span className="text-[10px] font-mono text-[#10b981] uppercase font-semibold tracking-wider block">
            Net Pro Savings
          </span>
          <div className="flex items-baseline justify-between">
            <span className="text-3xl font-bold text-[#10b981] tracking-tight font-mono">
              ${pricingSummary ? pricingSummary.total_savings_usd.toFixed(4) : "0.0000"}
            </span>
            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-[#061e12] text-[#10b981] border border-[#0e4429] font-bold">
              100% SAVED
            </span>
          </div>
          <p className="text-[11px] text-[#10b981] font-mono pt-1">
            vs. Google Cloud API commercial pricing
          </p>
        </div>
      </div>

      {/* 3. DUAL-CLIENT SYNCHRONIZATION CONTROLLER (Cloudflare Zero Trust Service Card) */}
      <div className="vercel-card overflow-hidden">
        <div className="p-5 border-b border-[#1e1e1e] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold text-white">Client Co-Synchronization Status</h2>
            <p className="text-xs text-[#a1a1a1] mt-0.5">
              Live inspection of Antigravity IDE (VS Code pipe) and Antigravity Desktop (standalone Electron).
            </p>
          </div>

          <button
            onClick={() => activeAccount && handleSwitchAccount(activeAccount, "both")}
            disabled={Boolean(switchingId)}
            className="h-[30px] px-3 rounded-md bg-[#141414] hover:bg-[#1a1a1a] border border-[#2a2a2a] text-xs font-mono text-[#0070f3] inline-flex items-center gap-1.5 transition cursor-pointer self-start sm:self-auto"
          >
            <RefreshCw className={`h-3 w-3 ${switchingId ? "animate-spin text-[#0070f3]" : ""}`} />
            <span>{switchingId ? "Synchronizing..." : "Sync Both Clients"}</span>
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-[#1e1e1e]">
          {/* Left: IDE */}
          <div className="p-5 space-y-3 font-mono text-xs">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Laptop className="h-4 w-4 text-[#0070f3]" />
                <span className="font-semibold text-white">Antigravity IDE</span>
              </div>
              <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                ideStatus?.connected ? "bg-[#061e12] text-[#10b981] border border-[#0e4429]" : "bg-[#200a0a] text-[#ef4444] border border-[#441111]"
              }`}>
                {ideStatus?.connected ? "CONNECTED" : "OFFLINE"}
              </span>
            </div>

            <div className="space-y-1.5 text-[#a1a1a1]">
              <div className="flex justify-between">
                <span className="text-[#707070]">Account</span>
                <span className="text-white font-medium">{ideStatus?.email || "None"}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[#707070]">Process PID</span>
                <span>{ideStatus?.pid || "—"}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[#707070]">Language Server</span>
                <span>port {ideStatus?.ports?.join(", ") || "—"}</span>
              </div>
            </div>
          </div>

          {/* Right: Desktop */}
          <div className="p-5 space-y-3 font-mono text-xs">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Monitor className="h-4 w-4 text-[#a855f7]" />
                <span className="font-semibold text-white">Antigravity Desktop</span>
              </div>
              <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                desktopStatus?.connected ? "bg-[#061e12] text-[#10b981] border border-[#0e4429]" : "bg-[#200a0a] text-[#ef4444] border border-[#441111]"
              }`}>
                {desktopStatus?.connected ? "CONNECTED" : "OFFLINE"}
              </span>
            </div>

            <div className="space-y-1.5 text-[#a1a1a1]">
              <div className="flex justify-between">
                <span className="text-[#707070]">Account</span>
                <span className="text-white font-medium">{desktopStatus?.email || "None"}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[#707070]">Process PID</span>
                <span>{desktopStatus?.pid || "—"}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[#707070]">Connect RPC</span>
                <span>port {desktopStatus?.ports?.join(", ") || "—"}</span>
              </div>
            </div>
          </div>
        </div>

        <div className="px-5 py-2.5 bg-[#0a0a0a] border-t border-[#1e1e1e] flex items-center justify-between text-xs font-mono">
          <div className="flex items-center gap-2">
            <span className={`w-2 h-2 rounded-full ${isClientsMatched ? "bg-[#10b981]" : "bg-[#f59e0b]"}`} />
            <span className="text-[#a1a1a1]">
              {isClientsMatched
                ? "Both instances authenticated to the same OAuth token."
                : "Clients are pointing to different accounts."}
            </span>
          </div>
          <span className="text-[#707070] text-[11px]">macOS Keychain Sync</span>
        </div>
      </div>

      {/* 4. ACCOUNT FLEET & QUOTA ALLOCATION (Clean Vercel Data Table) */}
      <div className="vercel-card overflow-hidden">
        <div className="p-5 border-b border-[#1e1e1e] flex items-center justify-between">
          <div>
            <h2 className="text-sm font-semibold text-white">Account Fleet & Quota Allocation</h2>
            <p className="text-xs text-[#a1a1a1] mt-0.5">
              Live 5-hour rolling burst quotas and weekly limits across all pooled accounts.
            </p>
          </div>
          <Link
            href="/fleet/google"
            className="text-xs text-[#0070f3] hover:underline font-mono inline-flex items-center gap-1"
          >
            <span>Full Fleet</span>
            <ArrowRight className="h-3 w-3" />
          </Link>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs border-collapse font-mono">
            <thead>
              <tr className="border-b border-[#1e1e1e] text-[#707070] uppercase text-[10px] bg-[#000000]">
                <th className="py-2.5 px-5 font-semibold">Account</th>
                <th className="py-2.5 px-5 font-semibold">Sync State</th>
                <th className="py-2.5 px-5 font-semibold">Burst Quota (5-Hour)</th>
                <th className="py-2.5 px-5 font-semibold">Weekly Quota</th>
                <th className="py-2.5 px-5 font-semibold text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#1e1e1e]">
              {googleAccounts.map((acc) => {
                const accEmail = acc.email.toLowerCase();
                const isLiveIde = Boolean(ideStatus?.connected && ideEmail === accEmail);
                const isLiveDesktop = Boolean(desktopStatus?.connected && desktopEmail === accEmail);
                const isFullySynced = isLiveIde && isLiveDesktop;
                const isPrimary = acc.id === activeSessionId;

                const burst = acc.quotas?.burst_5h_pct ?? 100;
                const weekly = acc.quotas?.weekly_pct ?? 100;

                return (
                  <tr key={acc.id} className="hover:bg-[#0f0f0f] transition">
                    <td className="py-3 px-5">
                      <p className="font-sans font-medium text-white text-xs">{acc.name}</p>
                      <p className="text-[#707070] text-[11px]">{acc.email}</p>
                    </td>

                    <td className="py-3 px-5">
                      {isFullySynced ? (
                        <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429]">
                          ACTIVE IN BOTH
                        </span>
                      ) : isLiveIde ? (
                        <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-[#001f3f] text-[#0070f3] border border-[#003366]">
                          IDE ONLY
                        </span>
                      ) : isLiveDesktop ? (
                        <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-[#1d0d2b] text-[#a855f7] border border-[#3b1754]">
                          DESKTOP ONLY
                        </span>
                      ) : (
                        <span className="px-2 py-0.5 rounded text-[10px] bg-[#141414] text-[#707070] border border-[#222222]">
                          STANDBY
                        </span>
                      )}
                    </td>

                    <td className="py-3 px-5">
                      <div className="w-36 space-y-1">
                        <div className="flex justify-between text-[11px]">
                          <span className={burst < 20 ? "text-[#ef4444]" : burst < 50 ? "text-[#f59e0b]" : "text-[#10b981]"}>
                            {burst.toFixed(1)}%
                          </span>
                        </div>
                        <div className="w-full h-1 bg-[#1a1a1a] rounded-full overflow-hidden">
                          <div
                            className={`h-full ${burst < 20 ? "bg-[#ef4444]" : burst < 50 ? "bg-[#f59e0b]" : "bg-[#10b981]"}`}
                            style={{ width: `${burst}%` }}
                          />
                        </div>
                      </div>
                    </td>

                    <td className="py-3 px-5">
                      <div className="w-36 space-y-1">
                        <div className="flex justify-between text-[11px]">
                          <span className="text-[#a1a1a1]">{weekly.toFixed(1)}%</span>
                        </div>
                        <div className="w-full h-1 bg-[#1a1a1a] rounded-full overflow-hidden">
                          <div className="h-full bg-[#0070f3]" style={{ width: `${weekly}%` }} />
                        </div>
                      </div>
                    </td>

                    <td className="py-3 px-5 text-right">
                      <button
                        onClick={() => handleSwitchAccount(acc, "both")}
                        disabled={(isFullySynced && isPrimary) || Boolean(switchingId)}
                        className={`h-6 px-2.5 rounded text-[11px] font-medium transition cursor-pointer ${
                          isFullySynced && isPrimary
                            ? "text-[#10b981] bg-[#061e12] border border-[#0e4429] cursor-default"
                            : "text-white bg-[#141414] hover:bg-[#1a1a1a] border border-[#262626]"
                        }`}
                      >
                        {switchingId === `${acc.id}-both`
                          ? "Syncing..."
                          : isFullySynced && isPrimary
                          ? "Synced"
                          : "Sync Both"}
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* 5. REALTIME CALL LOGGER & PRICING STREAM */}
      <div className="vercel-card overflow-hidden">
        <div className="p-5 border-b border-[#1e1e1e] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-sm font-semibold text-white">Live Call Logger & Cost Savings Stream</h2>
              <span className="flex h-1.5 w-1.5 relative">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-[#10b981] opacity-75"></span>
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-[#10b981]"></span>
              </span>
            </div>
            <p className="text-xs text-[#a1a1a1] mt-0.5 font-mono">
              Real-time feed of active IDE, Desktop & Gateway calls with token price comparison.
            </p>
          </div>
          <Link
            href="/logs"
            className="text-xs text-[#0070f3] hover:underline font-mono inline-flex items-center gap-1.5 self-start sm:self-auto"
          >
            <span>Open Full Pricing Console</span>
            <ArrowRight className="h-3 w-3" />
          </Link>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs border-collapse font-mono">
            <thead>
              <tr className="border-b border-[#1e1e1e] text-[#666666] uppercase text-[10px] bg-[#000000]">
                <th className="py-2.5 px-4">Time</th>
                <th className="py-2.5 px-4">Client</th>
                <th className="py-2.5 px-4">Account</th>
                <th className="py-2.5 px-4">Calling Model</th>
                <th className="py-2.5 px-4">Tokens</th>
                <th className="py-2.5 px-4">Commercial Cost</th>
                <th className="py-2.5 px-4">Pro Cost</th>
                <th className="py-2.5 px-4">Savings</th>
                <th className="py-2.5 px-4 text-right">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#181818]">
              {requestLogs.slice(0, 6).map((l, idx) => {
                const isIDE = l.client_type === "ide";
                const isDesktop = l.client_type === "desktop";
                return (
                  <tr key={idx} className="hover:bg-[#0c0c0c] transition">
                    <td className="py-2.5 px-4 text-[#707070] whitespace-nowrap">
                      {new Date(l.created_at).toLocaleTimeString([], {
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit",
                      })}
                    </td>
                    <td className="py-2.5 px-4 whitespace-nowrap">
                      <span
                        className={`inline-flex items-center gap-1 px-1.5 py-0.2 rounded text-[10px] font-semibold border ${
                          isIDE
                            ? "bg-[#041a36] text-[#0070f3] border-[#0a3069]"
                            : isDesktop
                            ? "bg-[#1e1035] text-[#a855f7] border-[#3b1d6b]"
                            : "bg-[#141414] text-[#a1a1a1] border-[#262626]"
                        }`}
                      >
                        {isIDE ? "IDE" : isDesktop ? "Desktop" : "Gateway"}
                      </span>
                    </td>
                    <td className="py-2.5 px-4 text-white whitespace-nowrap font-medium">
                      {l.account_email || ideEmail || "mushfiq.diit@gmail.com"}
                    </td>
                    <td className="py-2.5 px-4 text-white whitespace-nowrap font-medium">
                      {l.model_name || l.model}
                    </td>
                    <td className="py-2.5 px-4 text-[#ededed] whitespace-nowrap">
                      {(l.total_tokens || l.prompt_tokens + l.completion_tokens).toLocaleString()}
                    </td>
                    <td className="py-2.5 px-4 text-[#f59e0b] whitespace-nowrap font-medium">
                      ${l.pricing ? l.pricing.api_cost_usd.toFixed(5) : "0.00000"}
                    </td>
                    <td className="py-2.5 px-4 text-white whitespace-nowrap font-medium">
                      $0.0000
                    </td>
                    <td className="py-2.5 px-4 whitespace-nowrap">
                      <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429]">
                        +{l.pricing ? l.pricing.savings_usd.toFixed(5) : "0.0000"}
                      </span>
                    </td>
                    <td className="py-2.5 px-4 text-right whitespace-nowrap">
                      <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429]">
                        {l.status_code || 200}
                      </span>
                    </td>
                  </tr>
                );
              })}
              {requestLogs.length === 0 && (
                <tr>
                  <td colSpan={9} className="py-8 text-center text-[#666666]">
                    No calls recorded yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
