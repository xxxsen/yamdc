"use client";

import { useState } from "react";

import { ChainName } from "@/components/handler-debug-shell/chain-name";
import type { HandlerDebugInstance } from "@/lib/api";

export interface ChainPickerProps {
  selectedChainHandlers: HandlerDebugInstance[];
  unselectedChainHandlers: HandlerDebugInstance[];
  metaJSON: string;
  onAdd: (handlerID: string) => void;
  onRemove: (handlerID: string) => void;
  onMove: (sourceID: string, targetID: string) => void;
  onMetaJSONChange: (next: string) => void;
}

// ChainPicker: handler-debug 主控制台 - 左侧 "已选 / 未选" 两列 + 右侧
// Meta JSON 编辑器. Pure UI, 所有业务语义经 props 注入.
//
// 关于 draggingHandlerID 局部化: 拖拽状态从原组件拆下来留在内部, 因为
// 它的读写都只发生在 "已选 handler" 这一列的 drag 事件里 -- 父组件不
// 需要感知哪个卡片正在被拖. 拆出去反而会把 drag 事件委托出去, 制造
// 不必要的耦合. 注意 onDrop 必须 preventDefault 才能触发 drop 回调
// (浏览器默认行为会吞掉).
export function ChainPicker({
  selectedChainHandlers,
  unselectedChainHandlers,
  metaJSON,
  onAdd,
  onRemove,
  onMove,
  onMetaJSONChange,
}: ChainPickerProps) {
  const [draggingHandlerID, setDraggingHandlerID] = useState<string | null>(null);

  return (
    <div className="handler-debug-chain-top">
      <div className="handler-debug-chain-workspace">
        <div className="handler-debug-chain-column">
          <div className="handler-debug-chain-head">
            <strong>已选 Handler</strong>
            <span className="handler-debug-chain-count">{selectedChainHandlers.length}</span>
          </div>
          <div className="handler-debug-chain-list">
            {selectedChainHandlers.map((item) => (
              <button
                key={item.id}
                type="button"
                className="handler-debug-chain-card handler-debug-chain-card-selected"
                onClick={() => onRemove(item.id)}
                draggable
                onDragStart={() => setDraggingHandlerID(item.id)}
                onDragEnd={() => setDraggingHandlerID(null)}
                onDragOver={(event) => event.preventDefault()}
                onDrop={(event) => {
                  event.preventDefault();
                  if (draggingHandlerID) {
                    onMove(draggingHandlerID, item.id);
                  }
                  setDraggingHandlerID(null);
                }}
              >
                <span className="handler-debug-chain-grip">::</span>
                <ChainName name={item.name} />
              </button>
            ))}
            {selectedChainHandlers.length === 0 ? (
              <div className="ruleset-debug-empty">点击右侧未选中的 handler 加入链路。</div>
            ) : null}
          </div>
        </div>
        <div className="handler-debug-chain-column">
          <div className="handler-debug-chain-head">
            <strong>未选 Handler</strong>
            <span className="handler-debug-chain-count">{unselectedChainHandlers.length}</span>
          </div>
          <div className="handler-debug-chain-list">
            {unselectedChainHandlers.map((item) => (
              <button
                key={item.id}
                type="button"
                className="handler-debug-chain-card"
                onClick={() => onAdd(item.id)}
              >
                <ChainName name={item.name} />
              </button>
            ))}
            {unselectedChainHandlers.length === 0 ? (
              <div className="ruleset-debug-empty">当前全部 handler 都已加入链路。</div>
            ) : null}
          </div>
        </div>
      </div>
      <div className="handler-debug-chain-meta">
        <div className="handler-debug-chain-head">
          <strong>Meta JSON</strong>
        </div>
        <textarea
          className="input handler-debug-textarea handler-debug-textarea-compact"
          value={metaJSON}
          onChange={(event) => onMetaJSONChange(event.target.value)}
        />
      </div>
    </div>
  );
}
