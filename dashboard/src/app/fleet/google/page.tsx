"use client";

import React from "react";
import { ArrowRight, RefreshCw, Monitor, Laptop, CheckCircle2, AlertTriangle } from "lucide-react";
import { useConsole, Account } from "@/context/ConsoleContext";

export default function GoogleFleetPage() {
  const { accounts, activeSessionId, ideStatus, desktopStatus, switchingId, handleSwitchAccount } = useConsole();
  const googleAccounts = accounts.filter((a) => a.provider === "google");

  const ideEmail = ideStatus?.connected && ideStatus.email ? ideStatus.email.toLowerCase() : "";
  const desktopEmail = desktopStatus?.connected && desktopStatus.email ? desktopStatus.email.toLowerCase() : "";
  const isBothConnected = Boolean(ideStatus?.connected && desktopStatus?.connected);
  const isAccountsMatched = Boolean(ideEmail && desktopEmail && ideEmail === desktopEmail);

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-6 border-b border-[#1e1e1e] gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Antigravity Fleet Management</h1>
          <p className="text-xs text-[#a1a1a1] mt-1 font-mono">
            Manage pooled Google Cloud Code Pro accounts with dual-client synchronization.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <span className="px-2.5 py-1 rounded text-xs font-mono font-medium bg-[#141414] text-[#a1a1a1] border border-[#262626]">
            {googleAccounts.length} Pro Accounts Pooled
          </span>
        </div>
      </div>

      {/* Sync Status Banner (Cloudflare Zero Trust Style) */}
      <div
        className={`vercel-card p-5 transition duration-150 ${
          isAccountsMatched ? "border-[#0e4429]" : "border-[#442c0a]"
        }`}
      >
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            {isAccountsMatched ? (
              <div className="p-2 rounded bg-[#061e12] text-[#10b981] border border-[#0e4429]">
                <CheckCircle2 className="h-4 w-4" />
              </div>
            ) : (
              <div className="p-2 rounded bg-[#201505] text-[#f59e0b] border border-[#442c0a]">
                <AlertTriangle className="h-4 w-4" />
              </div>
            )}
            <div>
              <div className="flex items-center gap-2">
                <span className="font-semibold text-sm text-white">
                  {isAccountsMatched
                    ? "IDE & Desktop Synchronized"
                    : "IDE & Desktop Credentials Mismatch"}
                </span>
                <span
                  className={`px-1.5 py-0.2 text-[9px] font-mono rounded ${
                    isAccountsMatched
                      ? "bg-[#061e12] text-[#10b981] border border-[#0e4429]"
                      : "bg-[#201505] text-[#f59e0b] border border-[#442c0a]"
                  }`}
                >
                  {isAccountsMatched ? "MATCHED" : "UNSYNCED"}
                </span>
              </div>
              <p className="text-xs text-[#707070] mt-0.5 font-mono">
                {isAccountsMatched ? (
                  <>Active session on: <span className="text-white font-medium">{ideStatus?.email}</span></>
                ) : (
                  <>IDE: <span className="text-white">{ideStatus?.email || "Offline"}</span> &bull; Desktop: <span className="text-white">{desktopStatus?.email || "Offline"}</span></>
                )}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2 font-mono text-xs">
            <div className="flex items-center gap-1.5 text-white bg-[#141414] px-2.5 py-1.5 rounded border border-[#262626]">
              <Laptop className="h-3.5 w-3.5 text-[#0070f3]" />
              <span>IDE: {ideStatus?.connected ? `PID ${ideStatus.pid}` : "Offline"}</span>
            </div>
            <div className="flex items-center gap-1.5 text-white bg-[#141414] px-2.5 py-1.5 rounded border border-[#262626]">
              <Monitor className="h-3.5 w-3.5 text-[#a855f7]" />
              <span>Desktop: {desktopStatus?.connected ? `PID ${desktopStatus.pid}` : "Offline"}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Accounts Grid (Clean Vercel Card Grid) */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {googleAccounts.map((acc) => {
          const accEmail = acc.email.toLowerCase();
          const isLiveIde = Boolean(ideStatus?.connected && ideEmail === accEmail);
          const isLiveDesktop = Boolean(desktopStatus?.connected && desktopEmail === accEmail);
          const isFullySynced = isLiveIde && isLiveDesktop;
          const isPrimary = acc.id === activeSessionId;

          const burstPct = acc.quotas?.burst_5h_pct ?? 100;
          const weeklyPct = acc.quotas?.weekly_pct ?? 100;
          const isCooldown = acc.cooldown_until > Date.now();
          const isSwitching = switchingId === `${acc.id}-both` || switchingId === acc.id;

          return (
            <div
              key={acc.id}
              className={`vercel-card p-5 space-y-4 transition ${
                isFullySynced
                  ? "border-[#0e4429] bg-[#030d08]"
                  : isPrimary
                  ? "border-[#003366] bg-[#020b14]"
                  : ""
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-semibold text-xs text-white">{acc.name}</span>
                    {isFullySynced && (
                      <span className="px-1.5 py-0.2 text-[9px] font-mono bg-[#061e12] text-[#10b981] border border-[#0e4429] rounded">
                        ACTIVE IN BOTH
                      </span>
                    )}
                    {!isFullySynced && isLiveIde && (
                      <span className="px-1.5 py-0.2 text-[9px] font-mono bg-[#001f3f] text-[#0070f3] border border-[#003366] rounded">
                        IDE ONLY
                      </span>
                    )}
                    {!isFullySynced && isLiveDesktop && (
                      <span className="px-1.5 py-0.2 text-[9px] font-mono bg-[#1d0d2b] text-[#a855f7] border border-[#3b1754] rounded">
                        DESKTOP ONLY
                      </span>
                    )}
                    {isCooldown && (
                      <span className="px-1.5 py-0.2 text-[9px] font-mono bg-[#201505] text-[#f59e0b] border border-[#442c0a] rounded">
                        COOLDOWN
                      </span>
                    )}
                  </div>
                  <p className="text-[11px] font-mono text-[#707070]">{acc.email}</p>
                </div>

                <button
                  onClick={() => handleSwitchAccount(acc, "both")}
                  disabled={(isFullySynced && isPrimary) || Boolean(switchingId)}
                  className={`h-7 px-3 text-xs font-medium rounded transition inline-flex items-center gap-1.5 font-mono cursor-pointer ${
                    isFullySynced && isPrimary
                      ? "bg-[#061e12] text-[#10b981] border border-[#0e4429] cursor-default"
                      : "bg-[#141414] hover:bg-[#1a1a1a] text-white border border-[#262626]"
                  }`}
                >
                  {isSwitching ? (
                    <RefreshCw className="h-3 w-3 animate-spin" />
                  ) : isFullySynced ? (
                    <CheckCircle2 className="h-3 w-3 text-[#10b981]" />
                  ) : (
                    <ArrowRight className="h-3 w-3" />
                  )}
                  <span>
                    {isFullySynced && isPrimary
                      ? "Active"
                      : "Sync Both"}
                  </span>
                </button>
              </div>

              {/* Quota Gauge Bars */}
              <div className="space-y-3 pt-3 border-t border-[#1e1e1e]">
                <div>
                  <div className="flex justify-between text-[11px] mb-1 font-mono">
                    <span className="text-[#707070]">Burst Quota (5-Hour Rolling)</span>
                    <span
                      className={
                        burstPct < 20 ? "text-[#ef4444]" : burstPct < 50 ? "text-[#f59e0b]" : "text-[#10b981]"
                      }
                    >
                      {burstPct.toFixed(1)}% remaining
                    </span>
                  </div>
                  <div className="w-full h-1 bg-[#1a1a1a] rounded-full overflow-hidden">
                    <div
                      className={`h-full ${
                        burstPct < 20 ? "bg-[#ef4444]" : burstPct < 50 ? "bg-[#f59e0b]" : "bg-[#10b981]"
                      }`}
                      style={{ width: `${Math.min(100, Math.max(0, burstPct))}%` }}
                    />
                  </div>
                </div>

                <div>
                  <div className="flex justify-between text-[11px] mb-1 font-mono">
                    <span className="text-[#707070]">Weekly Quota Limit</span>
                    <span className="text-[#a1a1a1]">{weeklyPct.toFixed(1)}% remaining</span>
                  </div>
                  <div className="w-full h-1 bg-[#1a1a1a] rounded-full overflow-hidden">
                    <div
                      className="h-full bg-[#0070f3]"
                      style={{ width: `${Math.min(100, Math.max(0, weeklyPct))}%` }}
                    />
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
