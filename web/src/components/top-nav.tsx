"use client";

import { ClipboardCheck, FolderKanban } from "lucide-react";
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
  ] as const;

  return (
    <nav className="sidebar-nav">
      {items.map((item) => {
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
      })}
    </nav>
  );
}
