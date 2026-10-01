"use client";

import React from "react";
import { Database, Server } from "lucide-react";

export default function InfraPage() {
  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <div className="pb-6 border-b border-[#1e1e1e]">
        <h1 className="text-2xl font-bold tracking-tight text-white">System Infrastructure</h1>
        <p className="text-xs text-[#a1a1a1] mt-1 font-mono">
          Host topology, OrbStack container runtime, and backend supervisor specifications.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Node 1: PostgreSQL */}
        <div className="vercel-card p-5 space-y-3 font-mono text-xs">
          <div className="flex items-center justify-between pb-3 border-b border-[#1e1e1e]">
            <div className="flex items-center gap-2">
              <Database className="h-4 w-4 text-[#0070f3]" />
              <span className="font-semibold text-white">PostgreSQL (OrbStack)</span>
            </div>
            <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429]">
              CONNECTED
            </span>
          </div>

          <div className="space-y-2 text-[#a1a1a1]">
            <div>
              <span className="text-[#707070] text-[10px] uppercase font-bold block">Container</span>
              <p className="text-white mt-0.5">dev-postgres (postgres:17-alpine)</p>
            </div>
            <div>
              <span className="text-[#707070] text-[10px] uppercase font-bold block">Connection URL</span>
              <p className="text-[#0070f3] mt-0.5 break-all">postgres://dev:password@127.0.0.1:5432/antigravity_harness</p>
            </div>
            <div>
              <span className="text-[#707070] text-[10px] uppercase font-bold block">Tables</span>
              <p className="text-[#707070] mt-0.5">
                accounts, account_quotas, virtual_api_keys, request_logs, system_settings
              </p>
            </div>
          </div>
        </div>

        {/* Node 2: Go Gateway Daemon */}
        <div className="vercel-card p-5 space-y-3 font-mono text-xs">
          <div className="flex items-center justify-between pb-3 border-b border-[#1e1e1e]">
            <div className="flex items-center gap-2">
              <Server className="h-4 w-4 text-[#f59e0b]" />
              <span className="font-semibold text-white">Go Gateway Daemon</span>
            </div>
            <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429]">
              LISTENING
            </span>
          </div>

          <div className="space-y-2 text-[#a1a1a1]">
            <div>
              <span className="text-[#707070] text-[10px] uppercase font-bold block">Architecture</span>
              <p className="text-white mt-0.5">go1.26.2 darwin/arm64 (Mach-O 64-bit)</p>
            </div>
            <div>
              <span className="text-[#707070] text-[10px] uppercase font-bold block">Listening Port</span>
              <p className="text-[#f59e0b] mt-0.5">http://0.0.0.0:8050</p>
            </div>
            <div>
              <span className="text-[#707070] text-[10px] uppercase font-bold block">Supervisor Protocol</span>
              <p className="text-[#707070] mt-0.5">Connect RPC HTTPS + Antigravity IDE Unix Pipe</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
