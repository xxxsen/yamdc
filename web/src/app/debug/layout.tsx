import type { ReactNode } from "react";

import { DebugToolsShell } from "@/components/debug-tools-shell";

export default function DebugLayout({
  children,
}: Readonly<{
  children: ReactNode;
}>) {
  return <DebugToolsShell>{children}</DebugToolsShell>;
}
