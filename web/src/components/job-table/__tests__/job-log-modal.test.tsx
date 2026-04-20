// @vitest-environment jsdom

import { act } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { JobItem, JobLogItem } from "@/lib/api";

import { mount } from "@/components/ui/__tests__/test-helpers";

import { JobLogModal } from "../job-log-modal";

// JobLogModal 的职责只有两件:
//   1. 按 created_at 逆序展示日志, 让用户打开弹窗第一眼就能看到最新那条;
//   2. 背景点击 / 关闭按钮触发 onClose (review-preview-overlay / Button)。
// 滚动行为是纯 CSS (flex + min-height:0 + overflow-y:auto), 单元测试不再
// 复验运行时滚动效果 — jsdom 没 layout, assert 这个会流于形式; 这里只 cover
// 排序与关闭回调这种有决定性行为的部分。

function makeJob(id = 42): JobItem {
  return {
    id,
    rel_path: "movies/demo.mp4",
    number: "ABC-001",
  } as unknown as JobItem;
}

function makeLog(partial: Partial<JobLogItem> & { id: number; created_at: number }): JobLogItem {
  return {
    job_id: 42,
    level: "info",
    stage: "run",
    message: "msg",
    detail: "",
    ...partial,
  } as JobLogItem;
}

afterEach(() => {
  document.querySelectorAll(".review-preview-overlay").forEach((el) => el.remove());
});

describe("JobLogModal", () => {
  it("按 created_at 逆序渲染: 最新日志排在 DOM 顶部", () => {
    const logs: JobLogItem[] = [
      makeLog({ id: 1, created_at: 1000, message: "first" }),
      makeLog({ id: 2, created_at: 3000, message: "latest" }),
      makeLog({ id: 3, created_at: 2000, message: "middle" }),
    ];
    const { unmount } = mount(
      <JobLogModal job={makeJob()} logs={logs} message="" onClose={vi.fn()} />,
    );
    const items = Array.from(document.querySelectorAll(".file-log-item .file-log-text"));
    expect(items.map((n) => n.textContent)).toEqual(["latest", "middle", "first"]);
    unmount();
  });

  it("同一 created_at 时: 用 id 做 tiebreaker (大 id 在前)", () => {
    const logs: JobLogItem[] = [
      makeLog({ id: 10, created_at: 5000, message: "older" }),
      makeLog({ id: 11, created_at: 5000, message: "newer" }),
    ];
    const { unmount } = mount(
      <JobLogModal job={makeJob()} logs={logs} message="" onClose={vi.fn()} />,
    );
    const items = Array.from(document.querySelectorAll(".file-log-item .file-log-text"));
    expect(items.map((n) => n.textContent)).toEqual(["newer", "older"]);
    unmount();
  });

  it("overlay 点击触发 onClose, dialog 内部点击不触发", () => {
    const onClose = vi.fn();
    const { unmount } = mount(
      <JobLogModal job={makeJob()} logs={[]} message="" onClose={onClose} />,
    );
    const overlay = document.querySelector(".review-preview-overlay") as HTMLElement;
    const dialog = document.querySelector(".file-log-dialog") as HTMLElement;

    act(() => {
      dialog.click();
    });
    expect(onClose).not.toHaveBeenCalled();

    act(() => {
      overlay.click();
    });
    expect(onClose).toHaveBeenCalledTimes(1);
    unmount();
  });

  it("message 非空时渲染 file-log-message, 空时不渲染", () => {
    const { unmount } = mount(
      <JobLogModal job={makeJob()} logs={[]} message="日志加载中..." onClose={vi.fn()} />,
    );
    expect(document.querySelector(".file-log-message")!.textContent).toBe("日志加载中...");
    unmount();

    const { unmount: u2 } = mount(
      <JobLogModal job={makeJob()} logs={[]} message="" onClose={vi.fn()} />,
    );
    expect(document.querySelector(".file-log-message")).toBeNull();
    u2();
  });

  it("detail 字段非空时才渲染 file-log-detail", () => {
    const logs: JobLogItem[] = [
      makeLog({ id: 1, created_at: 1, detail: "stack trace..." }),
      makeLog({ id: 2, created_at: 2, detail: "" }),
    ];
    const { unmount } = mount(
      <JobLogModal job={makeJob()} logs={logs} message="" onClose={vi.fn()} />,
    );
    const details = Array.from(document.querySelectorAll(".file-log-detail"));
    expect(details).toHaveLength(1);
    expect(details[0].textContent).toBe("stack trace...");
    unmount();
  });
});
