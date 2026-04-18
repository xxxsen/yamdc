import { defineConfig } from "vitest/config";
import path from "node:path";

// coverage.include 刻意只枚举"已经被测试固化"的模块, 目的是让阈值守住
// 这些模块的行为不被后续重构意外改崩, 而不是给 UI 组件刷无意义的分。
// 新增被测文件时, 需要同步把它加进 include, 并视情况调 thresholds。
// 详见 td/022-frontend-optimization-roadmap.md §3.3。
const COVERED_SOURCES = [
  "src/lib/api.ts",
  "src/lib/upload-debug.ts",
  "src/components/plugin-editor/plugin-editor-utils.ts",
  "src/components/plugin-editor/plugin-editor-constants.ts",
];

export default defineConfig({
  test: {
    include: ["src/**/*.test.ts"],
    coverage: {
      provider: "v8",
      include: COVERED_SOURCES,
      thresholds: {
        // branches 93 是 plugin-editor-utils.ts 当前实测值的地板, 抬到 95
        // 会误伤; 后续补齐分支测试后再收紧。其它三项保持 95% 行业常规线。
        statements: 95,
        branches: 93,
        functions: 95,
        lines: 95,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
});
