"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function TopNav() {
  const pathname = usePathname();

  return (
    <nav style={{ display: "flex", gap: 10 }}>
      <Link
        className={`btn ${isActive(pathname, "/processing") ? "btn-primary" : ""}`}
        href="/processing"
      >
        待处理
      </Link>
      <Link
        className={`btn ${isActive(pathname, "/review") ? "btn-primary" : ""}`}
        href="/review"
      >
        待 Review
      </Link>
    </nav>
  );
}

