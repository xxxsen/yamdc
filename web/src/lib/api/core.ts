// core.ts: HTTP 通信原语与跨资源共享的最小类型。
// 其它 api/*.ts 模块都只从这里拿 apiRequest / buildPath, 不自己直接 fetch,
// 保证请求行为 (error 解析、code!=0 抛错、URL 拼接) 唯一且可替换。

export interface MediaFileRef {
  name: string;
  key: string;
}

export interface APIResponse<T> {
  code: number;
  message: string;
  data: T;
}

// PluginEditorEnvelope 是 debug/plugin-editor/* 接口返回的 data 外壳,
// 放在 core 里是因为 apiRequest 的泛型经常直接写 APIResponse<Envelope<X>>,
// 让 debug.ts 和 core.ts 共享同一份定义避免重复声明。
export interface PluginEditorEnvelope<T> {
  ok: boolean;
  warnings: string[];
  data: T;
}

function getBaseURL(): string {
  if (typeof window !== "undefined") {
    return "";
  }
  return process.env.YAMDC_API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";
}

export function getAPIBaseURL(): string {
  return getBaseURL();
}

export interface ApiRequestInit {
  method?: string;
  body?: unknown;
  formData?: FormData;
  cache?: RequestCache;
  signal?: AbortSignal;
}

export function buildPath(base: string, query?: URLSearchParams): string {
  const qs = query?.toString();
  return qs ? `${base}?${qs}` : base;
}

async function readAPIResponse<T>(resp: Response, fallbackMessage: string): Promise<APIResponse<T>> {
  if (!resp.ok) {
    let serverMessage = "";
    try {
      const body = (await resp.json()) as { message?: string };
      serverMessage = body.message ?? "";
    } catch {
      // non-JSON error response (e.g. HTML 502 page)
    }
    throw new Error(serverMessage || `${fallbackMessage} (HTTP ${resp.status})`);
  }
  const data = (await resp.json()) as APIResponse<T>;
  if (data.code !== 0) {
    throw new Error(data.message || fallbackMessage);
  }
  return data;
}

export async function apiRequest<T>(path: string, init?: ApiRequestInit): Promise<APIResponse<T>> {
  const { method, body, formData, cache, signal } = init ?? {};
  const fetchInit: RequestInit = {};
  if (method) fetchInit.method = method;
  if (cache) fetchInit.cache = cache;
  if (signal) fetchInit.signal = signal;
  if (formData) {
    fetchInit.body = formData;
  } else if (body !== undefined) {
    fetchInit.headers = { "Content-Type": "application/json" };
    fetchInit.body = JSON.stringify(body);
  }
  const resp = await fetch(`${getBaseURL()}${path}`, fetchInit);
  return readAPIResponse<T>(resp, `request ${path} failed`);
}
