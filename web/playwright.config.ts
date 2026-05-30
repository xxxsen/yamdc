// Playwright 配置 (yamdc): Desktop Chrome only — 项目桌面优先, 不投入
// mobile / firefox / webkit 多路矩阵, 避免重复成本.
//
// fullyParallel=false 是为了让 backend 状态在 spec 之间可控:
//   yamdc 的 scan / save / library 共享同一份本地目录, 任何并行 spec
//   都可能读到中间状态, 让稳定性变成噪音. 串行跑慢一点, 调试更省.
//
// E2E_BASE_URL 让 CI 把 baseURL 转到外部 host (比如 sidecar container);
// 默认指向 devcontainer 里的 localhost:3000.

import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 180_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
