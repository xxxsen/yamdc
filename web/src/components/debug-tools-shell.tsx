"use client";

import { Bug } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

const TOOL_ITEMS = [
  {
    href: "/debug/ruleset",
    title: "规则集测试",
    description: "查看番号清洗规则的逐步执行链路。",
  },
  {
    href: "/debug/searcher",
    title: "插件检索测试",
    description: "输入番号后，直接验证 searcher 插件链的检索结果。",
  },
  {
    href: "/debug/handler",
    title: "Handler 测试",
    description: "编辑当前 handler 链和 Meta JSON，直接观察处理前后的差异。",
  },
] as const;

export function DebugToolsShell({ children }: Readonly<{ children: ReactNode }>) {
  const pathname = usePathname();

  return (
    <div className="debug-tools-page">
      <aside className="panel debug-tools-nav-panel">
        <div className="debug-tools-nav-head">
          <span className="debug-tools-nav-eyebrow">
            <Bug size={14} />
            调试工具
          </span>
          <h2>工具列表</h2>
          <p>这里放调试、诊断和辅助测试工具。左侧选择工具，右侧查看对应输出界面。</p>
        </div>

        <div className="debug-tools-list">
          {TOOL_ITEMS.map((item) => {
            const active = pathname === item.href || pathname.startsWith(`${item.href}/`);
            return (
              <Link key={item.href} href={item.href} className={`debug-tools-link ${active ? "debug-tools-link-active" : ""}`}>
                <span className="debug-tools-link-title">{item.title}</span>
                <span className="debug-tools-link-copy">{item.description}</span>
              </Link>
            );
          })}
        </div>
      </aside>

      <section className="debug-tools-content">{children}</section>
    </div>
  );
}
