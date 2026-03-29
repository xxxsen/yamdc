"use client";

import { Bug, LoaderCircle, Play } from "lucide-react";
import { useState, useTransition } from "react";

import { explainNumberCleaner, type NumberCleanerExplainResult } from "@/lib/api";

const DEFAULT_INPUT = "FC2PPV12345 中文字幕.mp4";

export function RulesetDebugShell() {
  const [input, setInput] = useState(DEFAULT_INPUT);
  const [result, setResult] = useState<NumberCleanerExplainResult | null>(null);
  const [error, setError] = useState("");
  const [isPending, startTransition] = useTransition();
  const steps = result?.steps ?? [];

  const handleRun = () => {
    const nextInput = input.trim();
    if (!nextInput) {
      setError("请输入待测试的番号或文件名。");
      setResult(null);
      return;
    }
    startTransition(() => {
      void (async () => {
        try {
          const next = await explainNumberCleaner(nextInput);
          setResult(next);
          setError("");
        } catch (nextError) {
          setResult(null);
          setError(nextError instanceof Error ? nextError.message : "规则集测试失败");
        }
      })();
    });
  };

  return (
    <div className="ruleset-debug-page">
      <section className="panel ruleset-debug-hero">
        <div className="ruleset-debug-hero-copy">
          <span className="ruleset-debug-eyebrow">
            <Bug size={14} />
            调试工具
          </span>
          <h2>规则集测试</h2>
          <p>输入一个文件名或番号，直接查看 `numbercleaner` 每一步规则的入参、出参和最终结果。</p>
        </div>
        <div className="ruleset-debug-runner">
          <label className="ruleset-debug-label" htmlFor="ruleset-debug-input">
            待测试内容
          </label>
          <div className="ruleset-debug-input-row">
            <input
              id="ruleset-debug-input"
              className="input ruleset-debug-input"
              value={input}
              onChange={(event) => setInput(event.target.value)}
              placeholder="例如：FC2PPV12345 中文字幕.mp4"
            />
            <button className="btn btn-primary ruleset-debug-run-button" type="button" onClick={handleRun} disabled={isPending}>
              {isPending ? <LoaderCircle size={16} className="ruleset-debug-spinner" /> : <Play size={16} />}
              <span>{isPending ? "解析中..." : "开始测试"}</span>
            </button>
          </div>
          {error ? <div className="ruleset-debug-error">{error}</div> : null}
        </div>
      </section>

      <div className="ruleset-debug-grid">
        <section className="panel ruleset-debug-summary-panel">
          <div className="ruleset-debug-panel-head">
            <h3>最终结果</h3>
            {result?.final ? <span className={`ruleset-debug-status ruleset-debug-status-${result.final.status}`}>{result.final.status}</span> : null}
          </div>
          {result?.final ? (
            <div className="ruleset-debug-summary">
              <div className="ruleset-debug-summary-row">
                <span>原始输入</span>
                <strong>{result.input}</strong>
              </div>
              <div className="ruleset-debug-summary-row">
                <span>去扩展名后</span>
                <strong>{result.input_no_ext || "-"}</strong>
              </div>
              <div className="ruleset-debug-summary-row">
                <span>标准化结果</span>
                <strong>{result.final.normalized || "-"}</strong>
              </div>
              <div className="ruleset-debug-summary-row">
                <span>主番号</span>
                <strong>{result.final.number_id || "-"}</strong>
              </div>
              <div className="ruleset-debug-summary-row">
                <span>置信度</span>
                <strong>{result.final.confidence || "-"}</strong>
              </div>
              <div className="ruleset-debug-summary-row">
                <span>分类</span>
                <strong>{result.final.category || "-"}</strong>
              </div>
              <div className="ruleset-debug-summary-row">
                <span>无码判断</span>
                <strong>{result.final.uncensor ? "true" : "false"}</strong>
              </div>
              <div className="ruleset-debug-summary-row">
                <span>命中规则</span>
                <strong>{result.final.rule_hits.length ? result.final.rule_hits.join(", ") : "-"}</strong>
              </div>
              <div className="ruleset-debug-summary-row">
                <span>警告</span>
                <strong>{result.final.warnings.length ? result.final.warnings.join(", ") : "-"}</strong>
              </div>
            </div>
          ) : (
            <div className="ruleset-debug-empty">运行一次规则测试后，这里会显示最终番号、状态和命中规则。</div>
          )}
        </section>

        <section className="panel ruleset-debug-steps-panel">
          <div className="ruleset-debug-panel-head">
            <h3>执行链路</h3>
            <span>{result ? `${steps.length} steps` : "等待运行"}</span>
          </div>
          {result ? (
            <div className="ruleset-debug-step-list">
              {steps.map((step, index) => (
                <article key={`${step.stage}-${step.rule}-${index}`} className={`ruleset-debug-step ${step.selected ? "ruleset-debug-step-selected" : ""}`}>
                  <div className="ruleset-debug-step-head">
                    <div>
                      <span className="ruleset-debug-step-index">{String(index + 1).padStart(2, "0")}</span>
                      <span className="ruleset-debug-step-stage">{step.stage}</span>
                      <strong>{step.rule}</strong>
                    </div>
                    <span className={`ruleset-debug-step-badge ${step.matched ? "ruleset-debug-step-badge-hit" : ""}`}>
                      {step.selected ? "selected" : step.matched ? "matched" : "skip"}
                    </span>
                  </div>
                  <div className="ruleset-debug-step-body">
                    <div>
                      <span>输入</span>
                      <code>{step.input || "-"}</code>
                    </div>
                    <div>
                      <span>输出</span>
                      <code>{step.output || "-"}</code>
                    </div>
                  </div>
                  {step.summary ? <p className="ruleset-debug-step-summary">{step.summary}</p> : null}
                  {step.values?.length ? <p className="ruleset-debug-step-values">values: {step.values.join(", ")}</p> : null}
                  {step.candidate ? (
                    <div className="ruleset-debug-step-candidate">
                      <span>candidate</span>
                      <code>
                        {step.candidate.number_id} / score={step.candidate.score} / matcher={step.candidate.matcher}
                      </code>
                    </div>
                  ) : null}
                </article>
              ))}
            </div>
          ) : (
            <div className="ruleset-debug-empty">运行后会按顺序展示 normalizers、rewrite、suffix、noise、matcher、post 和最终选择结果。</div>
          )}
        </section>
      </div>
    </div>
  );
}
