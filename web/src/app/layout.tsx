import type { Metadata } from "next";

import { TopNav } from "@/components/top-nav";

import "./globals.css";

export const metadata: Metadata = {
  title: "YAMDC WebUI",
  description: "Scan, review and manage scrape jobs.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <body>
        <div className="shell">
          <header
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 16,
              marginBottom: 20,
            }}
          >
            <div>
              <div style={{ fontSize: 12, letterSpacing: "0.16em", color: "var(--muted)", textTransform: "uppercase" }}>
                YAMDC
              </div>
              <h1 style={{ margin: "6px 0 0", fontSize: 36 }}>Media Capture Console</h1>
            </div>
            <TopNav />
          </header>
          {children}
        </div>
      </body>
    </html>
  );
}
