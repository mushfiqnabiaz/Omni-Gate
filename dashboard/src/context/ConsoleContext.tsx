"use client";

import React, { createContext, useContext, useState, useEffect } from "react";

export interface Quotas {
  weekly_pct: number;
  burst_5h_pct: number;
  claude_weekly_pct: number;
  claude_5h_pct: number;
}

export interface Account {
  id: string;
  provider: string;
  name: string;
  email: string;
  auth_type: string;
  plan_type: string;
  enabled: boolean;
  cooldown_until: number;
  cooldown_reason?: string;
  quotas?: Quotas;
}

export interface VirtualKey {
  id: string;
  name: string;
  prefix: string;
  rate_limit_rpm: number;
  rate_limit_tpm: number;
  total_requests: number;
  total_tokens: number;
  enabled: boolean;
  created_at: string;
}

export interface RequestLogPricing {
  prompt_rate_per_1m: number;
  completion_rate_per_1m: number;
  api_cost_usd: number;
  antigravity_cost_usd: number;
  savings_usd: number;
}

export interface RequestLog {
  req_id: string;
  provider: string;
  model: string;
  model_name?: string;
  account_name: string;
  account_email?: string;
  client_origin?: string;
  client_type?: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  duration_ms: number;
  status_code: number;
  error?: string | null;
  created_at: string;
  pricing?: RequestLogPricing;
}

export interface ModelComparison {
  model: string;
  name: string;
  family: string;
  calls: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  prompt_rate_per_1m: number;
  completion_rate_per_1m: number;
  commercial_cost_usd: number;
  antigravity_cost_usd: number;
  savings_usd: number;
}

export interface PricingSummary {
  total_calls: number;
  total_tokens: number;
  total_prompt_tokens: number;
  total_completion_tokens: number;
  commercial_api_cost_usd: number;
  antigravity_pro_cost_usd: number;
  total_savings_usd: number;
  savings_percentage: number;
  model_comparison: ModelComparison[];
}

export interface PaginationInfo {
  page: number;
  limit: number;
  total_records: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
}

export interface IDEStatus {
  pid: number;
  csrf: string;
  ports: number[];
  email: string;
  name: string;
  tier: string;
  app_type: string;
  connected: boolean;
  models_count?: number;
  quota_remaining_fraction?: number;
}

interface ConsoleContextType {
  accounts: Account[];
  activeSessionId: string;
  ideStatus: IDEStatus | null;
  desktopStatus: IDEStatus | null;
  virtualKeys: VirtualKey[];
  requestLogs: RequestLog[];
  pricingSummary: PricingSummary | null;
  pagination: PaginationInfo | null;
  loading: boolean;
  switchingId: string | null;
  toast: { text: string; type: "success" | "error" } | null;
  setToast: (t: { text: string; type: "success" | "error" } | null) => void;
  fetchFleet: () => Promise<void>;
  fetchKeys: () => Promise<void>;
  fetchLogs: (params?: { page?: number; limit?: number; client?: string; search?: string; model?: string }) => Promise<void>;
  handleSwitchAccount: (acc: Account, target?: "ide" | "desktop" | "both") => Promise<void>;
  showKeyModal: boolean;
  setShowKeyModal: (show: boolean) => void;
  newKeyName: string;
  setNewKeyName: (name: string) => void;
  newKeyRpm: number;
  setNewKeyRpm: (rpm: number) => void;
  createdKeySecret: string | null;
  setCreatedKeySecret: (secret: string | null) => void;
  handleCreateKey: (e: React.FormEvent) => Promise<void>;
  handleDeleteKey: (id: string) => Promise<void>;
}

const ConsoleContext = createContext<ConsoleContextType | null>(null);

export function ConsoleProvider({ children }: { children: React.ReactNode }) {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string>("");
  const [ideStatus, setIdeStatus] = useState<IDEStatus | null>(null);
  const [desktopStatus, setDesktopStatus] = useState<IDEStatus | null>(null);
  const [virtualKeys, setVirtualKeys] = useState<VirtualKey[]>([]);
  const [requestLogs, setRequestLogs] = useState<RequestLog[]>([]);
  const [pricingSummary, setPricingSummary] = useState<PricingSummary | null>(null);
  const [pagination, setPagination] = useState<PaginationInfo | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [switchingId, setSwitchingId] = useState<string | null>(null);
  const [toast, setToast] = useState<{ text: string; type: "success" | "error" } | null>(null);

  // Key modal state
  const [showKeyModal, setShowKeyModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [newKeyRpm, setNewKeyRpm] = useState(60);
  const [createdKeySecret, setCreatedKeySecret] = useState<string | null>(null);

  const fetchFleet = async () => {
    try {
      const res = await fetch("/api/proxy/api/fleet");
      if (res.ok) {
        const data = await res.json();
        setAccounts(data.accounts || []);
        setActiveSessionId(data.active_session_account_id || "");
        if (data.ide_status) {
          setIdeStatus(data.ide_status);
        }
        if (data.desktop_status) {
          setDesktopStatus(data.desktop_status);
        }
      }
    } catch (err) {
      console.error("Fleet fetch error:", err);
    }
  };

  const fetchKeys = async () => {
    try {
      const res = await fetch("/api/proxy/api/keys");
      if (res.ok) {
        const data = await res.json();
        setVirtualKeys(data.keys || []);
      }
    } catch (err) {
      console.error("Keys fetch error:", err);
    }
  };

  const fetchLogs = async (params?: { page?: number; limit?: number; client?: string; search?: string; model?: string }) => {
    try {
      const q = new URLSearchParams();
      if (params?.page) q.set("page", params.page.toString());
      if (params?.limit) q.set("limit", params.limit.toString());
      else q.set("limit", "10"); // Default 10 data as requested!
      if (params?.client && params.client !== "all") q.set("client", params.client);
      if (params?.search) q.set("search", params.search);
      if (params?.model && params.model !== "all") q.set("model", params.model);

      const res = await fetch(`/api/proxy/api/logs?${q.toString()}`);
      if (res.ok) {
        const data = await res.json();
        setRequestLogs(data.logs || []);
        if (data.pricing_summary) {
          setPricingSummary(data.pricing_summary);
        }
        if (data.pagination) {
          setPagination(data.pagination);
        }
      }
    } catch (err) {
      console.error("Logs fetch error:", err);
    }
  };

  useEffect(() => {
    setLoading(true);
    Promise.all([fetchFleet(), fetchKeys(), fetchLogs()]).finally(() => setLoading(false));

    // Fast 1.5s interval for realtime call streaming
    const logInterval = setInterval(() => {
      fetchLogs();
    }, 1500);

    // 5s interval for fleet & keys
    const fleetInterval = setInterval(() => {
      fetchFleet();
      fetchKeys();
    }, 5000);

    return () => {
      clearInterval(logInterval);
      clearInterval(fleetInterval);
    };
  }, []);

  const handleSwitchAccount = async (account: Account, target: "ide" | "desktop" | "both" = "both") => {
    const switchKey = `${account.id}-${target}`;
    setSwitchingId(switchKey);
    try {
      const res = await fetch("/api/proxy/api/set-active-account", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ account_id: account.id, target, force: false }),
      });
      const data = await res.json();
      if (res.ok) {
        setActiveSessionId(account.id);
        const targetLabel =
          target === "desktop"
            ? "Antigravity Desktop"
            : target === "ide"
            ? "Antigravity IDE"
            : "Antigravity IDE & Desktop";
        setToast({
          text: `${targetLabel} switched to ${account.email}. macOS Keychain synchronized.`,
          type: "success",
        });
        await fetchFleet();
      } else {
        setToast({ text: data.error || "Failed to switch account", type: "error" });
      }
    } catch (err: any) {
      setToast({ text: err.message || "Failed to switch account", type: "error" });
    } finally {
      setSwitchingId(null);
    }
  };

  const handleCreateKey = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await fetch("/api/proxy/api/keys", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: newKeyName || "Production Key", rate_limit_rpm: newKeyRpm }),
      });
      const data = await res.json();
      if (res.ok) {
        setCreatedKeySecret(data.api_key);
        fetchKeys();
      }
    } catch (err) {
      console.error("Create key error:", err);
    }
  };

  const handleDeleteKey = async (id: string) => {
    try {
      const res = await fetch(`/api/proxy/api/keys/${id}`, {
        method: "DELETE",
      });
      if (res.ok) {
        setToast({ text: "Virtual API key deleted and revoked.", type: "success" });
        fetchKeys();
      } else {
        setToast({ text: "Failed to delete key", type: "error" });
      }
    } catch (err: any) {
      setToast({ text: err.message || "Failed to delete key", type: "error" });
    }
  };

  return (
    <ConsoleContext.Provider
      value={{
        accounts,
        activeSessionId,
        ideStatus,
        desktopStatus,
        virtualKeys,
        requestLogs,
        pricingSummary,
        pagination,
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
        handleDeleteKey,
      }}
    >
      {children}
    </ConsoleContext.Provider>
  );
}

export function useConsole() {
  const ctx = useContext(ConsoleContext);
  if (!ctx) {
    throw new Error("useConsole must be used within a ConsoleProvider");
  }
  return ctx;
}
