// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import type { JobItem } from "@/lib/api";

import { NumberStatusIcon } from "../number-status-icon";

// NumberStatusIcon 是按 getNumberMeta(job).kind 派出 4 种圆形图标 (success /
// manual / warn / danger). 颜色 + icon + title 都跟 kind 绑死, 任何 kind
// 分支被改坏都会让用户无法分辨任务状态. 这里覆盖三类:
//   - 正常: success / warn / manual
//   - 异常: danger 默认文案 / danger 自定义 warning
//   - 边缘: number_clean_warnings 为空时的兜底文案

afterEach(() => {
  cleanup();
});

function makeJob(overrides: Partial<JobItem>): JobItem {
  return {
    id: 1,
    rel_path: "x.mp4",
    number: "ABC-001",
    status: "init",
    file_size: 0,
    updated_at: 0,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(overrides as any),
  } as JobItem;
}

describe("NumberStatusIcon", () => {
  it("success: 高置信度 → title=清洗成功，高置信度", () => {
    const job = makeJob({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      number_clean_status: "success" as any,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      number_clean_confidence: "high" as any,
    });
    const { container } = render(<NumberStatusIcon job={job} />);
    expect(container.querySelector("span")?.getAttribute("title")).toBe("清洗成功，高置信度");
  });

  it("manual: number_source=manual → title='用户已手动编辑影片 ID'", () => {
    const job = makeJob({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      number_source: "manual" as any,
    });
    const { container } = render(<NumberStatusIcon job={job} />);
    expect(container.querySelector("span")?.getAttribute("title")).toBe("用户已手动编辑影片 ID");
  });

  it("warn: 中置信度 → title=清洗成功，中等置信度", () => {
    const job = makeJob({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      number_clean_status: "success" as any,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      number_clean_confidence: "medium" as any,
    });
    const { container } = render(<NumberStatusIcon job={job} />);
    expect(container.querySelector("span")?.getAttribute("title")).toBe("清洗成功，中等置信度");
  });

  it("danger: 没有 cleaning 信息 → 兜底文案 (warn 类型, 无 warning 时为空 string)", () => {
    // 注意: getNumberMeta 在没有任何 cleaning meta 时落到 default warn 分支,
    // warning 为空 string. 该分支属于"清洗状态未知"的过渡态, title 为空 string,
    // 此时不应与 danger/manual 任何文案冲突.
    const job = makeJob({});
    const { container } = render(<NumberStatusIcon job={job} />);
    const span = container.querySelector("span");
    // warn 分支的 title 为空 string (即没有 title 属性时 getAttribute 返回 null
    // 或空 string), 关键是不能被误判成 success/manual/danger 的具体文案.
    const title = span?.getAttribute("title") ?? "";
    expect(title).not.toContain("高置信度");
    expect(title).not.toContain("手动编辑");
    expect(title).not.toContain("失败");
  });

  it("danger: low_quality + 自定义 warning → title 优先使用 number_clean_warnings", () => {
    const job = makeJob({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      number_clean_status: "low_quality" as any,
      number_clean_warnings: "无法识别番号格式",
    });
    const { container } = render(<NumberStatusIcon job={job} />);
    expect(container.querySelector("span")?.getAttribute("title")).toBe("无法识别番号格式");
  });

  it("danger: no_match + 无 warning → 兜底 '清洗失败，当前使用原始值'", () => {
    const job = makeJob({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      number_clean_status: "no_match" as any,
    });
    const { container } = render(<NumberStatusIcon job={job} />);
    expect(container.querySelector("span")?.getAttribute("title")).toBe("清洗失败，当前使用原始值");
  });
});
