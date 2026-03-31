"use client";

import { Archive, Bug, Clapperboard, ClipboardCheck, FolderKanban } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function TopNav() {
  const pathname = usePathname();
  const items = [
    {
      href: "/processing",
      title: "文件列表",
      subtitle: "扫描、筛选与提交",
      icon: FolderKanban,
    },
    {
      href: "/review",
      title: "Review 列表",
      subtitle: "复核刮削结果并入库",
      icon: ClipboardCheck,
    },
    {
      href: "/library",
      title: "已入库",
      subtitle: "查看与修改内容",
      icon: Archive,
    },
    {
      href: "/media-library",
      title: "媒体库",
      subtitle: "数据归档与维护",
      icon: Clapperboard,
    },
  ] as const;
  const toolItems = [
    {
      href: "/debug",
      title: "调试工具",
      subtitle: "测试、排查与诊断",
      icon: Bug,
    },
  ] as const;

  const renderItems = (navItems: readonly { href: string; title: string; subtitle: string; icon: typeof Bug }[]) =>
    navItems.map((item) => {
      const Icon = item.icon;
      return (
        <Link
          key={item.href}
          className={`sidebar-link ${isActive(pathname, item.href) ? "sidebar-link-active" : ""}`}
          href={item.href}
          title={item.title}
        >
          <span className="sidebar-link-icon">
            <Icon size={16} />
          </span>
          <span className="sidebar-link-copy">
            <span className="sidebar-link-title">{item.title}</span>
            <span className="sidebar-link-subtitle">{item.subtitle}</span>
          </span>
        </Link>
      );
    });

  return (
    <div className="sidebar-nav-stack">
      <nav className="sidebar-nav">{renderItems(items)}</nav>
      <div className="sidebar-nav-secondary">
        <div className="sidebar-nav-section-title">工具</div>
        <nav className="sidebar-nav">{renderItems(toolItems)}</nav>
      </div>
    </div>
  );
}
