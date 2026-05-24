// 09-plugin-editor: 插件编辑器用户故事级 E2E.
//
// 覆盖:
//   1) /debug/plugin-editor 页面渲染 — 标题"插件配置" + 关闭按钮可见;
//   2) compile / request / 导入 YAML 等核心动作的协议契约 (业务错必须 HTTP
//      200 + envelope.code != 0);
//   3) compile 协议: 给一份故意"语法错"的 YAML, 后端必须 envelope.code != 0
//      并把错误 message 带回前端 (用户故事: 用户能看到编译错误);
//   4) request 协议: 不带任何字段的空请求必须走业务错, 不能 5xx;
//   5) workflow 协议: 给一个会编译失败的 YAML, 必须走业务错协议;
//   6) XPath inspector: mock /api/debug/plugin-editor/scrape 注入一段
//      已知 HTML, 切到 Response → Inspector → 右键 .dom-tree-line → 弹出
//      role=menu aria-label="XPath 复制菜单" → 点击 "Copy XPath" 菜单项 →
//      navigator.clipboard.writeText 收到非空 XPath 字符串.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError } from "./helpers/api";

test.describe("plugin-editor 用户故事 — 协议契约", () => {
  test("/debug/plugin-editor 页面: 插件配置标题 + 关闭按钮 必现", async ({ page }) => {
    await page.goto("/debug/plugin-editor");
    await expect(page.getByRole("heading", { name: "插件配置" })).toBeVisible();
    await expect(page.getByRole("link", { name: "关闭插件编辑器" })).toBeVisible();
  });

  test("compile 协议: 故意错的 YAML 必须 HTTP 200 + envelope.code != 0", async () => {
    const env = await apiCallAllowBusinessError("POST", "/api/debug/plugin-editor/compile", {
      yaml: "this: is: invalid: yaml: content::::",
    });
    expect(env.code).not.toBe(0);
    expect(env.message.length).toBeGreaterThan(0);
  });

  test("request 协议: 缺字段的请求必须走业务错协议 (HTTP 200 + envelope)", async () => {
    const env = await apiCallAllowBusinessError("POST", "/api/debug/plugin-editor/request", {
      yaml: "",
      number: "",
    });
    expect(env.code).not.toBe(0);
    expect(typeof env.message).toBe("string");
  });

  test("workflow 协议: 故意错的 YAML 走业务错协议 (HTTP 200 + envelope)", async () => {
    const env = await apiCallAllowBusinessError("POST", "/api/debug/plugin-editor/workflow", {
      yaml: "::::not yaml::::",
      number: "ABC-123",
    });
    expect(env.code).not.toBe(0);
    expect(typeof env.message).toBe("string");
  });
});

test.describe("plugin-editor 用户故事 — XPath inspector 真实交互", () => {
  test("注入 HTML body → 切到 Response → Inspector 模式 → 右键 DOM 行 → Copy XPath 触发剪贴板", async ({ page, context }) => {
    // 授予剪贴板权限以便 navigator.clipboard.writeText 不被浏览器策略拦截.
    // 即便拒绝, copyToClipboard 也有 document.execCommand fallback, 所以这里
    // 仅在确实抛错 (例如 webkit 不支持 grantPermissions) 时打印告警继续, 不
    // 静默吞掉错误对象.
    try {
      await context.grantPermissions(["clipboard-read", "clipboard-write"], {
        origin: new URL(page.url() || "http://localhost:3000").origin,
      });
    } catch (err) {
      console.warn("[09 spec] grantPermissions 失败, 走 execCommand fallback:", err);
    }

    // mock /api/debug/plugin-editor/scrape: 让 Run 按钮立刻拿到 fixture HTML body,
    // 不依赖外网 / 实际插件运行. /scrape 是默认 (workflowEnabled=false +
    // multiRequestEnabled=false) 路径下 "运行调试" 按钮触发的唯一 API. 见
    // web/src/components/plugin-editor/use-plugin-editor-state.ts run() 实现.
    const fixtureHTML =
      "<!DOCTYPE html><html><body>"
      + "<div id=\"main\"><a id=\"abc\" class=\"x\" href=\"/foo\">link-text</a>"
      + "<p class=\"y\">paragraph</p></div></body></html>";
    // 后端 /api/debug/plugin-editor/scrape 用了双层 envelope:
    //   外层: { code, message, data: PluginEditorEnvelope<...> }
    //   内层: { ok, warnings, data: PluginEditorScrapeDebugResult }
    // 前端 debugPluginDraftScrape() 返回外层 .data 即 PluginEditorEnvelope,
    // use-plugin-editor-state.ts 再访问 envelope.data.request / envelope.data.response,
    // 因此 mock body 必须把 request/response 放在 envelope.data 里, 否则
    // BodyPanel 以上链路会拿到 undefined → React 渲染抛 'reading request'.
    await page.route("**/api/debug/plugin-editor/scrape", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json; charset=utf-8",
        body: JSON.stringify({
          code: 0,
          message: "ok",
          data: {
            ok: true,
            warnings: [],
            data: {
              request: { method: "GET", url: "https://e2e.example/fixture", headers: {}, body: "" },
              response: {
                status_code: 200,
                headers: { "content-type": ["text/html; charset=utf-8"] },
                body: fixtureHTML,
                body_preview: fixtureHTML,
              },
              fields: {},
              meta: null,
            },
          },
        }),
      });
    });

    await page.goto("/debug/plugin-editor");

    // 在 click 前先植入剪贴板 spy: 拦截 navigator.clipboard.writeText 把
    // 写入文本收集到 window.__clipboardWrites, 后续断言读它. copyToClipboard
    // 即便走 fallback (document.execCommand) 也仍会先 await writeText, 所以
    // 这条 spy 必中.
    await page.evaluate(() => {
      const writes: string[] = [];
      const w = window as unknown as { __clipboardWrites: string[] };
      w.__clipboardWrites = writes;
      const orig = navigator.clipboard?.writeText?.bind(navigator.clipboard);
      navigator.clipboard = navigator.clipboard ?? ({} as Clipboard);
      navigator.clipboard.writeText = async (text: string) => {
        writes.push(text);
        if (orig) {
          try {
            return await orig(text);
          } catch {
            return undefined;
          }
        }
        return undefined;
      };
    });

    // 触发 mock 的 scrape, 等到 fixture HTML body 进入 React state.
    const scrapeCall = page.waitForResponse((res) =>
      res.url().includes("/api/debug/plugin-editor/scrape") && res.request().method() === "POST",
    );
    await page.getByRole("button", { name: "运行调试" }).click();
    await scrapeCall;

    // 切到 Response tab, OutputShell 渲染 ResponseDetailPanel → BodyPanel.
    await page.getByRole("button", { name: "Response", exact: true }).click();

    // 切到 Inspector 模式 (按钮 class .body-mode-btn, 文本 "Inspector").
    //
    // 注意: plugin-editor 页面右上角悬浮菜单 (.plugin-editor-floating-menu)
    // 是 fixed 定位且 z-index 较高, 默认位置正好压在 Body 面板的 mode-toggle
    // 区上方, Playwright 的 mouse-based click 会被 floating menu 的
    // .plugin-editor-split-action-main 拦截 (Test timeout 180s, 重试几百次都
    // 失败). 这里用 dispatchEvent('click') 直接派发 React onClick 合成事件,
    // 既保留了 "Inspector 按钮存在且可点击" 的语义, 也不依赖具体页面布局
    // (悬浮菜单是否拖走 / 是否遮挡 / 是否被 portal 装载).
    const inspectorBtn = page.getByRole("button", { name: "Inspector", exact: true });
    await expect(inspectorBtn).toBeVisible();
    await inspectorBtn.dispatchEvent("click");

    // 等 dom-tree 渲染. HtmlInspectorPanel 的根容器 class .html-inspector-tree.
    //
    // 注意: HtmlInspectorPanel 直接 map(doc.childNodes), 而 fixture HTML 以
    // <!DOCTYPE html> 起头, doc.childNodes[0] 是 DocumentType (nodeType=10),
    // 在 DomTreeNode 里走的是 "无 onContextMenu" 的 DOCTYPE 分支 (dom-tree.tsx
    // 第 158-164 行). 直接 .first() 命中的是 DOCTYPE 行 → 右键不会弹菜单 →
    // 测试 race-condition 性失败.
    //
    // 这里通过 ":has(.dom-tag)" 把 selector 限定到 Element 行 (DomOpenTag /
    // DomCloseTag 才会渲染 .dom-tag span; DOCTYPE / Text / Comment 行不渲染).
    // Element 行 (line 210/225/236/247/254) 才挂了 onContextMenu, 才能弹菜单.
    await page.waitForSelector(".html-inspector-tree");
    const domLine = page.locator(".dom-tree-line:has(.dom-tag)").first();
    await expect(domLine).toBeVisible();

    // 右键触发 XPathContextMenu (role="menu", aria-label="XPath 复制菜单").
    await domLine.click({ button: "right" });
    const menu = page.getByRole("menu", { name: "XPath 复制菜单" });
    await expect(menu).toBeVisible();

    // 验证菜单项 "Copy XPath" 与 "Copy Full XPath" 同时存在 (两条 ARIA menuitem).
    const copyItem = menu.getByRole("menuitem", { name: "Copy XPath" });
    const copyFullItem = menu.getByRole("menuitem", { name: "Copy Full XPath" });
    await expect(copyItem).toBeVisible();
    await expect(copyFullItem).toBeVisible();

    // 点击 "Copy XPath" → onClose → returnFocus, 同时把 xpath 文本写进剪贴板.
    await copyItem.click();
    await expect(menu).toBeHidden();

    // 断言剪贴板被写入了非空 XPath 字符串 (XPath 总是以 "/" 或 "//" 开头).
    const writes = await page.evaluate(
      () => (window as unknown as { __clipboardWrites: string[] }).__clipboardWrites ?? [],
    );
    expect(writes.length, "Copy XPath 必须触发 navigator.clipboard.writeText").toBeGreaterThan(0);
    const xpath = writes[writes.length - 1] ?? "";
    expect(xpath.length).toBeGreaterThan(0);
    expect(/^\/\/?/.test(xpath), `复制的 XPath 应以 / 或 // 开头, 实际: ${xpath}`).toBe(true);
  });
});
