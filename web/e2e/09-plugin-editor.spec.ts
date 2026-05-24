// 09-plugin-editor: 插件编辑器用户故事级 E2E.
//
// 覆盖:
//   1) /debug/plugin-editor 页面渲染 — 标题"插件配置" + 关闭按钮可见;
//   2) compile / request / 导入 YAML 等核心动作的协议契约 (业务错必须 HTTP
//      200 + envelope.code != 0);
//   3) compile 协议: 给一份故意"语法错"的 YAML, 后端必须 envelope.code != 0
//      并把错误 message 带回前端 (用户故事: 用户能看到编译错误);
//   4) request 协议: 不带任何字段的空请求必须走业务错, 不能 5xx;
//   5) workflow 协议: 给一个会编译失败的 YAML, 必须走业务错协议.
//
// 注: XPath inspector 展开 / 复制菜单的 a11y 已经在 vitest dom-tree.test.tsx
// 里覆盖, 这里不再 E2E 验菜单交互, 把覆盖权交给单测.

import { expect, test } from "@playwright/test";

import { apiCallAllowBusinessError } from "./helpers/api";

test.describe("plugin-editor 用户故事", () => {
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
