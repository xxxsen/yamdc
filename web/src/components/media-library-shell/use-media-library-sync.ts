"use client";

import { type Dispatch, type SetStateAction, useEffect, useEffectEvent, useRef, useState } from "react";

import type { SizeFilter, SortMode, SortOrder } from "@/components/media-library-shell/filter-rail";
import {
  extractYearOptions,
  mergeYearOptions,
  toMediaLibrarySyncMessage,
} from "@/components/media-library-shell/utils";
import type { MediaLibraryItem, MediaLibraryStatus } from "@/lib/api";
import {
  getMediaLibraryStatus,
  listMediaLibraryItems,
  triggerMediaLibrarySync,
} from "@/lib/api";

export interface UseMediaLibrarySyncDeps {
  initialStatus: MediaLibraryStatus | null;
  deferredKeyword: string;
  yearFilter: string;
  sizeFilter: SizeFilter;
  sortMode: SortMode;
  sortOrder: SortOrder;
  setItems: Dispatch<SetStateAction<MediaLibraryItem[]>>;
  setYearOptions: Dispatch<SetStateAction<string[]>>;
}

export interface UseMediaLibrarySyncResult {
  configured: boolean;
  syncRunning: boolean;
  syncStarting: boolean;
  syncBusy: boolean;
  syncButtonLabel: string;
  syncMessage: string;
  syncCompletedFlash: boolean;
  // dataStale 在后台轮询失败时为 true (用户没主动操作, 不能弹 toast 打扰),
  // 任意一次成功的列表 / 状态刷新会清回 false. UI 用它显示"数据刷新失败,
  // 当前显示上一次结果"的非阻塞提示.
  dataStale: boolean;
  handleTriggerSync: () => void;
}

// useMediaLibrarySync owns the entire sync lifecycle for the media
// library page: initial status fetch, the 3s polling during a run,
// observing the running -> completed transition, refreshing the
// item list on filter changes, the transient "同步完成" flash, and
// the user-facing toast copy after the API call returns.
//
// Kept in the parent on purpose:
// * items / yearOptions state, because the detail modal's
//   onDetailChange also writes into items (so the source of truth
//   lives above the modal boundary).
// * filter state (keyword / yearFilter / etc.), because filter-rail
//   reads and writes it directly.
//
// The hook reads filter state to build listMediaLibraryItems
// parameters and writes items / yearOptions through the provided
// setters so the data stays at the parent level.
export function useMediaLibrarySync(deps: UseMediaLibrarySyncDeps): UseMediaLibrarySyncResult {
  const {
    initialStatus,
    deferredKeyword,
    yearFilter,
    sizeFilter,
    sortMode,
    sortOrder,
    setItems,
    setYearOptions,
  } = deps;

  const [configured, setConfigured] = useState(Boolean(initialStatus?.configured));
  const [syncRunning, setSyncRunning] = useState(initialStatus?.sync.status === "running");
  const [syncStarting, setSyncStarting] = useState(false);
  const [syncMessage, setSyncMessage] = useState("");
  const [syncCompletedFlash, setSyncCompletedFlash] = useState(false);
  const [dataStale, setDataStale] = useState(false);

  // observedSyncRunningRef gates the "show 同步完成 flash and refetch
  // items" path so we only fire the completion UX if we actually
  // saw the run enter the running state. Without this, a fast
  // backend + slow frontend can make the polling loop jump straight
  // from "idle" to "idle" with the server-side run already done,
  // and we would show neither the spinner nor the flash.
  const prevSyncRunningRef = useRef(initialStatus?.sync.status === "running");
  const observedSyncRunningRef = useRef(initialStatus?.sync.status === "running");

  // refreshItems 同时被两条路径调用:
  //   1) 后台轮询 (filter 变化、sync 完成) - 失败仅置 stale, 不打扰用户.
  //   2) 用户触发的"同步媒体库"按钮 - 失败由 handleTriggerSync 自己显
  //      示 syncMessage, 不复用这里的 stale 通道.
  // 任何成功路径都把 stale 清回 false, 因此偶发抖动会自愈.
  const refreshItems = useEffectEvent(async (nextParams?: {
    keyword?: string;
    year?: string;
    size?: string;
    sort?: string;
    order?: string;
  }) => {
    try {
      const next = await listMediaLibraryItems(nextParams);
      // 防御: 极端情况下 (mock 耗尽 / 后端契约破坏 / 网关拦截) 拿到的可能
      // 不是 MediaLibraryItem[] — 如果直接 setItems / extractYearOptions
      // 解构会抛 TypeError 把整个 React tree 弄崩. 视为后台刷新失败, 走
      // dataStale 通道, 保留上一次结果.
      if (!Array.isArray(next)) {
        setDataStale(true);
        return;
      }
      setItems(next);
      if (!nextParams?.year || nextParams.year === "all") {
        setYearOptions((current) => mergeYearOptions(current, extractYearOptions(next)));
      }
      setDataStale(false);
    } catch {
      // 后台轮询失败: 不抛 toast, 仅标记数据可能过期.
      setDataStale(true);
    }
  });

  const refreshStatus = useEffectEvent(async () => {
    try {
      const next = await getMediaLibraryStatus();
      setConfigured(next.configured);
      const nextSyncRunning = next.sync.status === "running";
      setSyncRunning(nextSyncRunning);
      if (nextSyncRunning) {
        setSyncStarting(false);
        observedSyncRunningRef.current = true;
      }
      // 本轮刷新是否要把 dataStale 置位. 不能在 if 分支里直接 setDataStale(true)
      // — try 块尾部有一条无条件的 setDataStale(false) (代表"本次状态拉取成
      // 功"的清理), 它会立刻覆盖分支里的 stale 信号; 也不能用 early return,
      // 因为后面的 prevSyncRunningRef.current = nextSyncRunning 是下一轮 polling
      // 边沿检测的必备 invariant, 一旦丢失再触发 running->idle 边沿就会失效.
      // 用本地变量推迟到尾部统一落盘.
      let stale = false;
      if (observedSyncRunningRef.current && prevSyncRunningRef.current && !nextSyncRunning) {
        setSyncCompletedFlash(true);
        observedSyncRunningRef.current = false;
        const nextItems = await listMediaLibraryItems({
          keyword: deferredKeyword,
          year: yearFilter,
          size: sizeFilter,
          sort: sortMode,
          order: sortOrder,
        });
        // 同步完成后的 items 拉取与 refreshItems 同源, 同样需要防御非数组
        // 返回 — 否则一次 mock 耗尽就会把 setItems 喂成 undefined, 让下游
        // grid 渲染时崩溃.
        if (Array.isArray(nextItems)) {
          setItems(nextItems);
        } else {
          stale = true;
        }
      }
      prevSyncRunningRef.current = nextSyncRunning;
      setDataStale(stale);
    } catch {
      // 后台 status 轮询失败也是非阻塞的, 同样只置 stale.
      setDataStale(true);
    }
  });

  useEffect(() => {
    if (!syncMessage) {
      return;
    }
    const timer = window.setTimeout(() => setSyncMessage(""), 2400);
    return () => window.clearTimeout(timer);
  }, [syncMessage]);

  useEffect(() => {
    if (!syncCompletedFlash) {
      return;
    }
    const timer = window.setTimeout(() => setSyncCompletedFlash(false), 1000);
    return () => window.clearTimeout(timer);
  }, [syncCompletedFlash]);

  useEffect(() => {
    void refreshStatus();
  }, []);

  useEffect(() => {
    if (!configured) {
      return;
    }
    const params = {
      keyword: deferredKeyword,
      year: yearFilter,
      size: sizeFilter,
      sort: sortMode,
      order: sortOrder,
    };
    void refreshItems(params);
  }, [configured, deferredKeyword, yearFilter, sizeFilter, sortMode, sortOrder]);

  const syncBusy = syncStarting || syncRunning;
  const syncButtonLabel = syncBusy ? "同步中..." : syncCompletedFlash ? "同步完成" : "同步媒体库";

  useEffect(() => {
    if (!syncBusy) {
      return;
    }
    const timer = window.setInterval(() => {
      void refreshStatus();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [syncBusy]);

  const handleTriggerSync = () => {
    setSyncCompletedFlash(false);
    setSyncMessage("媒体库同步已启动");
    setSyncStarting(true);
    void (async () => {
      try {
        await triggerMediaLibrarySync();
        setSyncRunning(true);
        setSyncStarting(false);
        observedSyncRunningRef.current = true;
        prevSyncRunningRef.current = true;
      } catch (error) {
        const message = toMediaLibrarySyncMessage(error);
        setSyncMessage(message);
        if (message === "媒体库正在同步中") {
          setSyncStarting(false);
          setSyncRunning(true);
          prevSyncRunningRef.current = true;
          observedSyncRunningRef.current = true;
          return;
        }
        setSyncStarting(false);
        setSyncRunning(false);
        prevSyncRunningRef.current = false;
      }
    })();
  };

  return {
    configured,
    syncRunning,
    syncStarting,
    syncBusy,
    syncButtonLabel,
    syncMessage,
    syncCompletedFlash,
    dataStale,
    handleTriggerSync,
  };
}
