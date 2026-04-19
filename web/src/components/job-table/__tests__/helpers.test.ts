import { describe, expect, it } from "vitest";

import type { JobItem } from "@/lib/api";

import {
  canSelectJob,
  compareJobsByNumber,
  getNumberHint,
  getNumberMeta,
  getPathSegments,
  jobSortKey,
  requiresManualNumberReview,
} from "../helpers";

function makeJob(partial: Partial<JobItem> = {}): JobItem {
  return {
    id: 1,
    job_uid: "uid",
    file_name: "a.mp4",
    file_ext: ".mp4",
    rel_path: "dir/a.mp4",
    abs_path: "/root/dir/a.mp4",
    number: "",
    raw_number: "",
    cleaned_number: "",
    number_source: "auto",
    number_clean_status: "success",
    number_clean_confidence: "high",
    number_clean_warnings: "",
    file_size: 1024,
    status: "init",
    error_msg: "",
    created_at: 0,
    updated_at: 0,
    conflict_reason: "",
    conflict_target: "",
    ...partial,
  };
}

describe("jobSortKey", () => {
  it("正常 case: 优先级 number > raw_number > cleaned_number", () => {
    expect(jobSortKey(makeJob({ number: "A-1", raw_number: "B", cleaned_number: "C" }))).toBe("A-1");
    expect(jobSortKey(makeJob({ number: "", raw_number: "B", cleaned_number: "C" }))).toBe("B");
    expect(jobSortKey(makeJob({ number: "", raw_number: "", cleaned_number: "C" }))).toBe("C");
  });

  it("边缘 case: 三者都空返回空串, 首尾空格被 trim", () => {
    expect(jobSortKey(makeJob({ number: "", raw_number: "", cleaned_number: "" }))).toBe("");
    expect(jobSortKey(makeJob({ number: "  X-1 " }))).toBe("X-1");
  });
});

describe("compareJobsByNumber", () => {
  it("正常 case: 按 number 数字感知排序, ABC-2 在 ABC-10 前", () => {
    const a = makeJob({ id: 1, number: "ABC-10" });
    const b = makeJob({ id: 2, number: "ABC-2" });
    expect(compareJobsByNumber(a, b)).toBeGreaterThan(0);
    expect(compareJobsByNumber(b, a)).toBeLessThan(0);
  });

  it("异常 case: 一方无 number 时, 有 number 的排前, 两方都无时按 id DESC", () => {
    const withNum = makeJob({ id: 5, number: "X-1" });
    const noNum = makeJob({ id: 10, number: "", raw_number: "", cleaned_number: "" });
    expect(compareJobsByNumber(withNum, noNum)).toBeLessThan(0);
    expect(compareJobsByNumber(noNum, withNum)).toBeGreaterThan(0);

    const a = makeJob({ id: 1, number: "" });
    const b = makeJob({ id: 2, number: "" });
    expect(compareJobsByNumber(a, b)).toBe(1);
    expect(compareJobsByNumber(b, a)).toBe(-1);
  });

  it("边缘 case: 同 number 按 id DESC 兜底", () => {
    const a = makeJob({ id: 1, number: "X-1" });
    const b = makeJob({ id: 2, number: "X-1" });
    expect(compareJobsByNumber(a, b)).toBe(1);
    expect(compareJobsByNumber(b, a)).toBe(-1);
  });
});

describe("canSelectJob", () => {
  it("正常 case: manual 源 + init 可选", () => {
    expect(canSelectJob(makeJob({ number_source: "manual", status: "init" }))).toBe(true);
  });

  it("正常 case: 自动 high 置信度 + failed 可重试也可选", () => {
    expect(
      canSelectJob(
        makeJob({
          number_source: "auto",
          number_clean_status: "success",
          number_clean_confidence: "high",
          status: "failed",
        }),
      ),
    ).toBe(true);
  });

  it("异常 case: conflict_reason 非空直接排除", () => {
    expect(
      canSelectJob(
        makeJob({
          number_source: "manual",
          status: "init",
          conflict_reason: "duplicate",
        }),
      ),
    ).toBe(false);
  });

  it("边缘 case: low 置信度 / processing 状态不可选", () => {
    expect(canSelectJob(makeJob({ number_clean_confidence: "low" }))).toBe(false);
    expect(canSelectJob(makeJob({ status: "processing" }))).toBe(false);
    expect(canSelectJob(makeJob({ status: "reviewing" }))).toBe(false);
  });
});

describe("getNumberMeta", () => {
  it("正常 case: manual → info tone, manual kind", () => {
    const meta = getNumberMeta(makeJob({ number_source: "manual" }));
    expect(meta.kind).toBe("manual");
    expect(meta.tone).toMatch(/info/);
  });

  it("正常 case: success + high → success kind", () => {
    const meta = getNumberMeta(
      makeJob({ number_clean_status: "success", number_clean_confidence: "high" }),
    );
    expect(meta.kind).toBe("success");
  });

  it("正常 case: success + medium → warn, 透传 warnings", () => {
    const meta = getNumberMeta(
      makeJob({
        number_clean_status: "success",
        number_clean_confidence: "medium",
        number_clean_warnings: "模糊匹配",
      }),
    );
    expect(meta.kind).toBe("warn");
    expect(meta.warning).toBe("模糊匹配");
  });

  it("异常 case: no_match / low_quality / low 置信度 → danger", () => {
    expect(getNumberMeta(makeJob({ number_clean_status: "no_match" })).kind).toBe("danger");
    expect(getNumberMeta(makeJob({ number_clean_status: "low_quality" })).kind).toBe("danger");
    expect(
      getNumberMeta(
        makeJob({
          number_clean_status: "unknown",
          number_clean_confidence: "low",
        }),
      ).kind,
    ).toBe("danger");
  });

  it("边缘 case: 完全未知状态兜底到 warn", () => {
    const meta = getNumberMeta(
      makeJob({
        number_source: "auto",
        number_clean_status: "pending",
        number_clean_confidence: "",
      }),
    );
    expect(meta.kind).toBe("warn");
  });
});

describe("requiresManualNumberReview", () => {
  it("正常 case: manual 源永远不需要人审", () => {
    expect(requiresManualNumberReview(makeJob({ number_source: "manual" }))).toBe(false);
  });

  it("异常 case: no_match / low_quality / low 置信度需要人审", () => {
    expect(requiresManualNumberReview(makeJob({ number_clean_status: "no_match" }))).toBe(true);
    expect(requiresManualNumberReview(makeJob({ number_clean_status: "low_quality" }))).toBe(true);
    expect(requiresManualNumberReview(makeJob({ number_clean_confidence: "low" }))).toBe(true);
  });

  it("边缘 case: success + high 不需要人审", () => {
    expect(
      requiresManualNumberReview(
        makeJob({ number_clean_status: "success", number_clean_confidence: "high" }),
      ),
    ).toBe(false);
  });
});

describe("getNumberHint", () => {
  it("正常 case: conflict 优先提示目标冲突", () => {
    expect(getNumberHint(makeJob({ conflict_reason: "duplicate" }))).toBe("目标文件名冲突，需先处理");
  });

  it("正常 case: manual 源提示已手动确认", () => {
    expect(getNumberHint(makeJob({ number_source: "manual" }))).toBe("已手动确认");
  });

  it("正常 case: success + high / medium 分别返回不同文案", () => {
    expect(
      getNumberHint(makeJob({ number_clean_status: "success", number_clean_confidence: "high" })),
    ).toBe("高置信度，可直接提交");
    expect(
      getNumberHint(
        makeJob({
          number_clean_status: "success",
          number_clean_confidence: "medium",
          number_clean_warnings: "多候选",
        }),
      ),
    ).toBe("多候选");
  });

  it("边缘 case: 其他情况无 warnings 返回默认文案", () => {
    expect(
      getNumberHint(
        makeJob({
          number_source: "auto",
          number_clean_status: "no_match",
          number_clean_warnings: "",
        }),
      ),
    ).toBe("需先手动修正影片 ID");
  });
});

describe("getPathSegments", () => {
  it("正常 case: 多级路径拆分 folder / name", () => {
    const { folder, name } = getPathSegments(makeJob({ rel_path: "a/b/c.mp4" }));
    expect(folder).toBe("a / b");
    expect(name).toBe("c.mp4");
  });

  it("边缘 case: 根目录文件 folder 为 '根目录'", () => {
    const { folder, name } = getPathSegments(makeJob({ rel_path: "c.mp4" }));
    expect(folder).toBe("根目录");
    expect(name).toBe("c.mp4");
  });

  it("异常 case: 空路径 / 纯分隔符回退到 rel_path", () => {
    const { folder, name } = getPathSegments(makeJob({ rel_path: "" }));
    expect(folder).toBe("根目录");
    expect(name).toBe("");

    // "/" → split+filter 之后 segments 为空, name 回退到 rel_path 原值
    const { folder: f2, name: n2 } = getPathSegments(makeJob({ rel_path: "/" }));
    expect(f2).toBe("根目录");
    expect(n2).toBe("/");
  });
});
