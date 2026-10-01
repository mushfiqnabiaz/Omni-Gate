"use client";

import React, { useState, useEffect } from "react";
import { ShieldCheck, Save, RefreshCw } from "lucide-react";
import { useConsole } from "@/context/ConsoleContext";

export default function ShieldPage() {
  const { setToast } = useConsole();
  const [enabled, setEnabled] = useState(true);
  const [burstThreshold, setBurstThreshold] = useState(20);
  const [weeklyThreshold, setWeeklyThreshold] = useState(20);
  const [autoContinue, setAutoContinue] = useState(true);
  const [webhookUrl, setWebhookUrl] = useState("");
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("/api/proxy/api/settings")
      .then((res) => res.json())
      .then((data) => {
        if (data.smart_shield_enabled !== undefined) setEnabled(Boolean(data.smart_shield_enabled));
        if (data.smart_shield_burst_threshold !== undefined) setBurstThreshold(Number(data.smart_shield_burst_threshold));
        if (data.smart_shield_weekly_threshold !== undefined) setWeeklyThreshold(Number(data.smart_shield_weekly_threshold));
        if (data.smart_shield_auto_continue !== undefined) setAutoContinue(Boolean(data.smart_shield_auto_continue));
        if (data.smart_shield_webhook_url !== undefined) setWebhookUrl(String(data.smart_shield_webhook_url));
      })
      .catch((err) => console.error("Error loading settings:", err))
      .finally(() => setLoading(false));
  }, []);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const res = await fetch("/api/proxy/api/settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          smart_shield_enabled: enabled,
          smart_shield_burst_threshold: burstThreshold,
          smart_shield_weekly_threshold: weeklyThreshold,
          smart_shield_auto_continue: autoContinue,
          smart_shield_webhook_url: webhookUrl,
        }),
      });
      if (res.ok) {
        setToast({ text: "Smart Shield policies updated in PostgreSQL.", type: "success" });
      } else {
        setToast({ text: "Failed to update policies.", type: "error" });
      }
    } catch (err: any) {
      setToast({ text: err.message || "Network error", type: "error" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex-1 flex flex-col p-6 space-y-6 max-w-7xl mx-auto w-full pb-10">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-6 border-b border-[#1e1e1e] gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-[#ededed] flex items-center gap-2">
            <ShieldCheck className="w-6 h-6 text-[#10b981]" />
            SmartShield 2.0 Engine
          </h1>
          <p className="text-xs text-[#888888] mt-1.5 font-mono">
            Autonomous account pool rotation, predictive quota preservation, and zero-stall handovers.
          </p>
        </div>

        <button
          onClick={handleSave}
          disabled={saving || loading}
          className="h-8 px-4 rounded-md bg-[#0070f3] hover:bg-[#0051a8] text-white text-xs font-semibold inline-flex items-center gap-2 transition cursor-pointer self-start sm:self-auto disabled:opacity-50"
        >
          {saving ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
          <span>{saving ? "Deploying..." : "Save Configuration"}</span>
        </button>
      </div>

      <form onSubmit={handleSave} className="space-y-6">
        <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-5 space-y-5 relative overflow-hidden">
          <div className="absolute top-0 right-0 w-32 h-32 bg-[#10b981]/5 rounded-bl-full pointer-events-none" />
          
          <div className="flex items-center justify-between pb-5 border-b border-[#1a1a1a]">
            <div>
              <span className="text-sm font-semibold text-[#ededed]">Autonomous Quota Rotation</span>
              <p className="text-xs text-[#888888] mt-0.5">
                Automatically rotates to next pooled account when remaining 5-hour quota falls below threshold.
              </p>
            </div>

            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="sr-only peer"
              />
              <div className="w-10 h-5 bg-[#222222] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-[#10b981]"></div>
            </label>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            <div className="p-4 rounded-lg bg-[#0e0e0e] border border-[#1e1e1e] space-y-3 hover:border-[#333333] transition">
              <div className="flex justify-between items-center text-xs">
                <span className="text-[#ededed] font-semibold">Burst Quota Threshold</span>
                <span className="font-mono text-[#f5a623] font-bold px-2 py-0.5 rounded bg-[#f5a623]/10 border border-[#f5a623]/20">{burstThreshold}%</span>
              </div>
              <p className="text-[11px] text-[#888888]">
                Execute zero-stall handover when remaining 5-hour burst quota &le; threshold.
              </p>
              <div className="pt-2">
                <input
                  type="range"
                  min="5"
                  max="50"
                  step="5"
                  value={burstThreshold}
                  onChange={(e) => setBurstThreshold(Number(e.target.value))}
                  className="w-full h-1 bg-[#222222] rounded appearance-none cursor-pointer accent-[#f5a623]"
                />
              </div>
            </div>

            <div className="p-4 rounded-lg bg-[#0e0e0e] border border-[#1e1e1e] space-y-3 hover:border-[#333333] transition">
              <div className="flex justify-between items-center text-xs">
                <span className="text-[#ededed] font-semibold">Weekly Quota Threshold</span>
                <span className="font-mono text-[#0070f3] font-bold px-2 py-0.5 rounded bg-[#0070f3]/10 border border-[#0070f3]/20">{weeklyThreshold}%</span>
              </div>
              <p className="text-[11px] text-[#888888]">
                Preserve headroom for interactive IDE use by rotating background tasks.
              </p>
              <div className="pt-2">
                <input
                  type="range"
                  min="5"
                  max="50"
                  step="5"
                  value={weeklyThreshold}
                  onChange={(e) => setWeeklyThreshold(Number(e.target.value))}
                  className="w-full h-1 bg-[#222222] rounded appearance-none cursor-pointer accent-[#0070f3]"
                />
              </div>
            </div>
          </div>

          <div className="p-4 rounded-lg bg-[#0e0e0e] border border-[#1e1e1e] flex items-center justify-between gap-4 hover:border-[#333333] transition">
            <div>
              <span className="text-xs font-semibold text-[#ededed]">Transparent Retries & Zero-Stall Handover</span>
              <p className="text-[11px] text-[#888888] mt-0.5">
                Automatically retry failed streaming chunks and instantly switch sessions on standby pooled accounts.
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer shrink-0">
              <input
                type="checkbox"
                checked={autoContinue}
                onChange={(e) => setAutoContinue(e.target.checked)}
                className="sr-only peer"
              />
              <div className="w-10 h-5 bg-[#222222] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-[#0070f3]"></div>
            </label>
          </div>
          
          <div className="p-4 rounded-lg bg-[#0e0e0e] border border-[#1e1e1e] space-y-3 hover:border-[#333333] transition">
            <div>
              <span className="text-xs font-semibold text-[#ededed]">Slack / Discord Webhook URL</span>
              <p className="text-[11px] text-[#888888] mt-0.5">
                Receive instant alerts for zero-stall handovers and daily telemetry summary reports.
              </p>
            </div>
            <input
              type="url"
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              placeholder="https://discord.com/api/webhooks/..."
              className="w-full px-3 py-2 rounded-md bg-[#111111] border border-[#333333] text-[#ededed] text-xs font-mono focus:outline-none focus:border-[#0070f3] transition"
            />
          </div>
        </div>

        {/* Failover Diagram */}
        <div className="bg-[#0a0a0a] border border-[#1e1e1e] rounded-lg p-5 space-y-4">
          <h2 className="text-[11px] font-mono font-semibold uppercase text-[#666666] tracking-wider">
            SmartShield Rotation Topology
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs font-mono">
            <div className="p-4 rounded-lg bg-[#0e0e0e] border border-[#1e1e1e] space-y-1.5 relative overflow-hidden group hover:border-[#0070f3]/50 transition">
              <div className="absolute top-0 right-0 w-16 h-16 bg-[#0070f3]/10 rounded-bl-full pointer-events-none" />
              <span className="text-[#ededed] font-bold block flex items-center gap-1.5">
                <div className="w-1.5 h-1.5 rounded-full bg-[#0070f3] animate-ping" />
                1. Active Primary
              </span>
              <p className="text-[11px] text-[#888888]">Live IDE & Desktop session handling user context.</p>
            </div>
            <div className="p-4 rounded-lg bg-[#0e0e0e] border border-[#1e1e1e] space-y-1.5 relative overflow-hidden group hover:border-[#10b981]/50 transition">
              <div className="absolute top-0 right-0 w-16 h-16 bg-[#10b981]/10 rounded-bl-full pointer-events-none" />
              <span className="text-[#10b981] font-bold block flex items-center gap-1.5">
                <div className="w-1.5 h-1.5 rounded-full bg-[#10b981]" />
                2. Standby Pool
              </span>
              <p className="text-[11px] text-[#888888]">Healthy accounts ready for immediate predictive burst failover.</p>
            </div>
            <div className="p-4 rounded-lg bg-[#0e0e0e] border border-[#1e1e1e] space-y-1.5 relative overflow-hidden group hover:border-[#f5a623]/50 transition">
              <div className="absolute top-0 right-0 w-16 h-16 bg-[#f5a623]/10 rounded-bl-full pointer-events-none" />
              <span className="text-[#f5a623] font-bold block flex items-center gap-1.5">
                <div className="w-1.5 h-1.5 rounded-full bg-[#f5a623]" />
                3. Cooldown Quarantine
              </span>
              <p className="text-[11px] text-[#888888]">Temporarily paused accounts recovering quota limits.</p>
            </div>
          </div>
        </div>
      </form>
    </div>
  );
}
