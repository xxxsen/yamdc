"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { useState } from "react";

import { TopNav } from "@/components/top-nav";

export function AppShell({
  children,
}: Readonly<{
  children: ReactNode;
}>) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div className={`app-shell ${collapsed ? "app-shell-collapsed" : ""}`}>
      {!collapsed ? (
        <aside className="panel sidebar">
          <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12 }}>
            <div>
              <div style={{ fontSize: 12, letterSpacing: "0.16em", color: "var(--muted)", textTransform: "uppercase" }}>
                YAMDC
              </div>
              <h1 style={{ margin: "8px 0 0", fontSize: 28, lineHeight: 1.05 }}>Media Capture</h1>
            </div>
          </div>
          <TopNav />
          <button
            className="sidebar-edge-toggle sidebar-edge-toggle-close"
            onClick={() => setCollapsed(true)}
            aria-label="折叠侧边栏"
            type="button"
          >
            <ChevronLeft size={16} />
          </button>
        </aside>
      ) : null}
      <main className="main-content">{children}</main>
      {collapsed ? (
        <button
          className="sidebar-edge-toggle sidebar-edge-toggle-open"
          onClick={() => setCollapsed(false)}
          aria-label="展开侧边栏"
          type="button"
        >
          <ChevronRight size={16} />
        </button>
      ) : null}
    </div>
  );
}
