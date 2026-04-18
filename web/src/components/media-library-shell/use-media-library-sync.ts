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

  // observedSyncRunningRef gates the "show 同步完成 flash and refetch
  // items" path so we only fire the completion UX if we actually
  // saw the run enter the running state. Without this, a fast
  // backend + slow frontend can make the polling loop jump straight
  // from "idle" to "idle" with the server-side run already done,
  // and we would show neither the spinner nor the flash.
  const prevSyncRunningRef = useRef(initialStatus?.sync.status === "running");
  const observedSyncRunningRef = useRef(initialStatus?.sync.status === "running");

  const refreshItems = useEffectEvent(async (nextParams?: {
    keyword?: string;
    year?: string;
    size?: string;
    sort?: string;
    order?: string;
  }) => {
    try {
      const next = await listMediaLibraryItems(nextParams);
      setItems(next);
      if (!nextParams?.year || nextParams.year === "all") {
        setYearOptions((current) => mergeYearOptions(current, extractYearOptions(next)));
      }
    } catch {
      // ignore polling errors
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
        setItems(nextItems);
      }
      prevSyncRunningRef.current = nextSyncRunning;
    } catch {
      // ignore polling errors
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
    handleTriggerSync,
  };
}
