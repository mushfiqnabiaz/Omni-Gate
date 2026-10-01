import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { ConsoleProvider } from "@/context/ConsoleContext";
import { ConsoleShell } from "@/components/ConsoleShell";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "OmniGate • Universal AI Gateway & Fleet Harness",
  description: "Universal Multi-Provider AI Gateway for Antigravity, Claude, ChatGPT & Codex with Auto-Failover & Fleet Pooling",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}>
      <body className="min-h-full flex flex-col bg-[#000000] text-[#f5f5f5]">
        <ConsoleProvider>
          <ConsoleShell>{children}</ConsoleShell>
        </ConsoleProvider>
      </body>
    </html>
  );
}
