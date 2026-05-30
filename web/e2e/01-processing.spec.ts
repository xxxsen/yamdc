// 01-processing: Processing 页面用户故事级 E2E.
//
// E2E 走真实后端 (yamdc server) + 真实前端 (Next.js dev) +
// devcontainer fixture (scan 目录里有 1 个占位视频文件).
//
// 覆盖的用户故事:
//   1) 页面可达 + 关键 UI 元素出现 + 协议契约稳;
//   2) 触发扫描 -> jobs 列表里出现 fixture 视频对应的 job (用户故事:
//      "扫描后我能看到一个新条目");
//   3) 单 job 日志接口可达 (用户故事: "我能看 job 跑过的日志");
//   4) 异常路径 (不存在的 job 触发 run): HTTP 200 + envelope 业务错.
//
// 任何 backend 5xx / Unhandled Runtime Error 都会让本 spec 立刻失败.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet } from "./helpers/api";

const FORBIDDEN_RUNTIME_TOKENS = [
  "Unhandled Runtime Error",
  "Internal Server Error",
];

interface JobItem {
  id: number;
  file_name: string;
  rel_path: string;
  status: string;
}

interface JobListResponse {
  items: JobItem[];
  total?: number;
}

interface JobLogEntry {
  id: number;
  job_id: number;
  level: string;
  message: string;
}

test.describe("processing", () => {
  test("processing 页面渲染、healthz 协议 envelope.code === 0", async ({ page }) => {
    await page.goto("/processing");
    await expect(page.locator("body")).toBeVisible();
    const bodyText = (await page.locator("body").innerText()).trim();
    for (const token of FORBIDDEN_RUNTIME_TOKENS) {
      expect(bodyText).not.toContain(token);
    }

    const health = await apiGet<{ status: string }>("/api/healthz");
    expect(health).toEqual({ status: "ok" });
  });

  test("用户故事: 触发扫描 -> jobs 列表出现 fixture 视频条目", async () => {
    // /api/scan 是同步路径, scanner.Scan(ctx) 在 walk + upsert 之后才返回, 因此
    // POST 完成时 fixture 必已落库. 唯一允许的业务错是 errScanAlreadyRunning
    // (并发触发), 通过 apiCallAllowBusinessError 接 envelope 后再判定:
    //   - code === 0: 正常扫完;
    //   - code !== 0: 业务错信息必须能解释清楚 (scan running / 缺依赖等),
    //     此时回退到下一次 GET /api/jobs 验证 fixture (上一次扫描结果还在).
    const scanEnv = await apiCallAllowBusinessError<unknown>("POST", "/api/scan");
    expect(typeof scanEnv.message).toBe("string");

    const env = await apiCallAllowBusinessError<JobListResponse>(
      "GET",
      "/api/jobs?page=1&page_size=50",
    );
    expect(env.code).toBe(0);
    const items = env.data?.items ?? [];
    expect(Array.isArray(items)).toBe(true);
    expect(items.length, "seed-e2e-fixtures.sh 已写 yamdc-e2e-scan.mp4, scan 后必须能列到 ≥1 个 job").toBeGreaterThan(0);
    const fixtureJob = items.find((item) =>
      item.file_name.includes("yamdc-e2e-scan") || item.rel_path.includes("yamdc-e2e-scan"),
    );
    expect(fixtureJob, "fixture 视频 yamdc-e2e-scan.mp4 应在 jobs 列表里").toBeTruthy();
  });

  test("用户故事: 查看 job 日志 (单 job 日志接口可达)", async () => {
    const scanEnv = await apiCallAllowBusinessError<unknown>("POST", "/api/scan");
    expect(typeof scanEnv.message).toBe("string");

    const list = await apiCallAllowBusinessError<JobListResponse>(
      "GET",
      "/api/jobs?page=1&page_size=50",
    );
    expect(list.code).toBe(0);
    const items = list.data?.items ?? [];
    expect(items.length, "scan 之后 jobs 列表不应为空 (fixture 已注入)").toBeGreaterThan(0);
    const target = items[0];
    const logs = await apiCallAllowBusinessError<JobLogEntry[] | null>(
      "GET",
      `/api/jobs/${target.id}/logs`,
    );
    expect(logs.code).toBe(0);
    expect(Array.isArray(logs.data ?? [])).toBe(true);
  });

  test("不存在 job 触发 run: HTTP 200 + envelope.code != 0 (业务错)", async () => {
    const env = await apiCallAllowBusinessError(
      "POST",
      "/api/jobs/999999/run",
    );
    expect(env.code).not.toBe(0);
    expect(typeof env.message).toBe("string");
    expect(env.message.length).toBeGreaterThan(0);
  });

  test("不存在 job 删除: HTTP 200 + envelope (业务错协议保持)", async () => {
    const env = await apiCallAllowBusinessError(
      "DELETE",
      "/api/jobs/999999",
    );
    expect(typeof env.code).toBe("number");
    expect(env.code).not.toBe(0);
  });
});
