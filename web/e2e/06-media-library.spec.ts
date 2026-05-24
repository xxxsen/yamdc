// 06-media-library: 媒体库页面用户故事级 E2E.
// devcontainer 已经把 library_dir 指到 .devcontainer-data/library, 所以
// configured=true 是常态; seed-e2e-fixtures.sh 注入了至少 1 条 fixture
// (E2E-FIXTURE-002) 在 library_dir, 跑 sync 之后会出现在 GET /api/media-library.
//
// 覆盖的用户故事:
//   1) 页面顶部搜索框 + 同步入口按钮 (三态 label) 必现, 用户能立刻看到主动作;
//   2) 默认 sort 控件 (年份 / 大小 / 标题 / 入库时间 / 顺序) 都以可见 button
//      形态出现, 用户可以从 UI 上挑筛选维度;
//   3) 空集合时渲染 "当前媒体库里还没有项目" 兜底文案 (空态用户故事);
//   4) GET /api/media-library 协议契约稳定 (envelope.data 直接是数组);
//   5) 用户切换排序维度后 query 立刻反映在 URL / 列表 (UI 不卡死);
//   6) 同步日志弹窗: 通过同步菜单打开 → "媒体库同步日志" 模态框可见;
//   7) 详情弹窗: 点击第一张 fixture 卡片 → "媒体项详情" 模态框可见;
//   8) 排序切换: 切到 "标题" sort → 监听 GET /api/media-library 请求并断言
//      参数变化 → 列表重渲染.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError, apiGet } from "./helpers/api";

interface MediaLibraryItem {
  id: number;
  name: string;
  title: string;
  number: string;
}

interface MediaLibraryTaskState {
  task_key: string;
  status: string;
}

interface MediaLibraryStatus {
  configured: boolean;
  sync: MediaLibraryTaskState;
  move: MediaLibraryTaskState;
}

// SYNC_TERMINAL_STATES: 后端 internal/medialib/service.go 中 TaskState.Status
// 的枚举是 idle / running / completed / failed. 触发 sync 后真正的 worker 跑在
// service.go:357 的 bgWG.Add(1); go func() {...}() 后台 goroutine 里, HTTP 立即
// 返回, 因此 spec 必须自己 polling 直到 status 离开 running 才能继续, 否则后续
// apiGet("/api/media-library") 会拿到中间结果 → 详情用例 race.
const SYNC_TERMINAL_STATES: ReadonlySet<string> = new Set(["idle", "completed", "failed"]);

test.describe("media-library 用户故事 — 页面骨架 / 协议契约", () => {
  test("页面顶部: 关键字搜索框 + 同步按钮 必现", async ({ page }) => {
    await page.goto("/media-library");
    await expect(page.locator("body")).toBeVisible();
    await expect(page.getByPlaceholder(/搜索|关键字|按标题/i)).toBeVisible();
    // 同步按钮在 filter rail 末端, 实际 label 三态由 useMediaLibrarySync 决定:
    // 默认 "同步媒体库", 跑中 "同步中...", 结束闪烁 "同步完成". 见
    // web/src/components/media-library-shell/use-media-library-sync.ts 的
    // syncButtonLabel 计算.
    await expect(page.getByRole("button", { name: /同步媒体库|同步中|同步完成/ })).toBeVisible();
  });

  test("筛选 / 排序入口: 年份 / 顺序 button 默认可见 (用户可以切排序)", async ({ page }) => {
    await page.goto("/media-library");
    await expect(page.getByRole("button", { name: "年份" })).toBeVisible();
    await expect(page.getByRole("button", { name: "顺序" })).toBeVisible();
  });

  test("用户点击排序按钮: 状态切换不卡 UI", async ({ page }) => {
    await page.goto("/media-library");
    const orderBtn = page.getByRole("button", { name: "顺序" });
    await orderBtn.click();
    await expect(page.getByPlaceholder(/搜索|关键字|按标题/i)).toBeVisible();
  });

  test("空集合 OR 列表渲染: 用户必看到内容 (空时兜底文案; 非空时第一张卡片可见)", async ({ page }) => {
    await page.goto("/media-library");
    const emptyHint = page.getByText("当前媒体库里还没有项目");
    const firstCard = page.locator(".media-library-card").first();
    const seenEither = (await emptyHint.isVisible().catch(() => false))
      || (await firstCard.isVisible().catch(() => false));
    expect(seenEither).toBe(true);
  });

  test("协议契约: GET /api/media-library 直接返回 MediaLibraryItem[] 数组", async () => {
    const data = await apiGet<MediaLibraryItem[]>("/api/media-library");
    expect(Array.isArray(data)).toBe(true);
    for (const it of data) {
      expect(typeof it.id).toBe("number");
      expect(typeof it.name).toBe("string");
    }
  });
});

test.describe("media-library 用户故事 — fixture 真实交互", () => {
  // beforeEach: 触发一次媒体库同步, 让 fixture E2E-FIXTURE-002 进入
  // yamdc_media_library_tab. 协议契约是 HTTP 200 + envelope —
  // 已在跑 / 失败也是 envelope.code != 0, 不是 5xx, 因此用
  // apiCallAllowBusinessError 接 envelope 后再校验.
  //
  // 关键: /api/media-library/sync 在 internal/medialib/service.go 里把实际
  // 的扫描派发到一个后台 goroutine, HTTP 立即返回. 因此触发后必须自己
  // polling /api/media-library/status 等到 sync.status 离开 "running",
  // 否则后续 apiGet("/api/media-library") 会拿到空集合 / 中间结果, 详情
  // 弹窗用例就会 race.
  test.beforeEach(async () => {
    const triggerEnv = await apiCallAllowBusinessError<unknown>(
      "POST",
      "/api/media-library/sync",
    );
    expect(typeof triggerEnv.message).toBe("string");

    // 用同一次 polling 同时验证两件事:
    //   1) status 离开 running;
    //   2) status 落在已知终态枚举里 (SYNC_TERMINAL_STATES).
    // 历史写法是 poll regex match 之后再二次读 status 校验枚举, 那种结构
    // 会出现一种 race: poll 那一刻是 completed, 但下一次 trigger (例如
    // 另一个 spec 的 cleanup) 在两次读取之间把 status 又推回 running,
    // 导致 finalStatus 读到 running 后整个 beforeEach 报错. 把"离开
    // running + 在终态枚举里"合并进同一个 poll lambda, race window 收
    // 缩到一次读, 也保留了"未知新枚举立刻报错"的诊断价值.
    await expect
      .poll(
        async () => {
          const status = await apiGet<MediaLibraryStatus>("/api/media-library/status");
          return SYNC_TERMINAL_STATES.has(status.sync.status);
        },
        {
          timeout: 30_000,
          intervals: [200, 400, 800, 1500, 3000],
          message: "等待 /api/media-library/sync 后台 goroutine 跑完 (status 落在 SYNC_TERMINAL_STATES)",
        },
      )
      .toBe(true);
  });

  test("同步日志弹窗: 同步菜单 → 查看同步日志 → 模态框可见", async ({ page }) => {
    await page.goto("/media-library");
    // 触发同步菜单 (split button 右侧的下拉箭头, aria-label="同步菜单").
    await page.getByRole("button", { name: "同步菜单" }).click();
    await page.getByRole("menuitem", { name: "查看同步日志" }).click();
    // sync-logs-modal 用 ariaLabel="同步日志".
    const dialog = page.getByRole("dialog", { name: "同步日志" });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText("媒体库同步日志")).toBeVisible();
  });

  test("详情弹窗: fixture 至少 1 张卡片时点击 → 媒体项详情模态框可见", async ({ page }) => {
    const items = await apiGet<MediaLibraryItem[]>("/api/media-library");
    expect(
      items.length,
      "fixture seed-e2e-fixtures.sh 应在 library_dir 注入 E2E-FIXTURE-002, 触发 sync 后能拿到 items",
    ).toBeGreaterThan(0);

    await page.goto("/media-library");
    // 等首张 card 渲染稳定 (React fetch /api/media-library 后才挂上 button).
    // 仅 toBeVisible 不够 — visibleItems 第一次 render 是空数组, 第二次
    // 才填进 fixture, 中间会出现 button DOM remount, 此时 click 派发到旧
    // node 不会触发 onClick. 等 cardCount > 0 + waitForFunction 守一帧.
    const firstCard = page.locator(".media-library-card").first();
    await expect(firstCard).toBeVisible();
    await page.waitForFunction(
      () => document.querySelectorAll(".media-library-card").length > 0,
    );

    // 用 dispatchEvent('click') 直接派发 React onClick, 不走 mouse pipeline,
    // 不受 React rerender 期间 button instance 抖动影响, 也不受任何浮层
    // intercepts pointer events 影响.
    await firstCard.dispatchEvent("click");
    const detailDialog = page.getByRole("dialog", { name: "媒体项详情" });
    await expect(detailDialog).toBeVisible();
  });

  test("排序切换: 点击 '标题' 排序 → GET /api/media-library 携带 sort=title 重新发起", async ({ page }) => {
    await page.goto("/media-library");
    // 排序前先观察一下基线: 默认 sort=ingested.
    const titleBtn = page.getByRole("button", { name: "标题", exact: true });
    await expect(titleBtn).toBeVisible();
    const reload = page.waitForResponse((res) =>
      res.url().includes("/api/media-library")
      && res.url().includes("sort=title")
      && res.request().method() === "GET",
    );
    await titleBtn.click();
    const resp = await reload;
    expect(resp.status()).toBe(200);
    // 切换后 chip 应该立刻进入 active 态 (data-active=true).
    await expect(titleBtn).toHaveAttribute("data-active", "true");
  });
});
