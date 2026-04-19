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
          {logs.map((item) => (
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
