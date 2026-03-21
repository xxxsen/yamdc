"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function TopNav() {
  const pathname = usePathname();

  return (
    <nav style={{ display: "grid", gap: 10 }}>
      <Link
        className={`sidebar-link ${isActive(pathname, "/processing") ? "sidebar-link-active" : ""}`}
        href="/processing"
        title="文件列表"
      >
        文件列表
      </Link>
      <Link
        className={`sidebar-link ${isActive(pathname, "/review") ? "sidebar-link-active" : ""}`}
        href="/review"
        title="Review 列表"
      >
        Review列表
      </Link>
    </nav>
  );
}
