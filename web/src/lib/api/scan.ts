import { apiRequest } from "./core";

export async function triggerScan(signal?: AbortSignal) {
  return await apiRequest<unknown>("/api/scan", { method: "POST", signal });
}
