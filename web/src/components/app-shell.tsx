"use client";

import { ChevronLeft, ChevronRight, X } from "lucide-react";
import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

import { TopNav } from "@/components/top-nav";

const MEDIA_LIBRARY_RETURN_KEY = "yamdc.media-library.return-path";

export function AppShell({
  children,
}: Readonly<{
  children: ReactNode;
}>) {
  const [collapsed, setCollapsed] = useState(false);
  const pathname = usePathname();
  const router = useRouter();
  const isMediaLibraryRoute = pathname.startsWith("/media-library");
  const currentTime = new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date());

  useEffect(() => {
    if (typeof window === "undefined" || isMediaLibraryRoute) {
      return;
    }
    const nextPath = `${window.location.pathname}${window.location.search}${window.location.hash}`;
    window.sessionStorage.setItem(MEDIA_LIBRARY_RETURN_KEY, nextPath);
  }, [isMediaLibraryRoute, pathname]);

  const handleExitMediaLibrary = () => {
    if (typeof window === "undefined") {
      router.push("/review");
      return;
    }
    const target = window.sessionStorage.getItem(MEDIA_LIBRARY_RETURN_KEY);
    if (target && !target.startsWith("/media-library")) {
      router.push(target);
      return;
    }
    router.push("/review");
  };

  return (
    <div className={`app-shell ${collapsed ? "app-shell-collapsed" : ""} ${isMediaLibraryRoute ? "app-shell-wide" : ""}`}>
      {!collapsed && !isMediaLibraryRoute ? (
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
      <main className={`main-content ${isMediaLibraryRoute ? "main-content-wide" : ""}`}>{children}</main>
      {isMediaLibraryRoute ? (
        <button className="workspace-close-btn" type="button" aria-label="退出媒体库工作区" onClick={handleExitMediaLibrary}>
          <X size={18} />
        </button>
      ) : null}
      {collapsed && !isMediaLibraryRoute ? (
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
