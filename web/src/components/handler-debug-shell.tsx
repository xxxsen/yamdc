"use client";

import { Play } from "lucide-react";

import { ChainPicker } from "@/components/handler-debug-shell/chain-picker";
import { ResultPanel } from "@/components/handler-debug-shell/result-panel";
import { useHandlerDebugState } from "@/components/handler-debug-shell/use-handler-debug-state";
import { Button } from "@/components/ui/button";

// HandlerDebugShell: handler-debug 页面的组装层. 本体只负责:
// 1) 从 useHandlerDebugState 拉到所有业务状态 (参见 hook 里的生命
//    周期注释);
// 2) 渲染 hero (标题 + 运行按钮 + prefill/error 提示 + ChainPicker);
// 3) 渲染 ResultPanel.
//
// 原先此文件 532 行, 现在所有 useState / useEffect / useMemo /
// handler 都下沉到 hook, 子组件都拆到 handler-debug-shell/ 下的
// 独立文件 (chain-picker / chain-name / result-panel + 3 个 view).
export function HandlerDebugShell() {
  const {
    selectedChainHandlers,
    unselectedChainHandlers,
    metaJSON,
    setMetaJSON,
    addChainHandler,
    removeChainHandler,
    moveChainHandler,
    activeTab,
    setActiveTab,
    result,
    diffRows,
    picDiffState,
    isRunning,
    handleRun,
    error,
    prefillMessage,
  } = useHandlerDebugState();

  return (
    <div className="handler-debug-page">
      <section className="panel handler-debug-hero">
        <div className="handler-debug-copy">
          <span className="ruleset-debug-eyebrow">
            <Play size={14} />
            Handler 测试
          </span>
          <div className="handler-debug-title-row">
            <h2>Handler 链测试</h2>
            <Button
              variant="primary"
              className="ruleset-debug-run-button handler-debug-run-inline"
              onClick={handleRun}
              disabled={isRunning}
              loading={isRunning}
              leftIcon={<Play size={16} />}
            >
              <span>{isRunning ? "执行中..." : "运行"}</span>
            </Button>
          </div>
        </div>

        <div className="handler-debug-controls">
          <ChainPicker
            selectedChainHandlers={selectedChainHandlers}
            unselectedChainHandlers={unselectedChainHandlers}
            metaJSON={metaJSON}
            onAdd={addChainHandler}
            onRemove={removeChainHandler}
            onMove={moveChainHandler}
            onMetaJSONChange={setMetaJSON}
          />

          {prefillMessage ? <div className="handler-debug-message">{prefillMessage}</div> : null}
          {error ? <div className="ruleset-debug-error">{error}</div> : null}
        </div>
      </section>

      <ResultPanel
        activeTab={activeTab}
        onTabChange={setActiveTab}
        result={result}
        diffRows={diffRows}
        picDiffState={picDiffState}
      />
    </div>
  );
}
