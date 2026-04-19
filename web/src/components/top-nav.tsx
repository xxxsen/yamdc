"use client";

import { Archive, Bug, Clapperboard, ClipboardCheck, FolderKanban } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

type NavItem = {
  href: string;
  title: string;
  subtitle: string;
  icon: typeof Bug;
};

export function TopNav() {
  const pathname = usePathname();
  const items: readonly NavItem[] = [
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
  ];
  const toolItems: readonly NavItem[] = [
    {
      href: "/debug",
      title: "调试工具",
      subtitle: "测试、排查与诊断",
      icon: Bug,
    },
  ];

  const renderItems = (navItems: readonly NavItem[]) =>
    navItems.map((item) => {
      const Icon = item.icon;
      const active = isActive(pathname, item.href);
      // 用 utility 直接表达 active 态. 老 .sidebar-link-active 是 bg-accent
      // + color #fff8f4 (近白略暖); icon 容器在 active 下从 5% 黑变 16% 白;
      // subtitle 在 active 下从 muted 变 78% 白. utility 一一对应.
      const linkClass = [
        "flex items-center justify-start gap-3 px-4 py-3.5 rounded-2xl border transition-[background,border-color,transform] duration-150 ease hover:translate-x-0.5",
        active
          ? "bg-accent border-transparent text-[#fff8f4]"
          : "bg-white/55 border-transparent text-foreground",
      ].join(" ");
      const iconClass = [
        "inline-flex items-center justify-center w-[34px] h-[34px] rounded-xl",
        active ? "bg-[rgba(255,248,244,0.16)]" : "bg-[rgba(31,26,20,0.05)]",
      ].join(" ");
      const subtitleClass = [
        "text-[12px] leading-[1.45]",
        active ? "text-[rgba(255,248,244,0.78)]" : "text-muted",
      ].join(" ");
      return (
        <Link key={item.href} className={linkClass} href={item.href} title={item.title}>
          <span className={iconClass}>
            <Icon size={16} />
          </span>
          <span className="grid gap-0.5 min-w-0">
            <span className="font-semibold">{item.title}</span>
            <span className={subtitleClass}>{item.subtitle}</span>
          </span>
        </Link>
      );
    });

  return (
    <div className="flex flex-1 flex-col gap-[18px] min-h-0 basis-auto">
      <nav className="grid gap-2.5">{renderItems(items)}</nav>
      <div className="mt-auto flex flex-col gap-2.5 pt-4 border-t border-[rgba(105,79,51,0.12)]">
        <div className="text-[11px] font-bold tracking-[0.16em] uppercase text-muted">
          工具
        </div>
        <nav className="grid gap-2.5">{renderItems(toolItems)}</nav>
      </div>
    </div>
  );
}
