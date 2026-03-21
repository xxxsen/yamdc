"use client";

import Image from "next/image";
import { useState, useTransition } from "react";

import type { JobItem, ReviewMeta, ScrapeDataItem } from "@/lib/api";
import { deleteJob, getAssetURL, getReviewJob, importReviewJob, saveReviewJob } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

interface Props {
  jobs: JobItem[];
  initialScrapeData: ScrapeDataItem | null;
}

function parseMeta(data: ScrapeDataItem | null): ReviewMeta | null {
  if (!data) {
    return null;
  }
  const raw = data.review_data || data.raw_data;
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as ReviewMeta;
  } catch {
    return null;
  }
}

function stringifyList(raw: string) {
  return raw
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean);
}

function listToText(items?: string[]) {
  return (items ?? []).join("\n");
}

function buildPayload(meta: ReviewMeta, actorsText: string, genresText: string) {
  return JSON.stringify(
    {
      ...meta,
      actors: stringifyList(actorsText),
      genres: stringifyList(genresText),
    },
    null,
    2,
  );
}

export function ReviewShell({ jobs, initialScrapeData }: Props) {
  const initialMeta = parseMeta(initialScrapeData);
  const [items, setItems] = useState<JobItem[]>(jobs);
  const [selected, setSelected] = useState<JobItem | null>(jobs[0] ?? null);
  const [meta, setMeta] = useState<ReviewMeta | null>(initialMeta);
  const [actorsText, setActorsText] = useState(listToText(initialMeta?.actors));
  const [genresText, setGenresText] = useState(listToText(initialMeta?.genres));
  const [rawJSON, setRawJSON] = useState(initialScrapeData?.review_data || initialScrapeData?.raw_data || "");
  const [message, setMessage] = useState<string>(jobs.length === 0 ? "当前没有待 review 的任务" : "");
  const [isPending, startTransition] = useTransition();

  const loadDetail = (job: JobItem) => {
    setSelected(job);
    startTransition(async () => {
      try {
        setMessage("加载刮削结果...");
        const data = await getReviewJob(job.id);
        const nextMeta = parseMeta(data);
        setMeta(nextMeta);
        setActorsText(listToText(nextMeta?.actors));
        setGenresText(listToText(nextMeta?.genres));
        setRawJSON(data?.review_data || data?.raw_data || "");
        setMessage(data ? "" : "该任务还没有 scrape_data");
      } catch (error) {
        setMeta(null);
        setActorsText("");
        setGenresText("");
        setRawJSON("");
        setMessage(error instanceof Error ? error.message : "加载失败");
      }
    });
  };

  const removeJobFromList = (jobID: number) => {
    const next = items.filter((item) => item.id !== jobID);
    setItems(next);
    const nextSelected = next[0] ?? null;
    if (!nextSelected) {
      setSelected(null);
      setMeta(null);
      setActorsText("");
      setGenresText("");
      setRawJSON("");
      return;
    }
    loadDetail(nextSelected);
  };

  const updateMeta = (patch: Partial<ReviewMeta>) => {
    const nextMeta = { ...(meta ?? {}), ...patch };
    setMeta(nextMeta);
    setRawJSON(buildPayload(nextMeta, actorsText, genresText));
  };

  const updateActors = (value: string) => {
    setActorsText(value);
    if (!meta) {
      return;
    }
    setRawJSON(buildPayload(meta, value, genresText));
  };

  const updateGenres = (value: string) => {
    setGenresText(value);
    if (!meta) {
      return;
    }
    setRawJSON(buildPayload(meta, actorsText, value));
  };

  const handleSave = () => {
    if (!selected || !meta) {
      return;
    }
    const payload = buildPayload(meta, actorsText, genresText);
    startTransition(async () => {
      try {
        setMessage("保存 review 数据...");
        await saveReviewJob(selected.id, payload);
        const data = await getReviewJob(selected.id);
        const nextMeta = parseMeta(data);
        setMeta(nextMeta);
        setActorsText(listToText(nextMeta?.actors));
        setGenresText(listToText(nextMeta?.genres));
        setRawJSON(data?.review_data || data?.raw_data || "");
        setMessage("review 数据已保存");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "保存失败");
      }
    });
  };

  const handleImport = () => {
    if (!selected || !meta) {
      return;
    }
    const payload = buildPayload(meta, actorsText, genresText);
    startTransition(async () => {
      try {
        setMessage("执行入库...");
        await saveReviewJob(selected.id, payload);
        await importReviewJob(selected.id);
        removeJobFromList(selected.id);
        setMessage("入库完成，任务已移出 review 列表");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "入库失败");
      }
    });
  };

  const handleDelete = () => {
    if (!selected) {
      return;
    }
    const ok = window.confirm(`确认删除该任务及源文件吗？\n\n${selected.rel_path}`);
    if (!ok) {
      return;
    }
    startTransition(async () => {
      try {
        setMessage("删除任务...");
        await deleteJob(selected.id);
        removeJobFromList(selected.id);
        setMessage("任务已删除");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "删除失败");
      }
    });
  };

  return (
    <div className="two-col">
      <aside className="panel" style={{ padding: 18, height: "100%", overflow: "hidden", display: "flex", flexDirection: "column", minHeight: 0 }}>
        <h2 style={{ marginTop: 0 }}>待 Review 列表</h2>
        <div style={{ display: "grid", gap: 10, overflow: "auto", minHeight: 0, paddingRight: 4 }}>
          {items.length === 0 ? <div style={{ color: "var(--muted)" }}>当前没有待 review 的任务</div> : null}
          {items.map((job) => (
            <button
              key={job.id}
              className="panel"
              style={{
                padding: 12,
                borderRadius: 14,
                textAlign: "left",
                cursor: "pointer",
                border: selected?.id === job.id ? "1px solid var(--accent)" : undefined,
                background: selected?.id === job.id ? "rgba(180, 79, 45, 0.08)" : undefined,
              }}
              onClick={() => loadDetail(job)}
              disabled={isPending}
            >
              <div style={{ fontWeight: 600 }}>{job.rel_path}</div>
              <div style={{ marginTop: 4, color: "var(--muted)", fontSize: 14 }}>{job.number}</div>
              <div style={{ marginTop: 4, color: "var(--muted)", fontSize: 13 }}>
                更新时间 {formatUnixMillis(job.updated_at)}
              </div>
            </button>
          ))}
        </div>
      </aside>
      <section className="panel" style={{ padding: 18, height: "100%", overflow: "hidden", display: "flex", flexDirection: "column", minHeight: 0 }}>
        <h2 style={{ marginTop: 0 }}>刮削内容</h2>
        {selected ? (
          <div style={{ marginBottom: 12, color: "var(--muted)" }}>
            当前任务 #{selected.id} / {selected.rel_path}
          </div>
        ) : null}
        {message ? <p style={{ color: "var(--muted)", marginBottom: 16 }}>{message}</p> : null}
        <div style={{ display: "flex", gap: 10, marginBottom: 12 }}>
          <button className="btn" onClick={handleSave} disabled={!selected || isPending || !meta}>
            保存修改
          </button>
          <button className="btn btn-primary" onClick={handleImport} disabled={!selected || isPending || !meta}>
            入库
          </button>
          <button className="btn" onClick={handleDelete} disabled={!selected || isPending}>
            删除
          </button>
        </div>
        <div style={{ overflow: "auto", minHeight: 0, paddingRight: 4 }}>
          {meta ? (
            <div style={{ display: "grid", gap: 14 }}>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>番号</div>
                <input className="input" value={meta.number ?? ""} onChange={(e) => updateMeta({ number: e.target.value })} />
              </label>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>导演</div>
                <input className="input" value={meta.director ?? ""} onChange={(e) => updateMeta({ director: e.target.value })} />
              </label>
            </div>
            <label>
              <div style={{ marginBottom: 6, color: "var(--muted)" }}>标题</div>
              <input className="input" value={meta.title ?? ""} onChange={(e) => updateMeta({ title: e.target.value })} />
            </label>
            <label>
              <div style={{ marginBottom: 6, color: "var(--muted)" }}>翻译标题</div>
              <input className="input" value={meta.title_translated ?? ""} onChange={(e) => updateMeta({ title_translated: e.target.value })} />
            </label>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12 }}>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>制作商</div>
                <input className="input" value={meta.studio ?? ""} onChange={(e) => updateMeta({ studio: e.target.value })} />
              </label>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>发行商</div>
                <input className="input" value={meta.label ?? ""} onChange={(e) => updateMeta({ label: e.target.value })} />
              </label>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>系列</div>
                <input className="input" value={meta.series ?? ""} onChange={(e) => updateMeta({ series: e.target.value })} />
              </label>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>简介</div>
                <textarea className="input" style={{ minHeight: 120 }} value={meta.plot ?? ""} onChange={(e) => updateMeta({ plot: e.target.value })} />
              </label>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>翻译简介</div>
                <textarea
                  className="input"
                  style={{ minHeight: 120 }}
                  value={meta.plot_translated ?? ""}
                  onChange={(e) => updateMeta({ plot_translated: e.target.value })}
                />
              </label>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>演员列表，每行一项</div>
                <textarea className="input" style={{ minHeight: 120 }} value={actorsText} onChange={(e) => updateActors(e.target.value)} />
              </label>
              <label>
                <div style={{ marginBottom: 6, color: "var(--muted)" }}>标签列表，每行一项</div>
                <textarea className="input" style={{ minHeight: 120 }} value={genresText} onChange={(e) => updateGenres(e.target.value)} />
              </label>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 12 }}>
              {meta.cover ? (
                <div className="panel" style={{ padding: 12 }}>
                  <div style={{ marginBottom: 8, fontWeight: 600 }}>封面</div>
                  <div style={{ position: "relative", width: "100%", aspectRatio: "2 / 3", overflow: "hidden", borderRadius: 12 }}>
                    <Image src={getAssetURL(meta.cover.key)} alt="cover" fill style={{ objectFit: "cover" }} unoptimized />
                  </div>
                </div>
              ) : null}
              {meta.poster ? (
                <div className="panel" style={{ padding: 12 }}>
                  <div style={{ marginBottom: 8, fontWeight: 600 }}>海报</div>
                  <div style={{ position: "relative", width: "100%", aspectRatio: "2 / 3", overflow: "hidden", borderRadius: 12 }}>
                    <Image src={getAssetURL(meta.poster.key)} alt="poster" fill style={{ objectFit: "cover" }} unoptimized />
                  </div>
                </div>
              ) : null}
            </div>
            {meta.sample_images && meta.sample_images.length > 0 ? (
              <div className="panel" style={{ padding: 12 }}>
                <div style={{ marginBottom: 8, fontWeight: 600 }}>样品图</div>
                <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))", gap: 12 }}>
                  {meta.sample_images.map((item) => (
                    <div key={item.key} style={{ position: "relative", width: "100%", aspectRatio: "16 / 9", overflow: "hidden", borderRadius: 10 }}>
                      <Image src={getAssetURL(item.key)} alt={item.name} fill style={{ objectFit: "cover" }} unoptimized />
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
            <details className="panel" style={{ padding: 12 }}>
              <summary style={{ cursor: "pointer", fontWeight: 600 }}>JSON 预览</summary>
              <pre style={{ margin: "12px 0 0", whiteSpace: "pre-wrap", wordBreak: "break-word", fontSize: 13 }}>{rawJSON}</pre>
            </details>
            </div>
          ) : (
            <div style={{ color: "var(--muted)" }}>选择左侧任务后在这里展示刮削结果</div>
          )}
        </div>
      </section>
    </div>
  );
}
