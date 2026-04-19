import { defineConfig } from "vitest/config";
import path from "node:path";

// coverage.include 刻意只枚举"已经被测试固化"的模块, 目的是让阈值守住
// 这些模块的行为不被后续重构意外改崩, 而不是给 UI 组件刷无意义的分。
// 新增被测文件时, 需要同步把它加进 include, 并视情况调 thresholds。
// 详见 td/022-frontend-optimization-roadmap.md §3.3。
const COVERED_SOURCES = [
  // api/* 是 lib/api.ts 按资源拆分后的实际实现; lib/api.ts 本身只做 re-export,
  // 不单独列 — 它的行为全部由 api/* 的测试覆盖, 收进 include 只会拉低百分比。
  "src/lib/api/**/*.ts",
  "src/lib/upload-debug.ts",
  // lib/utils.ts: 公共工具函数 (cn / formatBytes / formatUnixMillis),
  // 被全站组件广泛引用, 行为冻结在单测里防止回归。
  "src/lib/utils.ts",
  "src/components/plugin-editor/plugin-editor-utils.ts",
  "src/components/plugin-editor/plugin-editor-constants.ts",
  // plugin-editor-state-ops.ts: usePluginEditorState 内部的 16 个 StateOp 工厂.
  // 每个 op 是一次不可变状态更新, 一错就全错, 行为冻结在单测.
  "src/components/plugin-editor/plugin-editor-state-ops.ts",
  // plugin-editor/use-plugin-editor-state.ts: 编辑器 shell 的主 state 容器,
  // localStorage 调试草稿的 hydrate/save + debug 运行四条 action 分支
  // (compile / request / workflow / scrape) + 剪贴板 / YAML 导入 / 清空草稿.
  // 大头状态机, 行为冻结在单测里.
  "src/components/plugin-editor/use-plugin-editor-state.ts",
  // plugin-editor/output-panels/dom-utils.ts: XPath 计算 + DOM 文本匹配 /
  // 遍历. 算法复杂, 是 body 面板的核心逻辑, 回归风险高.
  "src/components/plugin-editor/output-panels/dom-utils.ts",
  // library-shell/move-refresh-reducer.ts: pure state machine for library
  // shell's 移动到媒体库 / 重新扫描库 生命周期. 迁移自 §3.4, 修掉了 0KB
  // fast-path 漏刷新 bug. 行为冻结在单测里.
  "src/components/library-shell/move-refresh-reducer.ts",
  // library-shell/use-library-move-refresh.ts: reducer 之上包裹 polling / flash
  // timer / auto-refresh 等副作用, 是两次 bug fix (commit 1d2f24d watermark +
  // commit 7bce4bd auto-refresh 不串扰 refresh 按钮) 的落脚点. 用
  // @testing-library/react renderHook + fake timers 把完整生命周期冻在单测里.
  "src/components/library-shell/use-library-move-refresh.ts",
  // library-shell/utils.ts: library-shell 的纯工具集 (cloneMeta / pickVariant /
  // normalizeMeta / handleMoveToMediaLibraryError / getMoveButtonLabel 等).
  // §2.2 B-2 里从 library-shell.tsx 拆出来的纯函数, 行为冻结防腐.
  "src/components/library-shell/utils.ts",
  // library-shell/use-library-asset-actions.ts: library 详情页的资产操作 hook
  // (替换封面 / 海报 / fanart, 删除 fanart, 从封面裁剪海报). 逻辑涉及
  // file input / object URL / version cache bust / override lifecycle, 是
  // 用户可见度最高的交互之一, 回归风险高, 行为冻结在单测里.
  "src/components/library-shell/use-library-asset-actions.ts",
  // media-library-shell/utils.ts: 媒体库页面派生逻辑 (getReleaseYear /
  // extractYearOptions / mergeYearOptions / toMediaLibrarySyncMessage /
  // formatSyncLogTime).
  "src/components/media-library-shell/utils.ts",
  // media-library-shell/use-media-library-sync.ts: 媒体库同步任务的 polling +
  // flash 生命周期, 与 use-library-move-refresh 同构的 "running -> completed"
  // 观测守门 (observedSyncRunningRef). 行为冻结在单测里.
  "src/components/media-library-shell/use-media-library-sync.ts",
  // media-library-shell/use-media-library-detail.ts: 详情 modal 的 fetch /
  // 关闭 / Escape 监听 / applyDetailChange 同步回 items 列表. 交互路径
  // 短但关键, 被主 shell 直接复用, 行为冻结在单测里.
  "src/components/media-library-shell/use-media-library-detail.ts",
  // handler-debug-shell/use-handler-debug-state.ts: handler-debug 页面主
  // state 容器. 覆盖默认全选 / searcher prefill 接管 / metaJSON+chainIDs
  // 持久化 / handleRun 三种校验 + 成功 + 失败路径 + add/remove/move.
  "src/components/handler-debug-shell/use-handler-debug-state.ts",
  // media-library-detail-shell/utils.ts: 媒体库详情页的纯工具集 (cloneMeta /
  // pickVariant / normalizeMeta / getVariantCoverPath). 与 library-shell/utils
  // 并行的一份, 未来可能合并; 先各自冻结.
  "src/components/media-library-detail-shell/utils.ts",
  // media-library-detail-shell/use-media-library-detail-state.ts: 媒体库详情
  // 页面主 state 容器. 覆盖 8s polling, 编辑态守门, handleSaveEdit 的
  // dirty check + 成功/失败路径, handleCancelEdit 回滚 draft, message
  // auto-clear 对 '失败/error' 关键字的短路.
  "src/components/media-library-detail-shell/use-media-library-detail-state.ts",
  // review-shell/utils.ts: review 页面 meta 解析与 payload 构造
  // (parseMeta / parseRawMeta / normalizeList / buildPayload / imageTitle).
  "src/components/review-shell/utils.ts",
  // review-shell/use-review-batch-actions.ts: review 页面的单条/批量 入库
  // + 删除 action 集散. moveRunning 短路, persistReview 失败短路,
  // 部分/全部失败的 message 汇总都冻结在单测里.
  "src/components/review-shell/use-review-batch-actions.ts",
  // review-shell/use-review-asset-actions.ts: review 页面的封面/海报/fanart
  // 上传 + crop + remove. 覆盖 file-input 触发 / cancel / 上传失败 / crop
  // 成功 & 失败 / fanart 替换同 key 等边界.
  "src/components/review-shell/use-review-asset-actions.ts",
  // handler-debug-shell/utils.ts: JSON 按行 LCS diff 算法 + debug 常量.
  // 算法边界情况多, 行为冻结.
  "src/components/handler-debug-shell/utils.ts",
  // job-table/helpers.ts: job 表格排序 / 选中判定 / 番号 meta 派生等.
  // 本来就有 24 个测试, 这次补进白名单让阈值真正守护它.
  "src/components/job-table/helpers.ts",
  // job-table/use-job-actions.ts: job 列表页的 action 集散 hook — 8s 轮询 +
  // 250ms 搜索 debounce + handleScan/handleRun/handleRerun/handleOpenLogs/
  // handleDelete/handleToggleSelectAll/handleToggleSelectJob/handleStartEditNumber
  // /handleCommitEditNumber/handleCancelEditNumber/handleRunSelectedJobs.
  // 回归风险最高的用户交互入口, 行为冻结在单测里.
  "src/components/job-table/use-job-actions.ts",
  // components/ui/*: 项目公共原子组件 (Button / Badge / Modal ...),
  // 每个组件带单测, 入白名单后阈值守护其行为不被后续重构改崩。
  "src/components/ui/**/*.tsx",
];

export default defineConfig({
  test: {
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
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
