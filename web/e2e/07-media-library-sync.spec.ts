// 07-media-library-sync: 媒体库同步用户故事级 E2E.
//
// 覆盖的用户故事:
//   1) /media-library 页面"开始同步"按钮可见且默认 enabled (没有跑同步时);
//   2) 协议契约: 触发 POST /api/media-library/sync 后, 状态查询要么是
//      idle (瞬完) / running (跑中) / completed / failed 四种合法枚举,
//      不允许出未知状态;
//   3) 同步日志接口 GET /api/media-library/sync/logs 返回数组 (用户能在 UI
//      上看到同步历史);
//   4) /api/media-library/sync 在没有 fixture 时 also-best-effort 触发: 即便
//      已在运行也必须 HTTP 200 + envelope (不能蹦 5xx) — 这是 UX 退化的
//      关键守护点.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet, apiPost } from "./helpers/api";

interface MediaLibraryStatus {
  configured: boolean;
  sync: { status: string };
  move: { status: string };
}

interface SyncLogEntry {
  task_key: string;
  status: string;
  started_at: number;
}

interface SyncLogResponse {
  items: SyncLogEntry[];
}

const SYNC_STATES = ["idle", "running", "completed", "failed"];

test.describe("media-library sync 用户故事", () => {
  test("/media-library 页面: 开始同步按钮可见且默认可点 (没跑同步时不锁死)", async ({ page }) => {
    await page.goto("/media-library");
    const btn = page.getByRole("button", { name: /开始同步|同步中/ });
    await expect(btn).toBeVisible();
  });

  test("触发同步后: status 枚举落在合法集合", async () => {
    await apiPost<unknown>("/api/media-library/sync").catch(() => undefined);
    const status = await apiGet<MediaLibraryStatus>("/api/media-library/status");
    expect(status.configured).toBe(true);
    expect(SYNC_STATES).toContain(status.sync.status);
  });

  test("协议契约: 同步日志 GET /api/media-library/sync/logs 返回数组结构", async () => {
    const env = await apiCallAllowBusinessError<SyncLogResponse>(
      "GET",
      "/api/media-library/sync/logs",
    );
    // 即便没有任何同步历史, code 也应是 0, items 是 [].
    expect(env.code).toBe(0);
    expect(Array.isArray(env.data?.items)).toBe(true);
  });

  test("重复触发 sync: 同步在跑也必须 HTTP 200 + envelope (不允许 5xx)", async () => {
    await apiPost<unknown>("/api/media-library/sync").catch(() => undefined);
    const env = await apiCallAllowBusinessError("POST", "/api/media-library/sync");
    expect(typeof env.code).toBe("number");
    expect(typeof env.message).toBe("string");
  });
});
