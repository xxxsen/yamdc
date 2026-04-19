// index.ts: 对外统一入口, 让所有 `import { foo } from "@/lib/api"` 保持工作。
// 这里刻意只做 re-export, 不写业务: 新增 API 一律进对应资源模块, 然后在此追加一行。
// normalizeLibrary* 是跨模块复用的内部 helper, 暂不对外暴露 (外部使用方通过
// 各 list/get 函数拿到的已经是归一化后的值), 需要时再单独 export。

export {
  apiRequest,
  buildPath,
  getAPIBaseURL,
  type ApiRequestInit,
  type APIResponse,
  type MediaFileRef,
  type PluginEditorEnvelope,
} from "./core";

export {
  type JobStatus,
  type JobItem,
  type JobLogItem,
  type JobListResponse,
  listJobs,
  runJob,
  rerunJob,
  deleteJob,
  updateJobNumber,
  listJobLogs,
} from "./jobs";

export {
  type LibraryListItem,
  type LibraryMeta,
  type LibraryFileItem,
  type LibraryVariant,
  type LibraryDetail,
  listLibraryItems,
  getLibraryItem,
  updateLibraryItem,
  deleteLibraryItem,
  replaceLibraryAsset,
  cropLibraryPosterFromCover,
  deleteLibraryFile,
  getLibraryFileURL,
} from "./library";

export {
  type MediaLibraryItem,
  type MediaLibraryDetail,
  type TaskState,
  type MediaLibraryStatus,
  type MediaLibrarySyncLogEntry,
  listMediaLibraryItems,
  getMediaLibraryItem,
  updateMediaLibraryItem,
  replaceMediaLibraryAsset,
  deleteMediaLibraryFile,
  getMediaLibraryStatus,
  triggerMediaLibrarySync,
  listMediaLibrarySyncLogs,
  triggerMoveToMediaLibrary,
  getMediaLibraryFileURL,
} from "./media-library";

export {
  type ScrapeDataItem,
  type ReviewMeta,
  getReviewJob,
  saveReviewJob,
  importReviewJob,
  cropPosterFromCover,
  uploadReviewAsset,
} from "./review";

export { uploadAsset, getAssetURL } from "./assets";

export { triggerScan } from "./scan";

export {
  type MovieIDCleanerCandidate,
  type MovieIDCleanerExplainStep,
  type MovieIDCleanerResult,
  type MovieIDCleanerExplainResult,
  type SearcherDebugPluginCollection,
  type SearcherDebugStep,
  type SearcherDebugMovieMeta,
  type SearcherDebugPluginResult,
  type SearcherDebugResult,
  type HandlerDebugRequest,
  type HandlerDebugInstance,
  type HandlerDebugResult,
  type PluginEditorParser,
  type PluginEditorBrowserSpec,
  type PluginEditorSelector,
  type PluginEditorTransform,
  type PluginEditorField,
  type PluginEditorDraft,
  type PluginEditorCompileSummary,
  type PluginEditorCompileResult,
  type PluginEditorHTTPRequestDebug,
  type PluginEditorHTTPResponseDebug,
  type PluginEditorRequestDebugAttempt,
  type PluginEditorRequestDebugResult,
  type PluginEditorTransformStep,
  type PluginEditorFieldDebugResult,
  type PluginEditorScrapeDebugResult,
  type PluginEditorWorkflowMatchDetail,
  type PluginEditorWorkflowSelectorItem,
  type PluginEditorWorkflowStep,
  type PluginEditorWorkflowDebugResult,
  type PluginEditorCaseSpec,
  type PluginEditorCaseDebugResult,
  type PluginEditorImportResult,
  explainMovieIDCleaner,
  getSearcherDebugPlugins,
  debugSearcher,
  getHandlerDebugHandlers,
  debugHandler,
  compilePluginDraft,
  importPluginDraftYAML,
  debugPluginDraftRequest,
  debugPluginDraftWorkflow,
  debugPluginDraftScrape,
  debugPluginDraftCase,
} from "./debug";
