// Upload size guard shared between Library / Review / MediaLibrary 上传入口.
//
// 后端 (internal/web/jobs_routes.go: maxUploadImageBytes) 用 32 MiB 作为
// 硬上限并返回 HTTP 413; 前端在拿到 File 对象后立即检查 size, 让用户在
// 选完图片就看到 "图片不能超过 32 MiB" 而不是等一次往返失败.
//
// 单点导出常量 + 校验函数, 避免每个 hook 各写一遍数字, 数字漂了不一致.

export const MAX_UPLOAD_IMAGE_BYTES = 32 * 1024 * 1024;

export const UPLOAD_TOO_LARGE_MESSAGE = "图片不能超过 32 MiB";

export interface UploadSizeCheck {
  ok: boolean;
  message?: string;
}

// validateUploadSize 同步返回上传是否合法 + 用户文案. ok=false 时调用方
// 应直接 setMessage(message), 不应再调用上传 API.
export function validateUploadSize(file: File): UploadSizeCheck {
  if (file.size > MAX_UPLOAD_IMAGE_BYTES) {
    return { ok: false, message: UPLOAD_TOO_LARGE_MESSAGE };
  }
  return { ok: true };
}
