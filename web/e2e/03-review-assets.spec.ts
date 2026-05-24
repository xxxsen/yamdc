// 03-review-assets: review 资产 (cover/poster/fanart) 上传 / 超限拦截 /
// 海报裁剪 用户故事级 E2E.
//
// 协议契约部分 (HTTP 200 + envelope.code != 0 / 413 超限) 与 fixture 用户
// 故事部分共用同一份测试代码:
//   1) 协议契约: review asset 上传到不存在 job 必须 HTTP 200 + envelope.code != 0;
//   2) 协议契约: 上传 > 32 MiB 必须 HTTP 413 (协议外的安全保护层);
//   3) 用户故事: fixture reviewing job 详情页, 触发 cover/poster/fanart
//      上传 (经由 file chooser → /api/assets/upload → PUT /api/review/jobs/:id),
//      预览图更新.
//   4) 用户故事: 上传 > 32 MiB 必须被前端 validateUploadSize 拦截 — 不应
//      触发 /api/assets/upload 网络请求, 同时显示 "图片不能超过 32 MiB" 提示;
//   5) 用户故事: 海报裁剪 (从封面截取海报) 通过 cropper modal → 截取按钮
//      → POST /api/review/jobs/:id/poster-crop → 预览图更新.
//
// fixture 来自 scripts/devcontainer/seed-e2e-db.sh 写入的 reviewing job
// (rel_path=e2e-review-fixture/E2E-REVIEW-001.mp4).

import { expect, test, type Page } from "@playwright/test";

import { apiGet, E2E_API_BASE_URL } from "./helpers/api";

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

const FIXTURE_REL_PATH = "e2e-review-fixture/E2E-REVIEW-001.mp4";

// 32x32 透明 PNG (288 字节), 用于上传成功路径的 fixture.
// 通过 base64 解码生成. 文件足够小 (<<32 MiB) 且 http.DetectContentType
// 能稳定识别为 image/png.
// 32x32 grayscale PNG (8-bit, color type 0), 75 字节. 由 zlib 压缩 1024
// 个零像素行 (每行 1 字节 filter + 32 字节像素) 生成, IHDR/IDAT/IEND CRC
// 全部合法. 此前用的版本 IDAT 解压后字节数与 IHDR 不一致, 后端
// image.Decode 直接 "png: invalid format: too much pixel data" 抛错,
// poster-crop 因此永远拿不到合法 cover, 导致 03-review-assets 裁剪
// 用例必 fail. 替换为这个最小合法 PNG 后 image/png decoder 能正常解.
const TINY_PNG_BASE64 =
  "iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAAAAABWESUoAAAAEklEQVR42mNg" +
  "GAWjYBSMAuwAAAQgAAF8we+4AAAAAElFTkSuQmCC";

async function fetchFixtureJob(): Promise<JobItem | null> {
  const list = await apiGet<JobListResponse>("/api/jobs?status=reviewing&page=1&page_size=200");
  return list.items.find((item) => item.rel_path === FIXTURE_REL_PATH) ?? null;
}

// openFixtureReviewDetail 打开 /review 并选中 fixture job, 使详情面板渲染.
// 失败则用 expect 抛错让 spec 真实 fail (不静吞).
async function openFixtureReviewDetail(page: Page, job: JobItem): Promise<void> {
  await page.goto("/review");
  const card = page.locator(".review-job-card", {
    has: page.locator(`.review-job-card-path:has-text("${job.rel_path}")`),
  });
  await expect(card).toBeVisible();
  await card.locator(".review-job-card-main").click();
  // ReviewFormFields 渲染了一个标题输入框, 等它出现说明详情已加载.
  await expect(page.locator(".review-top-fields .review-input-strong").first()).toBeVisible();
}

test.describe("review assets — 协议契约", () => {
  test("review asset 不存在的 job: 协议保持 HTTP 200 + envelope.code != 0", async () => {
    const tinyPng = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
    const form = new FormData();
    form.set("file", new Blob([tinyPng], { type: "image/png" }), "tiny.png");

    const res = await fetch(
      `${E2E_API_BASE_URL}/api/review/jobs/999999/asset?target=cover`,
      { method: "POST", body: form },
    );
    expect(res.status).toBe(200);
    const env = (await res.json()) as { code: number; message: string };
    expect(env.code).not.toBe(0);
  });

  test("上传超过 32 MiB: 必须 HTTP 413 + envelope (协议外安全保护层)", async () => {
    const oversize = new Uint8Array(33 * 1024 * 1024);
    const form = new FormData();
    form.set("file", new Blob([oversize], { type: "image/png" }), "big.png");

    const res = await fetch(
      `${E2E_API_BASE_URL}/api/review/jobs/999999/asset?target=cover`,
      { method: "POST", body: form },
    );
    // 413 是 P1 修复明确的协议外安全保护层 (jobs_routes.go writeUploadTooLarge).
    expect(res.status).toBe(413);
    const env = (await res.json()) as { code: number; message: string };
    expect(env.code).not.toBe(0);
    expect(env.message).toContain("32");
  });
});

test.describe("review assets — fixture 真实用户故事", () => {
  test("上传 cover: 触发 file chooser → /api/assets/upload → PUT review job", async ({ page }) => {
    const fixture = await fetchFixtureJob();
    expect(fixture).not.toBeNull();
    const job = fixture!;
    await openFixtureReviewDetail(page, job);

    const tinyPngBuffer = Buffer.from(TINY_PNG_BASE64, "base64");
    const uploadCall = page.waitForResponse((res) =>
      res.url().endsWith("/api/assets/upload") && res.request().method() === "POST",
    );
    const reviewSave = page.waitForResponse((res) =>
      res.url().includes(`/api/review/jobs/${job.id}`) && res.request().method() === "PUT",
    );
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "上传封面" }).click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name: "e2e-cover.png",
      mimeType: "image/png",
      buffer: tinyPngBuffer,
    });
    const uploadResp = await uploadCall;
    const uploadBody = await uploadResp.json();
    expect(uploadBody.code).toBe(0);
    expect(typeof uploadBody.data?.key).toBe("string");
    const saveResp = await reviewSave;
    const saveBody = await saveResp.json();
    expect(saveBody.code).toBe(0);
  });

  test("上传 poster: 同一路径覆盖 poster 字段", async ({ page }) => {
    const fixture = await fetchFixtureJob();
    expect(fixture).not.toBeNull();
    const job = fixture!;
    await openFixtureReviewDetail(page, job);

    const tinyPngBuffer = Buffer.from(TINY_PNG_BASE64, "base64");
    const uploadCall = page.waitForResponse((res) =>
      res.url().endsWith("/api/assets/upload") && res.request().method() === "POST",
    );
    const reviewSave = page.waitForResponse((res) =>
      res.url().includes(`/api/review/jobs/${job.id}`) && res.request().method() === "PUT",
    );
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "上传海报" }).click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name: "e2e-poster.png",
      mimeType: "image/png",
      buffer: tinyPngBuffer,
    });
    const uploadResp = await uploadCall;
    expect((await uploadResp.json()).code).toBe(0);
    const saveResp = await reviewSave;
    expect((await saveResp.json()).code).toBe(0);
  });

  test("上传超 32 MiB: 前端 validateUploadSize 拦截, 不发起 /api/assets/upload 请求", async ({ page }) => {
    const fixture = await fetchFixtureJob();
    expect(fixture).not.toBeNull();
    const job = fixture!;
    await openFixtureReviewDetail(page, job);

    // 监听 /api/assets/upload 请求: 期望测试结束都不会触发. 用 closure 计数.
    let assetUploadCalls = 0;
    page.on("request", (req) => {
      if (req.url().endsWith("/api/assets/upload") && req.method() === "POST") {
        assetUploadCalls += 1;
      }
    });

    const oversize = Buffer.alloc(33 * 1024 * 1024);
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "上传封面" }).click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name: "huge.png",
      mimeType: "image/png",
      buffer: oversize,
    });

    // setMessage("图片不能超过 32 MiB"): review-shell 顶部的 message tone
    // 渲染在 ReviewDetailHeader; 只要 review 详情区域出现这串文案就算前端
    // 拦截成功. 用 page.getByText 等待文案显式出现, 防止 race.
    await expect(page.getByText("图片不能超过 32 MiB").first()).toBeVisible({ timeout: 5000 });

    // 等 1s 让任何潜在的 async upload 路径有机会发起请求.
    // 同步的 validateUploadSize 应该已经 return, 不会有 fetch 触发.
    await page.waitForTimeout(1000);
    expect(assetUploadCalls, "前端拦截后不应该再发任何 /api/assets/upload 请求").toBe(0);
  });

  test("裁剪 poster: 打开 cropper modal → 点 '截取' → POST /api/review/jobs/:id/poster-crop", async ({ page }) => {
    const fixture = await fetchFixtureJob();
    expect(fixture).not.toBeNull();
    const job = fixture!;

    // 前置: cropper 需要 cover 已存在 (openCropper 守 meta.cover != null).
    // 复用上一个 case 的上传, 但 spec 间不能假设顺序 — 这里独立上传一遍 cover.
    await openFixtureReviewDetail(page, job);
    {
      const tinyPngBuffer = Buffer.from(TINY_PNG_BASE64, "base64");
      const uploadCall = page.waitForResponse((res) =>
        res.url().endsWith("/api/assets/upload") && res.request().method() === "POST",
      );
      const reviewSave = page.waitForResponse((res) =>
        res.url().includes(`/api/review/jobs/${job.id}`) && res.request().method() === "PUT",
      );
      const chooserPromise = page.waitForEvent("filechooser");
      await page.getByRole("button", { name: "上传封面" }).click();
      const chooser = await chooserPromise;
      await chooser.setFiles({
        name: "e2e-cover-for-crop.png",
        mimeType: "image/png",
        buffer: tinyPngBuffer,
      });
      await uploadCall;
      await reviewSave;
    }

    // 打开 cropper modal: 工具按钮 aria-label="从封面截取海报".
    await page.getByRole("button", { name: "从封面截取海报" }).click();
    // ImageCropper 用 <Modal aria-label="从封面截取海报">, 等它出现.
    const dialog = page.getByRole("dialog", { name: "从封面截取海报" });
    await expect(dialog).toBeVisible();
    // 等 image onLoad 触发 setRect, "截取" 按钮才会显式渲染 (rect.width > 0).
    const cropConfirm = dialog.getByRole("button", { name: "截取" });
    await expect(cropConfirm).toBeVisible();

    const cropCall = page.waitForResponse((res) =>
      res.url().includes(`/api/review/jobs/${job.id}/poster-crop`)
      && res.request().method() === "POST",
    );
    await cropConfirm.click();
    const cropResp = await cropCall;
    const cropBody = await cropResp.json();
    expect(cropBody.code).toBe(0);
    expect(typeof cropBody.data?.key).toBe("string");
  });
});
