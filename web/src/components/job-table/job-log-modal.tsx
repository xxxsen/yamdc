import { useMemo } from "react";

import { X } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { JobItem, JobLogItem } from "@/lib/api";
import { formatUnixMillis } from "@/lib/utils";

interface Props {
  job: JobItem;
  logs: JobLogItem[];
  message: string;
  onClose: () => void;
}

export function JobLogModal({ job, logs, message, onClose }: Props) {
  // 按产生时间逆序: 最新的日志排在最上面, 打开弹窗就能看到最近发生了什么,
  // 不需要滚到底。id 作为 tiebreaker 保证相同时间戳的顺序稳定 (创建自增)。
  const sortedLogs = useMemo(() => {
    return [...logs].sort((a, b) => {
      if (b.created_at !== a.created_at) {
        return b.created_at - a.created_at;
      }
      return b.id - a.id;
    });
  }, [logs]);

  return (
    <div className="review-preview-overlay" onClick={onClose}>
      <div className="panel file-log-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="file-log-head">
          <div>
            <div className="file-log-kicker">Task Trace</div>
            <h3 className="file-log-title">任务日志 #{job.id}</h3>
            <div className="file-log-path">{job.rel_path}</div>
          </div>
          <Button onClick={onClose}>
            <X size={16} />
          </Button>
        </div>
        {message ? <div className="file-log-message">{message}</div> : null}
        <div className="file-log-list">
          {sortedLogs.map((item) => (
            <div key={item.id} className="file-log-item">
              <div className="file-log-meta">
                <span>{formatUnixMillis(item.created_at)}</span>
                <span className="file-log-pill">{item.level}</span>
                <span className="file-log-pill">{item.stage}</span>
              </div>
              <div className="file-log-text">{item.message}</div>
              {item.detail ? <div className="file-log-detail">{item.detail}</div> : null}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
