import { describe, expect, it } from "vitest";

import { cn, formatBytes, formatUnixMillis } from "../utils";

describe("cn", () => {
  it("合并普通 class 字符串", () => {
    expect(cn("a", "b", "c")).toBe("a b c");
  });

  it("过滤 false / null / undefined", () => {
    expect(cn("a", false, null, undefined, "b")).toBe("a b");
  });

  it("利用 tailwind-merge 消除冲突 utility, 后者覆盖前者", () => {
    expect(cn("p-2", "p-4")).toBe("p-4");
    expect(cn("text-red-500", "text-blue-500")).toBe("text-blue-500");
  });

  it("空输入返回空字符串", () => {
    expect(cn()).toBe("");
    expect(cn(null, undefined, false)).toBe("");
  });
});

describe("formatBytes", () => {
  it("正常 case: 按 1024 进制换算并保留合适小数位", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(500)).toBe("500 B");
    expect(formatBytes(1024)).toBe("1.0 KB");
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatBytes(1024 * 1024)).toBe("1.0 MB");
    expect(formatBytes(1024 * 1024 * 1024)).toBe("1.0 GB");
  });

  it("≥ 10 的单位数量不保留小数, 避免伪精度", () => {
    expect(formatBytes(10 * 1024)).toBe("10 KB");
    expect(formatBytes(20 * 1024 * 1024)).toBe("20 MB");
  });

  it("超出 TB 后停在最大单位不继续跃迁", () => {
    const oneTB = 1024 ** 4;
    expect(formatBytes(oneTB)).toBe("1.0 TB");
    expect(formatBytes(oneTB * 2048)).toMatch(/TB$/);
  });

  it("异常 case: 非法 / 非正数输入返回 '0 B'", () => {
    expect(formatBytes(-1)).toBe("0 B");
    expect(formatBytes(NaN)).toBe("0 B");
    expect(formatBytes(Number.POSITIVE_INFINITY)).toBe("0 B");
    expect(formatBytes(Number.NEGATIVE_INFINITY)).toBe("0 B");
  });

  it("边缘 case: 0 字节 / 恰好在进位阈值上", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1023)).toBe("1023 B");
    expect(formatBytes(1024)).toBe("1.0 KB");
  });
});

describe("formatUnixMillis", () => {
  it("正常 case: 格式化为上海时区 YYYY/MM/DD HH:MM", () => {
    // 2024-01-02T03:04:05Z = 上海时间 2024-01-02 11:04
    const ts = Date.UTC(2024, 0, 2, 3, 4, 5);
    const out = formatUnixMillis(ts);
    expect(out).toMatch(/2024/);
    expect(out).toMatch(/01/);
    expect(out).toMatch(/02/);
    expect(out).toMatch(/11:04/);
  });

  it("异常 case: 0 / 未传 / 假值返回 '-'", () => {
    expect(formatUnixMillis(0)).toBe("-");
  });

  it("边缘 case: Unix epoch 正整数应正常格式化", () => {
    const out = formatUnixMillis(1);
    expect(out).not.toBe("-");
    expect(out.length).toBeGreaterThan(0);
  });
});
