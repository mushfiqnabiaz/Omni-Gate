"use client";

import React, { useState } from "react";
import { Play, RefreshCw, Copy, Check, Terminal } from "lucide-react";
import { useConsole } from "@/context/ConsoleContext";

export default function PlaygroundPage() {
  const { fetchLogs } = useConsole();
  const [selectedModel, setSelectedModel] = useState("gemini-2.5-flash");
  const [promptText, setPromptText] = useState("Explain how OmniGate synchronizes Antigravity IDE and Desktop in two bullet points.");
  const [playgroundOutput, setPlaygroundOutput] = useState("");
  const [playgroundLoading, setPlaygroundLoading] = useState(false);
  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const [copiedResponse, setCopiedResponse] = useState(false);

  const handlePlaygroundSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setPlaygroundLoading(true);
    setPlaygroundOutput("");
    setLatencyMs(null);

    const startTime = performance.now();
    try {
      const res = await fetch("/api/proxy/v1/chat/completions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          model: selectedModel,
          stream: true,
          messages: [{ role: "user", content: promptText }],
        }),
      });

      if (!res.ok) {
        const errJson = await res.json().catch(() => ({}));
        const errStr = typeof errJson.error === 'object' ? errJson.error.message : errJson.error;
        setPlaygroundOutput(`HTTP ${res.status}: ${errStr || "Failed to complete stream"}`);
        setPlaygroundLoading(false);
        return;
      }

      const reader = res.body?.getReader();
      const decoder = new TextDecoder();
      let output = "";

      if (reader) {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          const chunk = decoder.decode(value);
          const lines = chunk.split("\n");
          for (const line of lines) {
            if (line.startsWith("data:") && !line.includes("[DONE]")) {
              try {
                const data = JSON.parse(line.replace("data:", "").trim());
                const delta = data.choices?.[0]?.delta?.content || "";
                output += delta;
                setPlaygroundOutput(output);
              } catch {}
            }
          }
        }
      }
      setLatencyMs(Math.round(performance.now() - startTime));
      fetchLogs();
    } catch (err: any) {
      setPlaygroundOutput(`Request error: ${err.message}`);
    } finally {
      setPlaygroundLoading(false);
    }
  };

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-6 border-b border-[#1e1e1e] gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white">AI Playground</h1>
          <p className="text-xs text-[#a1a1a1] mt-1 font-mono">
            Test completions directly against Google Cloud Code PaLM models via OmniGate proxy.
          </p>
        </div>

        {latencyMs !== null && (
          <div className="px-2.5 py-1 rounded bg-[#061e12] border border-[#0e4429] text-[#10b981] font-mono text-xs">
            Latency: {latencyMs}ms
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Input Card */}
        <div className="vercel-card p-5 space-y-4">
          <form onSubmit={handlePlaygroundSubmit} className="space-y-4">
            <div>
              <label className="block text-[10px] font-mono text-[#707070] mb-1 uppercase font-semibold">
                Model Engine
              </label>
              <select
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
                className="w-full bg-[#111111] border border-[#222222] rounded px-3 py-1.5 text-xs text-white focus:outline-none focus:border-[#0070f3] font-mono"
              >
                <option value="gemini-3.8-flash">gemini-3.8-flash (Ultra Fast • PaLM)</option>
                <option value="gemini-2.5-pro">gemini-2.5-pro (High Reasoning)</option>
                <option value="gemini-2.5-flash">gemini-2.5-flash (Standard)</option>
                <option value="gemini-1.5-pro">gemini-1.5-pro (Extended Context)</option>
              </select>
            </div>

            <div>
              <label className="block text-[10px] font-mono text-[#707070] mb-1 uppercase font-semibold">
                Prompt
              </label>
              <textarea
                rows={7}
                value={promptText}
                onChange={(e) => setPromptText(e.target.value)}
                className="w-full bg-[#111111] border border-[#222222] rounded p-3 text-xs text-white focus:outline-none focus:border-[#0070f3] font-mono resize-none leading-relaxed"
              />
            </div>

            <div className="flex items-center justify-between pt-1">
              <span className="text-[11px] font-mono text-[#707070]">
                Stream via Server-Sent Events (SSE)
              </span>
              <button
                type="submit"
                disabled={playgroundLoading}
                className="h-[30px] px-3.5 rounded bg-white hover:bg-[#eaeaea] text-black text-xs font-semibold inline-flex items-center gap-1.5 transition cursor-pointer"
              >
                {playgroundLoading ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Play className="h-3 w-3 fill-black" />
                )}
                <span>{playgroundLoading ? "Streaming..." : "Dispatch"}</span>
              </button>
            </div>
          </form>
        </div>

        {/* Output Card */}
        <div className="vercel-card p-5 flex flex-col justify-between">
          <div className="space-y-3">
            <div className="flex items-center justify-between pb-3 border-b border-[#1e1e1e]">
              <span className="text-[10px] font-mono uppercase font-semibold text-[#707070]">
                Stream Output Terminal
              </span>
              {playgroundOutput && (
                <button
                  onClick={() => {
                    navigator.clipboard.writeText(playgroundOutput);
                    setCopiedResponse(true);
                    setTimeout(() => setCopiedResponse(false), 2000);
                  }}
                  className="text-[#707070] hover:text-white text-xs font-mono inline-flex items-center gap-1 cursor-pointer"
                >
                  {copiedResponse ? <Check className="h-3 w-3 text-[#10b981]" /> : <Copy className="h-3 w-3" />}
                  <span>{copiedResponse ? "Copied" : "Copy"}</span>
                </button>
              )}
            </div>

            <div className="min-h-[220px] max-h-[340px] overflow-y-auto font-mono text-xs text-[#ededed] whitespace-pre-wrap leading-relaxed">
              {playgroundOutput ? (
                playgroundOutput
              ) : (
                <div className="flex flex-col items-center justify-center h-48 text-[#555555] space-y-2">
                  <Terminal className="h-5 w-5" />
                  <span>Ready for dispatch. Click "Dispatch" to stream response.</span>
                </div>
              )}
            </div>
          </div>

          <div className="pt-3 border-t border-[#1e1e1e] flex items-center justify-between text-[11px] font-mono text-[#707070]">
            <span>Model: {selectedModel}</span>
            <span>Gateway :8050</span>
          </div>
        </div>
      </div>
    </div>
  );
}
