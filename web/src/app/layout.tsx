import type { Metadata } from "next";
import Link from "next/link";

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
            <nav style={{ display: "flex", gap: 10 }}>
              <Link className="btn" href="/processing">
                待处理
              </Link>
              <Link className="btn" href="/review">
                待 Review
              </Link>
            </nav>
          </header>
          {children}
        </div>
      </body>
    </html>
  );
}

