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
  const currentTime = new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date());

  return (
    <div className={`app-shell ${collapsed ? "app-shell-collapsed" : ""}`}>
      {!collapsed ? (
        <aside className="panel sidebar">
          <div className="sidebar-brand">
            <div className="sidebar-brand-top">
              <div className="sidebar-brand-mark">
                <span>Y</span>
              </div>
              <div className="sidebar-brand-eyebrow">YAMDC</div>
            </div>
            <div className="sidebar-brand-copy">
              <h1 className="sidebar-brand-title">Media Capture</h1>
              <p className="sidebar-brand-subtitle">把扫描、刮削、复核和入库收在一个工作台里。</p>
            </div>
          </div>
          <div className="sidebar-status-card">
            <div className="sidebar-status-row">
              <span className="sidebar-status-label">Workspace</span>
              <span className="sidebar-status-value">Ready</span>
            </div>
            <div className="sidebar-status-row">
              <span className="sidebar-status-label">Local Time</span>
              <span className="sidebar-status-value">{currentTime}</span>
            </div>
          </div>
          <TopNav />
          <div className="sidebar-footnote">Queue-first workflow for processing and review.</div>
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
