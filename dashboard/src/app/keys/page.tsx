"use client";

import React from "react";
import { Plus, Trash2, Key } from "lucide-react";
import { useConsole } from "@/context/ConsoleContext";

export default function KeysPage() {
  const { virtualKeys, setShowKeyModal, handleDeleteKey } = useConsole();

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-6 border-b border-[#1e1e1e] gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Virtual API Keys</h1>
          <p className="text-xs text-[#a1a1a1] mt-1 font-mono">
            Issue drop-in OpenAI (`/v1/chat/completions`) & Gemini compatible keys with rate limiting.
          </p>
        </div>

        <button
          onClick={() => setShowKeyModal(true)}
          className="h-8 px-3 rounded-md bg-white hover:bg-[#eaeaea] text-black text-xs font-semibold inline-flex items-center gap-1.5 transition cursor-pointer shadow-sm self-start sm:self-auto"
        >
          <Plus className="h-3.5 w-3.5 stroke-[2.5]" />
          <span>New Virtual Key</span>
        </button>
      </div>

      <div className="vercel-card overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs border-collapse font-mono">
            <thead>
              <tr className="border-b border-[#1e1e1e] text-[#707070] uppercase text-[10px] bg-[#000000]">
                <th className="py-2.5 px-5 font-semibold">Key Identifier</th>
                <th className="py-2.5 px-5 font-semibold">Key Prefix</th>
                <th className="py-2.5 px-5 font-semibold">Rate Limits</th>
                <th className="py-2.5 px-5 font-semibold">Total Volume</th>
                <th className="py-2.5 px-5 font-semibold">Status</th>
                <th className="py-2.5 px-5 font-semibold text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#1e1e1e]">
              {virtualKeys.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-12 text-center text-[#707070]">
                    No virtual keys generated yet. Click "New Virtual Key" above.
                  </td>
                </tr>
              ) : (
                virtualKeys.map((k) => (
                  <tr key={k.id} className="hover:bg-[#0f0f0f] transition">
                    <td className="py-3 px-5 font-sans font-medium text-white">{k.name}</td>
                    <td className="py-3 px-5 text-[#0070f3]">{k.prefix}</td>
                    <td className="py-3 px-5 text-[#a1a1a1]">
                      {k.rate_limit_rpm} RPM &bull; {(k.rate_limit_tpm / 1000).toFixed(0)}k TPM
                    </td>
                    <td className="py-3 px-5 text-[#707070]">
                      {k.total_requests} reqs ({(k.total_tokens / 1000).toFixed(1)}k tokens)
                    </td>
                    <td className="py-3 px-5">
                      <span className="px-1.5 py-0.2 rounded text-[10px] font-bold bg-[#061e12] text-[#10b981] border border-[#0e4429]">
                        ACTIVE
                      </span>
                    </td>
                    <td className="py-3 px-5 text-right">
                      <button
                        onClick={() => {
                          if (confirm(`Revoke and delete virtual key "${k.name}"?`)) {
                            handleDeleteKey(k.id);
                          }
                        }}
                        title="Revoke and delete key"
                        className="p-1 rounded text-[#707070] hover:text-[#ef4444] transition cursor-pointer"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
