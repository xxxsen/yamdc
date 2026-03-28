"use client";

import { Search } from "lucide-react";
import Link from "next/link";
import { useDeferredValue, useEffect, useEffectEvent, useRef, useState } from "react";

import type { MediaLibraryItem, MediaLibraryStatus } from "@/lib/api";
import { getMediaLibraryFileURL, getMediaLibraryStatus, listMediaLibraryItems } from "@/lib/api";

interface Props {
  items: MediaLibraryItem[];
  initialStatus: MediaLibraryStatus | null;
}

export function MediaLibraryShell({ items: initialItems, initialStatus }: Props) {
  const [items, setItems] = useState(initialItems);
  const [configured, setConfigured] = useState(Boolean(initialStatus?.configured));
  const [keyword, setKeyword] = useState("");
  const prevSyncRunningRef = useRef(initialStatus?.sync.status === "running");
  const deferredKeyword = useDeferredValue(keyword);

  const refreshItems = useEffectEvent(async () => {
    try {
      const next = await listMediaLibraryItems();
      setItems(next);
    } catch {
      // ignore polling errors
    }
  });

  const refreshStatus = useEffectEvent(async () => {
    try {
      const next = await getMediaLibraryStatus();
      setConfigured(Boolean(next.configured));
      const syncRunning = next.sync.status === "running";
      if (prevSyncRunningRef.current && !syncRunning) {
        const nextItems = await listMediaLibraryItems();
        setItems(nextItems);
      }
      prevSyncRunningRef.current = syncRunning;
    } catch {
      // ignore polling errors
    }
  });

  useEffect(() => {
    void refreshStatus();
    const timer = window.setInterval(() => {
      void refreshStatus();
    }, 3000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!configured) {
      return;
    }
    void refreshItems();
    const timer = window.setInterval(() => {
      void refreshItems();
    }, 8000);
    return () => window.clearInterval(timer);
  }, [configured]);

  const query = deferredKeyword.trim().toLowerCase();
  const filteredItems = !query
    ? items
    : items.filter((item) =>
      [
        item.title,
        item.number,
        item.rel_path,
        item.name,
      ]
        .join(" ")
        .toLowerCase()
        .includes(query));

  return (
    <div className="media-library-page">
      <section className="panel media-library-overview">
        {!configured ? (
          <div className="review-empty-state">当前还没有配置 `library_dir`，媒体库页面暂不可用。</div>
        ) : (
          <div className="media-library-list-shell">
            <label className="media-library-search-bar">
              <Search size={16} />
              <input
                className="media-library-search-input"
                placeholder="按标题 / 番号 / 路径搜索"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
              />
            </label>
            <div className="media-library-card-grid media-library-card-grid-only">
              {filteredItems.map((item) => {
                const posterPath = item.poster_path || item.cover_path;
                return (
                  <Link key={item.id} href={`/media-library/${item.id}`} className="media-library-card">
                    <div className="media-library-card-poster">
                      {posterPath ? (
                        <img src={getMediaLibraryFileURL(posterPath)} alt={item.title || item.name} className="media-library-card-image" />
                      ) : (
                        <div className="library-thumb-fallback">{(item.number || item.title || item.name).slice(0, 2).toUpperCase()}</div>
                      )}
                    </div>
                    <div className="media-library-card-copy">
                      <div className="media-library-card-title media-library-card-title-only">{item.title || item.name}</div>
                      <div className="library-item-number">{item.number || "未命名番号"}</div>
                    </div>
                  </Link>
                );
              })}
              {items.length === 0 ? <div className="review-empty-state">当前媒体库里还没有项目</div> : null}
              {items.length > 0 && filteredItems.length === 0 ? <div className="review-empty-state">没有匹配的媒体库项目</div> : null}
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
