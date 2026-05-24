import { describe, expect, it } from "vitest";

import {
  MAX_UPLOAD_IMAGE_BYTES,
  UPLOAD_TOO_LARGE_MESSAGE,
  validateUploadSize,
} from "@/lib/upload-limits";

function makeFile(size: number): File {
  // Create a sparse-ish File using a Uint8Array placeholder so we cover
  // both happy path and oversized boundary without allocating real 32 MiB
  // when not needed; size is set via the underlying Blob bits.
  const bits = new Uint8Array(Math.min(size, 16));
  const file = new File([bits], "fake.png", { type: "image/png" });
  Object.defineProperty(file, "size", { value: size });
  return file;
}

describe("validateUploadSize", () => {
  it("approves a small image (正常路径)", () => {
    const result = validateUploadSize(makeFile(1024));
    expect(result.ok).toBe(true);
    expect(result.message).toBeUndefined();
  });

  it("rejects a file just above the limit (异常路径)", () => {
    const result = validateUploadSize(makeFile(MAX_UPLOAD_IMAGE_BYTES + 1));
    expect(result.ok).toBe(false);
    expect(result.message).toBe(UPLOAD_TOO_LARGE_MESSAGE);
  });

  it("approves a file exactly at the limit (边缘路径)", () => {
    const result = validateUploadSize(makeFile(MAX_UPLOAD_IMAGE_BYTES));
    expect(result.ok).toBe(true);
  });

  it("rejects a much larger file with the same warning copy (边缘路径)", () => {
    const result = validateUploadSize(makeFile(MAX_UPLOAD_IMAGE_BYTES * 4));
    expect(result.ok).toBe(false);
    expect(result.message).toBe(UPLOAD_TOO_LARGE_MESSAGE);
  });
});
