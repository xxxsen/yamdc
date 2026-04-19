// api.ts: 历史单文件入口, 现为 api/ 模块的 re-export 外壳。
// 外部代码 `import ... from "@/lib/api"` 继续可用, 不感知底下按资源拆分。
// 新增 API 请加到 api/<resource>.ts, 再在 api/index.ts 里 re-export, 不要回流到本文件。
// 详见 td/022-frontend-optimization-roadmap.md §3.2。

export * from "./api/index";
