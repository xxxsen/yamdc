// Server-side data loaders for the Next.js Server Components on
// /processing, /review, /library, /media-library.
//
// 目的: 把 "Server Component 调一次后端拿初始数据 + 失败兜底" 集中到一个
// 文件里, 让 page 只负责把结果交给 shell, 错误统一走项目内 ErrorState.
// 任何 loader 失败都 **不向上 throw**, 而是返回 { data: <fallback>,
// errorMessage: <用户文案> }, page 再把 errorMessage 透到 shell, shell
// 渲染 ErrorState + "重试" 按钮 (重试走客户端 API).
//
// 这样:
//  1. 后端短暂不可达不会让用户看到 Next 默认错误页, 仍能看到完整的
//     warm serif 视觉壳;
//  2. 任何 page 都能保持一致的 fallback 形状, 避免 /media-library
//     之前局部 catch、其它 page 直接 500 的不一致;
//  3. error.tsx 只是最后防线 (loader 内的 try/catch 应该能挡掉绝大
//     多数业务错误, 真正的 panic / 序列化错误才会落到 error.tsx).

import {
  getLibraryItem,
  getMediaLibraryStatus,
  getReviewJob,
  listJobs,
  listLibraryItems,
  listMediaLibraryItems,
  type JobListResponse,
  type LibraryDetail,
  type LibraryListItem,
  type MediaLibraryItem,
  type MediaLibraryStatus,
  type ScrapeDataItem,
} from "@/lib/api";

export interface ProcessingInitialData {
  jobs: JobListResponse;
}

export interface ReviewInitialData {
  jobs: JobListResponse["items"];
  initialScrapeData: ScrapeDataItem | null;
  initialMediaStatus: MediaLibraryStatus | null;
}

export interface LibraryInitialData {
  items: LibraryListItem[];
  initialDetail: LibraryDetail | null;
  initialMediaStatus: MediaLibraryStatus | null;
}

export interface MediaLibraryInitialData {
  items: MediaLibraryItem[];
  initialStatus: MediaLibraryStatus | null;
}

export interface InitialLoaderResult<T> {
  data: T;
  errorMessage: string | null;
}

// toLoaderMessage: 把任意异常翻译成用户能读的中文提示, 同时保留
// 后端给出的具体 message (如果有). 不暴露 stack, 不带技术名词.
export function toLoaderMessage(err: unknown, fallback: string): string {
  if (err instanceof Error && err.message) {
    return err.message;
  }
  return fallback;
}

// loadProcessingInitialData: /processing 首屏数据 (待处理 job 列表).
// 失败时返回空列表 + errorMessage, 不向上 throw.
export async function loadProcessingInitialData(): Promise<InitialLoaderResult<ProcessingInitialData>> {
  try {
    const jobs = await listJobs({ status: "init,processing,failed,reviewing", all: true });
    return { data: { jobs }, errorMessage: null };
  } catch (err) {
    return {
      data: {
        jobs: { items: [], total: 0, page: 1, page_size: 0 } as unknown as JobListResponse,
      },
      errorMessage: toLoaderMessage(err, "加载处理队列失败"),
    };
  }
}

// loadReviewInitialData: /review 首屏数据 (待 review 列表 + 第一个 job
// 的 scrape data + 媒体库状态). 列表/详情失败都视为整体失败; 媒体库状
// 态失败则单独降级 (空 / null), 与原页面行为保持兼容.
export async function loadReviewInitialData(): Promise<InitialLoaderResult<ReviewInitialData>> {
  let listError: string | null = null;
  let jobs: JobListResponse["items"] = [];
  try {
    const result = await listJobs({ status: "reviewing", page: 1, pageSize: 200 });
    jobs = result.items;
  } catch (err) {
    listError = toLoaderMessage(err, "加载待 review 列表失败");
  }
  let initialScrapeData: ScrapeDataItem | null = null;
  if (!listError && jobs.length > 0) {
    try {
      initialScrapeData = await getReviewJob(jobs[0].id);
    } catch (err) {
      listError = toLoaderMessage(err, "加载首条 scrape 数据失败");
    }
  }
  let initialMediaStatus: MediaLibraryStatus | null = null;
  try {
    initialMediaStatus = await getMediaLibraryStatus();
  } catch {
    initialMediaStatus = null;
  }
  return {
    data: { jobs, initialScrapeData, initialMediaStatus },
    errorMessage: listError,
  };
}

// loadLibraryInitialData: /library 首屏数据 (列表 + 首条详情 + 媒体库
// 状态). 列表失败 → 整体失败; 首条详情失败 → 详情 null + 同一 errorMessage
// 让 shell 提示用户; 媒体库状态失败 → null, 不影响主体.
export async function loadLibraryInitialData(): Promise<InitialLoaderResult<LibraryInitialData>> {
  let items: LibraryListItem[] = [];
  let listError: string | null = null;
  try {
    items = await listLibraryItems();
  } catch (err) {
    listError = toLoaderMessage(err, "加载已入库列表失败");
  }
  let initialDetail: LibraryDetail | null = null;
  if (!listError && items.length > 0) {
    try {
      initialDetail = await getLibraryItem(items[0].rel_path);
    } catch {
      initialDetail = null;
    }
  }
  let initialMediaStatus: MediaLibraryStatus | null = null;
  try {
    initialMediaStatus = await getMediaLibraryStatus();
  } catch {
    initialMediaStatus = null;
  }
  return {
    data: { items, initialDetail, initialMediaStatus },
    errorMessage: listError,
  };
}

// loadMediaLibraryInitialData: /media-library 首屏数据 (媒体库列表 +
// 状态). 任意一项失败都 fallback 到空 / null, 同时把第一个错误抛给 shell.
export async function loadMediaLibraryInitialData(): Promise<InitialLoaderResult<MediaLibraryInitialData>> {
  let items: MediaLibraryItem[] = [];
  let initialError: string | null = null;
  try {
    items = await listMediaLibraryItems({ sort: "ingested", order: "desc" });
  } catch (err) {
    initialError = toLoaderMessage(err, "加载媒体库列表失败");
  }
  let initialStatus: MediaLibraryStatus | null = null;
  try {
    initialStatus = await getMediaLibraryStatus();
  } catch (err) {
    if (!initialError) {
      initialError = toLoaderMessage(err, "加载媒体库状态失败");
    }
  }
  return {
    data: { items, initialStatus },
    errorMessage: initialError,
  };
}
