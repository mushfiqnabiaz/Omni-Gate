"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Search,
  ChevronDown,
  Plus,
  RefreshCw,
  LayoutDashboard,
  Play,
  Key,
  Activity,
  Sparkles,
  ShieldCheck,
  HardDrive,
  Check,
  CheckCircle2,
  Copy,
  Laptop,
  Monitor,
  AlertTriangle,
  X,
  ArrowRight,
  BarChart3,
} from "lucide-react";
import { useConsole } from "@/context/ConsoleContext";

export function ConsoleShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const {
    accounts,
    activeSessionId,
    ideStatus,
    desktopStatus,
    virtualKeys,
    loading,
    switchingId,
    toast,
    setToast,
    fetchFleet,
    fetchKeys,
    fetchLogs,
    handleSwitchAccount,
    showKeyModal,
    setShowKeyModal,
    newKeyName,
    setNewKeyName,
    newKeyRpm,
    setNewKeyRpm,
    createdKeySecret,
    setCreatedKeySecret,
    handleCreateKey,
  } = useConsole();

  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [showSwitcherDropdown, setShowSwitcherDropdown] = useState(false);
  const [copiedKey, setCopiedKey] = useState(false);

  const googleAccounts = accounts.filter((a) => a.provider === "google");
  const activeAccount = accounts.find((a) => a.id === activeSessionId) || googleAccounts[0];

  const ideEmail = ideStatus?.connected && ideStatus.email ? ideStatus.email.toLowerCase() : "";
  const desktopEmail = desktopStatus?.connected && desktopStatus.email ? desktopStatus.email.toLowerCase() : "";
  const isClientsMatched = Boolean(ideEmail && desktopEmail && ideEmail === desktopEmail);

  // Global ⌘K keyboard shortcut
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setSearchOpen((prev) => !prev);
      }
      if (e.key === "Escape") {
        setSearchOpen(false);
        setShowSwitcherDropdown(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const isActive = (path: string) => {
    if (path === "/overview") {
      return pathname === "/" || pathname === "/overview";
    }
    return pathname.startsWith(path);
  };

  const navItems = [
    { label: "Overview", href: "/overview", icon: LayoutDashboard },
    { label: "Antigravity Fleet", href: "/fleet/google", icon: Sparkles, count: googleAccounts.length },
    { label: "Virtual Keys", href: "/keys", icon: Key, count: virtualKeys.length },
    { label: "AI Playground", href: "/playground", icon: Play },
    { label: "Call Logger & Pricing", href: "/logs", icon: Activity, isLive: true },
    { label: "Analytics & ROI", href: "/analytics", icon: BarChart3 },
    { label: "Smart Shield", href: "/shield", icon: ShieldCheck },
    { label: "Infrastructure", href: "/infra", icon: HardDrive },
  ];

  return (
    <div className="min-h-screen bg-[#000000] text-[#ededed] font-sans antialiased flex flex-col selection:bg-[#0070f3]/30">
      {/* 1. TOP HEADER (Vercel / Cloudflare Style) */}
      <header className="h-[52px] bg-[#000000] border-b border-[#1e1e1e] px-4 flex items-center justify-between sticky top-0 z-40 select-none">
        {/* Left: Organization / App Scope */}
        <div className="flex items-center gap-3">
          <Link href="/overview" className="flex items-center gap-2.5 hover:opacity-90 transition">
            {/* Vercel-style geometric mark */}
            <div className="w-5 h-5 bg-white text-black flex items-center justify-center rounded-sm font-black text-[11px] leading-none">
              ▲
            </div>
            <span className="font-semibold text-sm tracking-tight text-white">OmniGate</span>
          </Link>
          <span className="text-[#383838]">/</span>
          <div className="flex items-center gap-1.5 text-xs text-[#a1a1a1] font-mono">
            <span>antigravity-fleet</span>
            <span className="px-1.5 py-0.2 rounded text-[10px] bg-[#141414] text-[#707070] border border-[#262626]">
              PRO
            </span>
          </div>
        </div>

        {/* Center: Search Command Bar */}
        <div className="hidden md:flex items-center">
          <button
            onClick={() => setSearchOpen(true)}
            className="flex items-center justify-between w-64 h-[30px] px-2.5 rounded-md bg-[#0a0a0a] hover:bg-[#111111] border border-[#222222] hover:border-[#333333] text-xs text-[#707070] hover:text-[#a1a1a1] transition font-mono"
          >
            <span className="flex items-center gap-2">
              <Search className="h-3 w-3 text-[#555555]" />
              <span>Search console...</span>
            </span>
            <kbd className="px-1 py-0.2 text-[9px] bg-[#161616] border border-[#2a2a2a] rounded text-[#666666]">
              ⌘K
            </kbd>
          </button>
        </div>

        {/* Right: Operational Status & Controls */}
        <div className="flex items-center gap-2.5">
          {/* Real-time Co-Sync Status Indicator */}
          <div className="hidden sm:flex items-center gap-2 px-2.5 py-1 rounded-md bg-[#0a0a0a] border border-[#1e1e1e] text-[11px] font-mono">
            <span className={`w-1.5 h-1.5 rounded-full ${isClientsMatched ? "bg-[#10b981]" : "bg-[#f59e0b]"}`} />
            <span className="text-[#a1a1a1]">
              {isClientsMatched ? "Clients Synced" : "Unsynced"}
            </span>
            <span className="text-[#383838]">•</span>
            <span className="text-[#0070f3] font-medium truncate max-w-[130px]">
              {ideStatus?.email?.split("@")[0] || activeAccount?.email?.split("@")[0]}
            </span>
          </div>

          {/* Quick Active Account Switcher */}
          <div className="relative">
            <button
              onClick={() => setShowSwitcherDropdown(!showSwitcherDropdown)}
              className="h-[30px] px-2.5 rounded-md bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-xs text-white font-mono flex items-center gap-1.5 transition"
            >
              <span className="w-1.5 h-1.5 rounded-full bg-[#10b981]"></span>
              <span className="truncate max-w-[110px]">{activeAccount?.email?.split("@")[0]}</span>
              <ChevronDown className="h-3 w-3 text-[#707070]" />
            </button>

            {showSwitcherDropdown && (
              <div className="absolute right-0 mt-1 w-72 bg-[#0a0a0a] border border-[#222222] rounded-md shadow-2xl p-1 z-50 space-y-0.5">
                <div className="px-2.5 py-1.5 text-[10px] uppercase font-mono text-[#707070] font-semibold border-b border-[#1a1a1a]">
                  Switch IDE & Desktop Account
                </div>
                {googleAccounts.map((acc) => {
                  const isSelected = acc.id === activeSessionId;
                  const burst = acc.quotas?.burst_5h_pct ?? 100;
                  return (
                    <button
                      key={acc.id}
                      onClick={() => {
                        handleSwitchAccount(acc, "both");
                        setShowSwitcherDropdown(false);
                      }}
                      disabled={switchingId !== null}
                      className={`w-full text-left px-2.5 py-1.5 rounded text-xs flex items-center justify-between transition ${
                        isSelected
                          ? "bg-[#141414] text-[#0070f3] font-medium"
                          : "hover:bg-[#121212] text-[#a1a1a1] hover:text-white"
                      }`}
                    >
                      <div className="truncate pr-2">
                        <p className="font-sans text-xs text-white truncate">{acc.name}</p>
                        <p className="font-mono text-[10px] text-[#707070] truncate">{acc.email}</p>
                      </div>
                      <span className="text-[10px] font-mono text-[#10b981]">{burst.toFixed(0)}%</span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {/* New Virtual Key Button */}
          <button
            onClick={() => setShowKeyModal(true)}
            className="h-[30px] px-3 rounded-md bg-white hover:bg-[#eaeaea] text-black text-xs font-semibold inline-flex items-center gap-1.5 transition cursor-pointer shadow-sm"
          >
            <Plus className="h-3 w-3 stroke-[2.5]" />
            <span>New Key</span>
          </button>

          {/* Refresh Fleet State */}
          <button
            onClick={() => {
              fetchFleet();
              fetchKeys();
              fetchLogs();
            }}
            className="h-[30px] w-[30px] rounded-md bg-[#0a0a0a] hover:bg-[#141414] border border-[#222222] text-[#707070] hover:text-white transition flex items-center justify-center cursor-pointer"
            title="Refresh Fleet State"
          >
            <RefreshCw className={`h-3 w-3 ${loading ? "animate-spin text-[#0070f3]" : ""}`} />
          </button>
        </div>
      </header>

      {/* 2. BODY LAYOUT */}
      <div className="flex-1 flex overflow-hidden">
        {/* LEFT SIDEBAR (Clean Vercel/Cloudflare Zero Trust Style) */}
        <aside className="w-60 bg-[#000000] border-r border-[#1e1e1e] flex flex-col justify-between p-3 select-none shrink-0">
          <div className="space-y-4">
            <div className="space-y-0.5">
              <span className="text-[10px] font-mono font-semibold uppercase tracking-wider text-[#666666] px-2.5 block mb-1">
                Navigation
              </span>
              {navItems.map((item) => {
                const Icon = item.icon;
                const active = isActive(item.href);
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`w-full h-8 px-2.5 rounded-md text-xs font-medium flex items-center justify-between transition ${
                      active
                        ? "bg-[#141414] text-white border border-[#262626]"
                        : "text-[#a1a1a1] hover:text-white hover:bg-[#0f0f0f]"
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <Icon className={`h-3.5 w-3.5 ${active ? "text-[#0070f3]" : "text-[#707070]"}`} />
                      <span>{item.label}</span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      {item.isLive && (
                        <span className="flex h-2 w-2 relative">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-[#10b981] opacity-75"></span>
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-[#10b981]"></span>
                        </span>
                      )}
                      {item.count !== undefined && (
                        <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-[#161616] text-[#707070] border border-[#222222]">
                          {item.count}
                        </span>
                      )}
                    </div>
                  </Link>
                );
              })}
            </div>
          </div>

          {/* Bottom Sidebar: Dual Client Status Card */}
          <div className="p-3 rounded-md bg-[#0a0a0a] border border-[#1e1e1e] space-y-2">
            <div className="flex items-center justify-between text-[10px] font-mono">
              <span className="text-[#707070] uppercase font-semibold">Client Co-Sync</span>
              <span className={isClientsMatched ? "text-[#10b981]" : "text-[#f59e0b]"}>
                {isClientsMatched ? "SYNCED" : "MISMATCH"}
              </span>
            </div>
            <div className="space-y-1 text-xs">
              <div className="flex justify-between text-[11px] text-[#707070]">
                <span>IDE</span>
                <span className="font-mono text-white">
                  {ideStatus?.connected ? `PID ${ideStatus.pid}` : "Offline"}
                </span>
              </div>
              <div className="flex justify-between text-[11px] text-[#707070]">
                <span>Desktop</span>
                <span className="font-mono text-white">
                  {desktopStatus?.connected ? `PID ${desktopStatus.pid}` : "Offline"}
                </span>
              </div>
            </div>
            <div className="pt-2 border-t border-[#1a1a1a] flex items-center justify-between text-[10px]">
              <span className="text-[#707070] font-mono truncate max-w-[110px]">
                {activeAccount?.email?.split("@")[0]}
              </span>
              <button
                onClick={() => activeAccount && handleSwitchAccount(activeAccount, "both")}
                disabled={Boolean(switchingId)}
                className="text-[#0070f3] hover:underline font-mono cursor-pointer"
              >
                {switchingId ? "Syncing..." : "Sync Both"}
              </button>
            </div>
          </div>
        </aside>

        {/* MAIN VIEWPORT */}
        <main className="flex-1 overflow-y-auto p-8 space-y-6">
          {/* Toast Notification */}
          {toast && (
            <div
              className={`p-3 rounded-md border flex items-center justify-between text-xs ${
                toast.type === "success"
                  ? "bg-[#061e12] border-[#0e4429] text-[#10b981]"
                  : "bg-[#200a0a] border-[#441111] text-[#ef4444]"
              }`}
            >
              <div className="flex items-center gap-2">
                <CheckCircle2 className="h-4 w-4 shrink-0" />
                <span>{toast.text}</span>
              </div>
              <button
                onClick={() => setToast(null)}
                className="text-[#707070] hover:text-white p-1"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          )}

          {children}
        </main>
      </div>

      {/* ⌘K Command Modal */}
      {searchOpen && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-xs flex items-start justify-center pt-24 p-4">
          <div className="bg-[#0a0a0a] border border-[#262626] rounded-lg max-w-lg w-full overflow-hidden shadow-2xl space-y-1">
            <div className="flex items-center gap-3 px-3.5 py-2.5 border-b border-[#1e1e1e]">
              <Search className="h-3.5 w-3.5 text-[#707070]" />
              <input
                type="text"
                autoFocus
                placeholder="Search console routes..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="flex-1 bg-transparent text-xs text-white placeholder-[#555555] focus:outline-none font-mono"
              />
              <kbd className="px-1 py-0.2 text-[9px] bg-[#1a1a1a] text-[#707070] rounded border border-[#2a2a2a]">
                ESC
              </kbd>
            </div>
            <div className="p-1 max-h-72 overflow-y-auto space-y-0.5">
              {navItems.map((item, idx) => {
                const Icon = item.icon;
                return (
                  <button
                    key={idx}
                    onClick={() => {
                      router.push(item.href);
                      setSearchOpen(false);
                    }}
                    className="w-full text-left px-3 py-2 rounded hover:bg-[#141414] flex items-center justify-between text-xs transition cursor-pointer"
                  >
                    <div className="flex items-center gap-2.5 text-white">
                      <Icon className="h-3.5 w-3.5 text-[#707070]" />
                      <span>{item.label}</span>
                    </div>
                    <ArrowRight className="h-3 w-3 text-[#444444]" />
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      )}

      {/* Virtual Key Modal */}
      {showKeyModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-xs flex items-center justify-center p-4">
          <div className="bg-[#0a0a0a] border border-[#262626] rounded-lg max-w-md w-full p-5 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between pb-3 border-b border-[#1e1e1e]">
              <span className="text-sm font-semibold text-white">Generate Virtual API Key</span>
              <button
                onClick={() => {
                  setShowKeyModal(false);
                  setCreatedKeySecret(null);
                }}
                className="text-[#707070] hover:text-white text-xs font-mono"
              >
                ESC
              </button>
            </div>

            {!createdKeySecret ? (
              <form onSubmit={handleCreateKey} className="space-y-3.5">
                <div>
                  <label className="block text-[10px] font-mono text-[#707070] mb-1 uppercase font-semibold">
                    Key Identifier Name
                  </label>
                  <input
                    type="text"
                    required
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    placeholder="e.g. Cursor Pro, Next.js Web App"
                    className="w-full bg-[#111111] border border-[#262626] rounded px-3 py-1.5 text-xs text-white focus:outline-none focus:border-[#0070f3] font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[10px] font-mono text-[#707070] mb-1 uppercase font-semibold">
                    Rate Limit (Requests / Min)
                  </label>
                  <input
                    type="number"
                    value={newKeyRpm}
                    onChange={(e) => setNewKeyRpm(Number(e.target.value))}
                    className="w-full bg-[#111111] border border-[#262626] rounded px-3 py-1.5 text-xs text-white focus:outline-none focus:border-[#0070f3] font-mono"
                  />
                </div>
                <div className="flex justify-end gap-2 pt-2">
                  <button
                    type="button"
                    onClick={() => setShowKeyModal(false)}
                    className="h-[30px] px-3 text-xs text-[#707070] hover:text-white cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="h-[30px] px-3.5 bg-white hover:bg-[#eaeaea] text-black text-xs font-medium rounded transition cursor-pointer"
                  >
                    Issue Key
                  </button>
                </div>
              </form>
            ) : (
              <div className="space-y-3">
                <div className="p-2.5 rounded bg-[#1c1507] border border-[#443007] text-[#f5a623] text-xs">
                  Copy this key now. Plaintext key cannot be shown again!
                </div>
                <div className="p-2.5 rounded bg-[#111111] border border-[#222222] font-mono text-xs text-[#0070f3] break-all select-all flex items-center justify-between">
                  <span>{createdKeySecret}</span>
                  <button
                    onClick={() => {
                      navigator.clipboard.writeText(createdKeySecret);
                      setCopiedKey(true);
                      setTimeout(() => setCopiedKey(false), 2000);
                    }}
                    className="ml-2 p-1 text-[#707070] hover:text-white shrink-0 cursor-pointer"
                    title="Copy Key"
                  >
                    <Copy className="h-3.5 w-3.5" />
                  </button>
                </div>
                <button
                  onClick={() => {
                    setCreatedKeySecret(null);
                    setShowKeyModal(false);
                  }}
                  className="w-full h-[30px] rounded bg-white text-black text-xs font-medium hover:bg-[#eaeaea] transition cursor-pointer"
                >
                  {copiedKey ? "Copied!" : "Done"}
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
