// 03-review-assets: 上传资产路径的协议契约 + 32 MiB 上限.
//
// 重点验:
//   1) review asset 上传到不存在 job: HTTP 200 + envelope.code != 0
//      (业务错协议保持).
//   2) 上传图片超过 32 MiB: 后端用 http.MaxBytesReader 在协议层拒绝,
//      返回 HTTP 413 + envelope (P0/P1 修复后协议外的安全保护层, 详见
//      internal/web/jobs_routes.go 中 writeUploadTooLarge 的注释).
//
// 真实 multipart 在 fetch 里构造一个 33 MiB 的 buffer 即可触发上限.

import { expect, test } from "@playwright/test";

import { E2E_API_BASE_URL } from "./helpers/api";

test.describe("review assets", () => {
  test("review asset 不存在的 job: 协议保持 HTTP 200 + envelope.code != 0", async () => {
    // 任意小于 32 MiB 的合法 png 都行; 这里直接构造 8 字节空 png header.
    // 但因为 job 不存在, 后端会先在 dependency / id 层 reject.
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

  test("上传超过 32 MiB: 必须 HTTP 413 + envelope (协议外的安全保护层)", async () => {
    // 33 MiB > 32 MiB cap. 用 0 填充足够便宜.
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
