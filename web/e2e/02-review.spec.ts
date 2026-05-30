// 02-review: Review 页面用户故事级 E2E.
//
// 真实 backend 启动 + sqlite 迁移完成后, scripts/devcontainer/seed-e2e-db.sh
// 会直接往 yamdc_job_tab + yamdc_scrape_data_tab 插一条 status=reviewing 的
// fixture (不走外网 scrape), rel_path 固定为 e2e-review-fixture/E2E-REVIEW-001.mp4,
// 让本 spec 能在不依赖网络的前提下覆盖完整的 review 用户故事:
//   1) 页面可达 + 列表渲染 (fixture 在则至少 1 条);
//   2) 点击 fixture job → 详情面板出现并显示 number/title 字段;
//   3) 在标题输入框改名 → blur 触发 PUT /api/review/jobs/:id → 刷新页面后值仍存在;
//   4) 协议契约: save / import / reject 不存在的 job 必须 HTTP 200 +
//      envelope.code != 0, 是协议契约的关键回归点;
//   5) 异常恢复: 用户重复触发 save 不存在 job, 多次返回稳定的 envelope,
//      不会在第二次产生 5xx (协议层稳定性).
//
// 注意: fixture 的 rel_path / number 是常量, spec 间共用. seed-e2e-db.sh
// 在 run-e2e-test.sh 的每次启动都会重置 fixture 状态回 reviewing, 所以
// reject / import 路径不要假设 fixture 一定还存在 — 用 listing API 查一次再走.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet } from "./helpers/api";

interface JobItem {
  id: number;
  rel_path: string;
  number: string;
  status: string;
  updated_at: number;
}

interface JobListResponse {
  items: JobItem[];
  total: number;
}

interface ScrapeDataItem {
  id: number;
  job_id: number;
  raw_data: string;
  review_data: string;
  final_data: string;
  status: string;
}

const FIXTURE_NUMBER = "E2E-REVIEW-001";
const FIXTURE_REL_PATH = `e2e-review-fixture/${FIXTURE_NUMBER}.mp4`;

async function fetchFixtureJob(): Promise<JobItem | null> {
  const list = await apiGet<JobListResponse>("/api/jobs?status=reviewing&page=1&page_size=200");
  return list.items.find((item) => item.rel_path === FIXTURE_REL_PATH) ?? null;
}

test.describe("review 页面 — 协议契约", () => {
  test("review 页面可达 + 渲染 (无 reviewing job 时也不崩)", async ({ page }) => {
    await page.goto("/review");
    await expect(page.locator("body")).toBeVisible();
    const text = (await page.locator("body").innerText()).trim();
    expect(text.length).toBeGreaterThan(0);
    expect(text).not.toContain("Unhandled Runtime Error");
  });

  test("review save 不存在的 job: 必须 HTTP 200 + envelope.code != 0", async () => {
    const env = await apiCallAllowBusinessError("PUT", "/api/review/jobs/999999", {
      review_data: "{}",
    });
    expect(env.code).not.toBe(0);
    expect(env.message.length).toBeGreaterThan(0);
  });

  test("review import 不存在的 job: 必须 HTTP 200 + envelope.code != 0", async () => {
    const env = await apiCallAllowBusinessError(
      "POST",
      "/api/review/jobs/999999/import",
    );
    expect(env.code).not.toBe(0);
  });

  test("review reject 不存在的 job: 协议契约 HTTP 200 + envelope.code != 0", async () => {
    const env = await apiCallAllowBusinessError(
      "POST",
      "/api/review/jobs/999999/reject",
      { reason: "e2e reject path" },
    );
    expect(env.code).not.toBe(0);
    expect(typeof env.message).toBe("string");
  });

  test("用户故事: 重复 save 不存在的 job 不会引发 5xx (协议稳定性)", async () => {
    for (let i = 0; i < 3; i++) {
      const env = await apiCallAllowBusinessError("PUT", "/api/review/jobs/999999", {
        review_data: "{}",
      });
      expect(env.code).not.toBe(0);
      expect(typeof env.message).toBe("string");
    }
  });
});

test.describe("review 页面 — fixture 真实用户故事", () => {
  test("fixture 出现在 GET /api/jobs?status=reviewing 列表里", async () => {
    const job = await fetchFixtureJob();
    expect(job, `seed-e2e-db.sh 没有把 ${FIXTURE_REL_PATH} 写进 reviewing 列表`).not.toBeNull();
    expect(job!.status).toBe("reviewing");
    expect(job!.number).toBe(FIXTURE_NUMBER);
  });

  test("打开 review 页 → fixture job 卡片可见 → 点击后详情面板渲染 number/title", async ({ page }) => {
    const fixture = await fetchFixtureJob();
    expect(fixture).not.toBeNull();
    const job = fixture!;
    await page.goto("/review");

    // 列表里至少能看到 fixture 的 rel_path; 用 :has-text 防止多个 reviewing
    // job 时误中其它行 (rel_path 完全等值匹配).
    const card = page.locator(".review-job-card", {
      has: page.locator(`.review-job-card-path:has-text("${job.rel_path}")`),
    });
    await expect(card).toBeVisible();

    await card.locator(".review-job-card-main").click();
    // ReviewFormFields 渲染了一个标题输入框, 等它出现说明详情已加载.
    const titleInput = page.locator(".review-top-fields .review-input-strong").first();
    await expect(titleInput).toBeVisible();
    await expect(titleInput).toHaveValue(/.+/);
  });

  test("auto-save: blur 标题输入框触发 PUT /api/review/jobs/:id 并落库", async ({ page }) => {
    const fixture = await fetchFixtureJob();
    expect(fixture).not.toBeNull();
    const job = fixture!;
    await page.goto("/review");

    const card = page.locator(".review-job-card", {
      has: page.locator(`.review-job-card-path:has-text("${job.rel_path}")`),
    });
    await card.locator(".review-job-card-main").click();

    const titleInput = page.locator(".review-top-fields .review-input-strong").first();
    await expect(titleInput).toBeVisible();
    const newTitle = `E2E Edited Title ${Date.now()}`;

    // 等待 PUT 触发 + 服务端落库. 注意 onBlur 才发请求, 因此先 fill 再
    // 主动 blur (用 keyboard Tab 触发).
    await titleInput.fill(newTitle);
    const savePromise = page.waitForResponse((res) =>
      res.url().includes(`/api/review/jobs/${job.id}`)
      && res.request().method() === "PUT"
      && res.status() === 200,
    );
    await titleInput.blur();
    const saveResp = await savePromise;
    const saveBody = await saveResp.json();
    expect(saveBody.code).toBe(0);

    // 持久化校验: 重新请求 GET /api/review/jobs/:id 看 review_data 里
    // 的 title 是否已落到 DB.
    const fresh = await apiGet<ScrapeDataItem | null>(`/api/review/jobs/${job.id}`);
    expect(fresh).not.toBeNull();
    const reviewMeta = JSON.parse(fresh!.review_data) as { title?: string };
    expect(reviewMeta.title).toBe(newTitle);
  });
});
