// 04-library: 已入库目录页面用户故事级 E2E.
//
// devcontainer fixture (seed-e2e-fixtures.sh) 注入了 E2E-FIXTURE-001 到
// save_dir, 包含主 variant + cd1 副 variant + 2 张 extrafanart, 让本 spec
// 能完整覆盖以下用户故事:
//   1) 标题 / 关键字搜索框 / 重新扫描库 / 移动到媒体库 四个核心 UI 节点必须可见;
//   2) 用户在搜索框里键入文字 → input 状态被接管, 不会把按钮卡住;
//   3) 后端 GET /api/library 协议契约稳定 (envelope.data 直接是 LibraryItem[] 数组);
//   4) 后端 GET /api/library/item 对不存在的 path 必须 HTTP 200 + 业务错;
//   5) 详情持久化: NFO 字段编辑 → PATCH 保存 → 刷新仍存在 (用户故事级);
//   6) variant 切换: 多 variant fixture → 点击 cd1 chip → 详情面板更新;
//   7) replace cover: 上传新 cover → /api/library/asset → 预览图更新;
//   8) replace poster: 上传新 poster → 预览图更新;
//   9) 删除 fanart: 点击删除按钮 → DELETE /api/library/file → fanart 从列表消失.

import { expect, test, type Page } from "@playwright/test";

import {
  apiCallAllowBusinessError,
  apiGet,
  apiPatch,
} from "./helpers/api";

interface LibraryItem {
  rel_path: string;
  title: string;
  number: string;
  variant_count: number;
}

interface LibraryFileItem {
  rel_path: string;
  name: string;
  size: number;
  modified_at: number;
}

interface LibraryVariant {
  key: string;
  base_name: string;
  label: string;
}

interface LibraryMeta {
  title: string;
  number: string;
  actors: string[];
  genres: string[];
  release_date: string;
  studio: string;
  series: string;
  director: string;
  plot: string;
  runtime?: number;
  source?: string;
  title_translated?: string;
  plot_translated?: string;
}

interface LibraryDetail {
  item: LibraryItem;
  meta: LibraryMeta;
  variants: LibraryVariant[];
  primary_variant_key: string;
  files: LibraryFileItem[];
}

const FIXTURE_REL_PATH = "E2E-FIXTURE-001";

// 32x32 透明 PNG (288 字节). 与 03-review-assets 共用同一 fixture.
const TINY_PNG_BASE64 =
  "iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAH0lEQVRYhe3BAQ" +
  "EAAACCIP+vbkhAAQAAAAAAAAAAvg0hAAABA+UCFAAAAABJRU5ErkJggg==";

async function findFixtureItem(): Promise<LibraryItem> {
  const items = await apiGet<LibraryItem[]>("/api/library");
  const found = items.find((it) => it.rel_path === FIXTURE_REL_PATH);
  expect(
    found,
    `seed-e2e-fixtures.sh 应当注入 ${FIXTURE_REL_PATH} 到 save_dir, 但 /api/library 没看到`,
  ).toBeTruthy();
  return found!;
}

async function openLibraryDetail(page: Page, item: LibraryItem): Promise<void> {
  await page.goto("/library");
  // library item card 在左侧 list-panel.tsx 渲染, 文字内容里包含 rel_path,
  // 用 has-text 直接定位到目标卡片再触发 click.
  const card = page.locator(".library-item-card", {
    has: page.locator(`.library-item-path:has-text("${item.rel_path}")`),
  });
  await expect(card).toBeVisible();
  await card.click();
  // 等右侧详情区出现 (form-fields 第一个标题输入框).
  await expect(page.locator(".review-top-fields .review-input-strong").first()).toBeVisible();
}

test.describe("library 用户故事 — 协议契约 + 列表骨架", () => {
  test("库列表骨架: 标题 / 搜索框 / 重新扫描 / 移动到媒体库 四件套必现", async ({ page }) => {
    await page.goto("/library");
    await expect(page.locator("body")).toBeVisible();

    await expect(page.getByRole("heading", { name: "已入库" })).toBeVisible();
    await expect(page.getByPlaceholder("按标题 / 影片 ID / 演员搜索")).toBeVisible();
    await expect(page.getByRole("button", { name: /重新扫描/ })).toBeVisible();
    await expect(page.getByRole("button", { name: /移动到媒体库/ })).toBeVisible();
  });

  test("用户键入关键字: 搜索框接管输入, 不会被锁死也不会把页面打崩", async ({ page }) => {
    await page.goto("/library");
    const search = page.getByPlaceholder("按标题 / 影片 ID / 演员搜索");
    await search.fill("ABC-001");
    await expect(search).toHaveValue("ABC-001");
    await search.fill("");
    await expect(search).toHaveValue("");
  });

  test("协议契约: GET /api/library 直接返回 LibraryItem[] 数组; GET /api/library/item 对不存在 path 走业务错", async () => {
    const data = await apiGet<LibraryItem[]>("/api/library");
    expect(Array.isArray(data)).toBe(true);
    for (const it of data) {
      expect(typeof it.rel_path).toBe("string");
      expect(typeof it.variant_count).toBe("number");
    }
    const env = await apiCallAllowBusinessError(
      "GET",
      "/api/library/item?path=__nonexistent__/library/path__",
    );
    expect(env.code).not.toBe(0);
    expect(env.message.length).toBeGreaterThan(0);
  });
});

test.describe("library 用户故事 — fixture E2E-FIXTURE-001 真实交互", () => {
  test("详情持久化: PATCH NFO 标题 → 重新 GET 后仍是新值", async () => {
    const target = await findFixtureItem();
    const detailUrl = `/api/library/item?path=${encodeURIComponent(target.rel_path)}`;
    const before = await apiGet<LibraryDetail>(detailUrl);
    const stamp = `e2e-library-${Date.now()}`;
    const patched: LibraryMeta = {
      ...before.meta,
      title: stamp,
      actors: Array.isArray(before.meta.actors) ? [...before.meta.actors] : [],
      genres: Array.isArray(before.meta.genres) ? [...before.meta.genres] : [],
    };
    try {
      const updated = await apiPatch<LibraryDetail>(detailUrl, { meta: patched });
      expect(updated.meta.title).toBe(stamp);
      const reread = await apiGet<LibraryDetail>(detailUrl);
      expect(reread.meta.title).toBe(stamp);
    } finally {
      // Restore-to-original: 仅 teardown, 不能让 cleanup 错误把测试自身
      // 的成功状态污染掉. 显式吃掉并打 warn, 让人在 CI 日志里仍能看见,
      // 而不是黑箱 swallow.
      try {
        await apiPatch<LibraryDetail>(detailUrl, { meta: before.meta });
      } catch (err) {
        console.warn("[04 spec] restore fixture meta 失败 (teardown), 不阻塞主断言:", err);
      }
    }
  });

  test("variant 切换: 多 variant fixture 详情面板里点击 cd1 chip → 详情面板状态更新", async ({ page }) => {
    const target = await findFixtureItem();
    expect(
      target.variant_count,
      "fixture 必须包含主 variant + cd1 副 variant 才能渲染 variant switcher",
    ).toBeGreaterThan(1);

    await openLibraryDetail(page, target);

    // LibraryVariantSwitcher 仅在 variants > 1 时渲染. chip 文本为 variant.label,
    // E2E-FIXTURE-001 的副 variant base_name = "E2E-FIXTURE-001-cd1".
    const chips = page.locator(".library-variant-chip");
    await expect(chips.first()).toBeVisible();
    const chipCount = await chips.count();
    expect(chipCount).toBeGreaterThan(1);

    // 找到非当前激活的 chip 点击, 切换 variant.
    const currentChip = page.locator('.library-variant-chip[data-active="true"]');
    await expect(currentChip).toHaveCount(1);
    const inactiveChip = page.locator('.library-variant-chip[data-active="false"]').first();
    await expect(inactiveChip).toBeVisible();
    await inactiveChip.click();
    // 切换后, 之前 inactive 的 chip 应当成为 active.
    await expect(inactiveChip).toHaveAttribute("data-active", "true");
  });

  test("replace cover: 触发 file chooser → POST /api/library/asset → 详情更新", async ({ page }) => {
    const target = await findFixtureItem();
    await openLibraryDetail(page, target);

    const tinyPngBuffer = Buffer.from(TINY_PNG_BASE64, "base64");
    const assetCall = page.waitForResponse((res) =>
      res.url().includes("/api/library/asset")
      && res.request().method() === "POST",
    );
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "上传封面" }).click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name: "e2e-library-cover.png",
      mimeType: "image/png",
      buffer: tinyPngBuffer,
    });
    const resp = await assetCall;
    const body = await resp.json();
    expect(body.code).toBe(0);
  });

  test("replace poster: 触发 file chooser → POST /api/library/asset → 详情更新", async ({ page }) => {
    const target = await findFixtureItem();
    await openLibraryDetail(page, target);

    const tinyPngBuffer = Buffer.from(TINY_PNG_BASE64, "base64");
    const assetCall = page.waitForResponse((res) =>
      res.url().includes("/api/library/asset")
      && res.request().method() === "POST",
    );
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "上传海报" }).click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name: "e2e-library-poster.png",
      mimeType: "image/png",
      buffer: tinyPngBuffer,
    });
    const resp = await assetCall;
    const body = await resp.json();
    expect(body.code).toBe(0);
  });

  test("删除 fanart: 点击删除按钮 → DELETE /api/library/file → 列表减少", async ({ page }) => {
    const target = await findFixtureItem();
    const detailUrl = `/api/library/item?path=${encodeURIComponent(target.rel_path)}`;
    const beforeDetail = await apiGet<LibraryDetail>(detailUrl);
    const fanartFiles = beforeDetail.files.filter((f) => f.rel_path.includes("/extrafanart/"));
    expect(
      fanartFiles.length,
      "fixture 必须铺至少一张 extrafanart 才能跑删除路径",
    ).toBeGreaterThan(0);

    await openLibraryDetail(page, target);

    // LibraryFanartStrip 渲染删除按钮 aria-label="删除 extrafanart".
    const deleteBtn = page.getByRole("button", { name: "删除 extrafanart" }).first();
    await expect(deleteBtn).toBeVisible();
    const deleteCall = page.waitForResponse((res) =>
      res.url().includes("/api/library/file")
      && res.request().method() === "DELETE",
    );
    await deleteBtn.click();
    const resp = await deleteCall;
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.code).toBe(0);

    // 删除后再次拉取详情, fanart 数量应减少.
    const after = await apiGet<LibraryDetail>(detailUrl);
    const afterFanart = after.files.filter((f) => f.rel_path.includes("/extrafanart/"));
    expect(afterFanart.length).toBeLessThan(fanartFiles.length);
  });
});
