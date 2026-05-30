// E2E 共享 API 客户端 (yamdc): 严格按生产 web/src/lib/api/core.ts 的
// envelope 协议消费 — 任何 HTTP 2xx 响应必须带 `{ code, message, data }`,
// code != 0 视为业务错; 非 2xx / 解析失败也是硬错.
//
// E2E_API_BASE_URL 默认 http://localhost:8080 (devcontainer 内后端);
// CI 上可以指到 sidecar 容器对外的 hostname.

export interface ApiEnvelope<T> {
  code: number;
  message: string;
  data: T;
}

export const E2E_API_BASE_URL =
  process.env.E2E_API_BASE_URL ?? "http://localhost:8080";

interface CallOptions {
  body?: unknown;
  headers?: Record<string, string>;
  allowBusinessError?: boolean;
}

async function call<T>(
  method: string,
  path: string,
  options: CallOptions = {},
): Promise<T> {
  const url = `${E2E_API_BASE_URL}${path}`;
  const headers: Record<string, string> = { ...options.headers };
  let bodyText: string | undefined;
  if (options.body !== undefined) {
    bodyText = JSON.stringify(options.body);
    if (!Object.keys(headers).some((k) => k.toLowerCase() === "content-type")) {
      headers["Content-Type"] = "application/json";
    }
  }
  const res = await fetch(url, { method, headers, body: bodyText });
  const rawBody = await res.text();
  if (!res.ok) {
    throw new Error(
      `E2E API ${method} ${path} failed: HTTP ${res.status} ${res.statusText}\nbody: ${rawBody}`,
    );
  }
  let parsed: ApiEnvelope<T>;
  try {
    parsed = JSON.parse(rawBody) as ApiEnvelope<T>;
  } catch (err) {
    throw new Error(
      `E2E API ${method} ${path} returned non-JSON body: ${(err as Error).message}\nbody: ${rawBody}`,
    );
  }
  if (parsed.code !== 0 && !options.allowBusinessError) {
    throw new Error(
      `E2E API ${method} ${path} returned business error: code=${parsed.code} message=${parsed.message}\nbody: ${rawBody}`,
    );
  }
  return parsed.data;
}

export async function apiGet<T>(path: string): Promise<T> {
  return call<T>("GET", path);
}

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return call<T>("POST", path, { body });
}

export async function apiPut<T>(path: string, body?: unknown): Promise<T> {
  return call<T>("PUT", path, { body });
}

export async function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  return call<T>("PATCH", path, { body });
}

export async function apiDelete(path: string): Promise<void> {
  await call<unknown>("DELETE", path);
}

export async function apiCallAllowBusinessError<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<ApiEnvelope<T>> {
  // 用来覆盖"异常路径": 后端必须 200 + envelope.code != 0 才算合规.
  const url = `${E2E_API_BASE_URL}${path}`;
  const headers: Record<string, string> = {};
  let bodyText: string | undefined;
  if (body !== undefined) {
    bodyText = JSON.stringify(body);
    headers["Content-Type"] = "application/json";
  }
  const res = await fetch(url, { method, headers, body: bodyText });
  if (!res.ok) {
    const txt = await res.text();
    throw new Error(`unexpected non-2xx for business-error path ${method} ${path}: HTTP ${res.status} body: ${txt}`);
  }
  const json = (await res.json()) as ApiEnvelope<T>;
  return json;
}
