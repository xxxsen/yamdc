"use client";

import { handleEditorTextareaKeyDown, jsonKeyCount } from "./plugin-editor-utils";
import type { RequestFormState } from "./plugin-editor-types";

export function RequestForm(props: {
  state: RequestFormState;
  onChange: (updater: (prev: RequestFormState) => RequestFormState) => void;
  expandAdvanced?: boolean;
  compactJSONBlocks?: boolean;
  nextRequestLayout?: boolean;
  fetchType?: string;
}) {
  const { state } = props;
  const targetMode = state.rawURL ? "url" : "path";
  const targetValue = targetMode === "url" ? state.rawURL : state.path;

  function handleTargetModeChange(mode: "path" | "url") {
    const current = targetValue;
    if (mode === "url") {
      props.onChange((prev) => ({ ...prev, path: "", rawURL: current }));
      return;
    }
    props.onChange((prev) => ({ ...prev, path: current, rawURL: "" }));
  }

  function handleTargetValueChange(value: string) {
    if (targetMode === "url") {
      props.onChange((prev) => ({ ...prev, rawURL: value }));
      return;
    }
    props.onChange((prev) => ({ ...prev, path: value }));
  }

  function patchField<K extends keyof RequestFormState>(key: K, value: RequestFormState[K]) {
    props.onChange((prev) => ({ ...prev, [key]: value }));
  }

  const isBrowserMode = props.fetchType === "browser";

  const browserRenderingBlock = isBrowserMode ? (
    <details className="plugin-editor-request-json-detail plugin-editor-browser-detail" open>
      <summary>
        <span>Browser Rendering</span>
      </summary>
      <div className="plugin-editor-request-inline-row plugin-editor-advanced-grid plugin-editor-browser-fields">
        <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
          <span>Wait XPath</span>
          <input className="input" value={state.browserWaitSelector} onChange={(event) => patchField("browserWaitSelector", event.target.value)} placeholder='例如 //div[@class="result-list"]' />
        </label>
        <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
          <span>Wait Timeout (s)</span>
          <input className="input" type="number" value={state.browserWaitTimeout} onChange={(event) => patchField("browserWaitTimeout", event.target.value)} placeholder="默认 60" />
        </label>
      </div>
    </details>
  ) : null;

  return (
    <>
      {props.nextRequestLayout ? (
        <>
          <div className="plugin-editor-request-inline-row">
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-method plugin-editor-request-inline-field-next-top">
              <span>Method</span>
              <select className="input" value={state.method} onChange={(event) => patchField("method", event.target.value)}>
                <option value="GET">GET</option>
                <option value="POST">POST</option>
              </select>
            </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xs plugin-editor-request-inline-field-next-top">
            <span>Target Type</span>
            <select className="input" value={targetMode} onChange={(event) => handleTargetModeChange(event.target.value as "path" | "url")}>
              <option value="path">path</option>
              <option value="url">url</option>
            </select>
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-lg plugin-editor-request-inline-field-next-top">
            <span>{targetMode === "url" ? "URL" : "Path"}</span>
            <input
              className="input"
              value={targetValue}
              onChange={(event) => handleTargetValueChange(event.target.value)}
              placeholder={targetMode === "url" ? "例如 https://example.com/${number}" : "例如 /search/${number}"}
            />
          </label>
        </div>
          <div className="plugin-editor-request-inline-row plugin-editor-request-inline-row-next-meta">
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-accept">
              <span>Accept Status</span>
              <input className="input" value={state.acceptStatusText} onChange={(event) => patchField("acceptStatusText", event.target.value)} placeholder="200,302" />
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-next-not-found">
              <span title="Not Found Status">Not Found Status</span>
              <input className="input" value={state.notFoundStatusText} onChange={(event) => patchField("notFoundStatusText", event.target.value)} placeholder="404" />
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-content-type">
              <span>Content-Type</span>
              <select className="input" value={state.bodyKind} onChange={(event) => patchField("bodyKind", event.target.value)}>
                <option value="json">json</option>
                <option value="form">form</option>
                <option value="raw">raw</option>
              </select>
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-next-charset">
              <span title="Decode Charset">Decode Charset</span>
              <input className="input" value={state.decodeCharset} onChange={(event) => patchField("decodeCharset", event.target.value)} placeholder="例如 euc-jp" />
            </label>
          </div>
        </>
      ) : (
        <div className="plugin-editor-request-inline-row">
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-method">
            <span>Method</span>
            <select className="input" value={state.method} onChange={(event) => patchField("method", event.target.value)}>
              <option value="GET">GET</option>
              <option value="POST">POST</option>
            </select>
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xs">
            <span>Target Type</span>
            <select className="input" value={targetMode} onChange={(event) => handleTargetModeChange(event.target.value as "path" | "url")}>
              <option value="path">path</option>
              <option value="url">url</option>
            </select>
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-lg">
            <span>{targetMode === "url" ? "URL" : "Path"}</span>
            <input
              className="input"
              value={targetValue}
              onChange={(event) => handleTargetValueChange(event.target.value)}
              placeholder={targetMode === "url" ? "例如 https://example.com/${number}" : "例如 /search/${number}"}
            />
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-accept">
            <span>Accept Status</span>
            <input className="input" value={state.acceptStatusText} onChange={(event) => patchField("acceptStatusText", event.target.value)} placeholder="200,302" />
          </label>
          <label className="plugin-editor-field-inline plugin-editor-request-inline-field-content-type">
            <span>Content-Type</span>
            <select className="input" value={state.bodyKind} onChange={(event) => patchField("bodyKind", event.target.value)}>
              <option value="json">json</option>
              <option value="form">form</option>
              <option value="raw">raw</option>
            </select>
          </label>
        </div>
      )}
      {props.compactJSONBlocks ? (
        <div className="plugin-editor-request-json-stack">
          <details className="plugin-editor-request-json-detail">
            <summary>
              <span>Header JSON</span>
              <span className={`plugin-editor-request-json-count ${jsonKeyCount(state.headersJSON) > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>
                {jsonKeyCount(state.headersJSON) > 0 ? jsonKeyCount(state.headersJSON) : 0}
              </span>
            </summary>
            <textarea className="input plugin-editor-textarea" value={state.headersJSON} onChange={(event) => patchField("headersJSON", event.target.value)} onKeyDown={handleEditorTextareaKeyDown} />
          </details>
          <details className="plugin-editor-request-json-detail">
            <summary>
              <span>Cookie JSON</span>
              <span className={`plugin-editor-request-json-count ${jsonKeyCount(state.cookiesJSON) > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>
                {jsonKeyCount(state.cookiesJSON) > 0 ? jsonKeyCount(state.cookiesJSON) : 0}
              </span>
            </summary>
            <textarea className="input plugin-editor-textarea" value={state.cookiesJSON} onChange={(event) => patchField("cookiesJSON", event.target.value)} onKeyDown={handleEditorTextareaKeyDown} />
          </details>
          <details className="plugin-editor-request-json-detail">
            <summary>
              <span>Query JSON</span>
              <span className={`plugin-editor-request-json-count ${jsonKeyCount(state.queryJSON) > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>
                {jsonKeyCount(state.queryJSON) > 0 ? jsonKeyCount(state.queryJSON) : 0}
              </span>
            </summary>
            <textarea className="input plugin-editor-textarea" value={state.queryJSON} onChange={(event) => patchField("queryJSON", event.target.value)} onKeyDown={handleEditorTextareaKeyDown} />
          </details>
          <details className="plugin-editor-request-json-detail">
            <summary>
              <span>Body</span>
              <span className={`plugin-editor-request-json-count ${jsonKeyCount(state.bodyJSON) > 0 ? "" : "plugin-editor-request-json-count-hidden"}`}>
                {jsonKeyCount(state.bodyJSON) > 0 ? jsonKeyCount(state.bodyJSON) : 0}
              </span>
            </summary>
            <textarea className="input plugin-editor-textarea" value={state.bodyJSON} onChange={(event) => patchField("bodyJSON", event.target.value)} onKeyDown={handleEditorTextareaKeyDown} />
          </details>
          {props.nextRequestLayout && browserRenderingBlock}
        </div>
      ) : (
        <div className="plugin-editor-json-grid">
          <label className="plugin-editor-field">
            <span>Query JSON</span>
            <textarea className="input plugin-editor-textarea" value={state.queryJSON} onChange={(event) => patchField("queryJSON", event.target.value)} onKeyDown={handleEditorTextareaKeyDown} />
          </label>
          <label className="plugin-editor-field">
            <span>Headers JSON</span>
            <textarea className="input plugin-editor-textarea" value={state.headersJSON} onChange={(event) => patchField("headersJSON", event.target.value)} onKeyDown={handleEditorTextareaKeyDown} />
          </label>
          <label className="plugin-editor-field">
            <span>Body</span>
            <textarea className="input plugin-editor-textarea" value={state.bodyJSON} onChange={(event) => patchField("bodyJSON", event.target.value)} onKeyDown={handleEditorTextareaKeyDown} />
          </label>
        </div>
      )}
      {props.expandAdvanced && !props.nextRequestLayout ? (
        <div className="plugin-editor-request-advanced-open">
          <div className="plugin-editor-request-inline-row plugin-editor-advanced-grid">
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
              <span>Not Found Status</span>
              <input className="input" value={state.notFoundStatusText} onChange={(event) => patchField("notFoundStatusText", event.target.value)} placeholder="404" />
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
              <span>Decode Charset</span>
              <input className="input" value={state.decodeCharset} onChange={(event) => patchField("decodeCharset", event.target.value)} placeholder="例如 euc-jp" />
            </label>
          </div>
          {browserRenderingBlock}
        </div>
      ) : !props.expandAdvanced ? (
        <details className="plugin-editor-advanced">
          <summary>高级选项</summary>
          <div className="plugin-editor-request-inline-row plugin-editor-advanced-grid">
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
              <span>Not Found Status</span>
              <input className="input" value={state.notFoundStatusText} onChange={(event) => patchField("notFoundStatusText", event.target.value)} placeholder="404" />
            </label>
            <label className="plugin-editor-field-inline plugin-editor-request-inline-field-xl">
              <span>Decode Charset</span>
              <input className="input" value={state.decodeCharset} onChange={(event) => patchField("decodeCharset", event.target.value)} placeholder="例如 euc-jp" />
            </label>
          </div>
          <div className="plugin-editor-form-grid plugin-editor-advanced-grid">
            <label className="plugin-editor-field plugin-editor-field-wide">
              <span>Cookies JSON</span>
              <textarea className="input plugin-editor-textarea" value={state.cookiesJSON} onChange={(event) => patchField("cookiesJSON", event.target.value)} onKeyDown={handleEditorTextareaKeyDown} />
            </label>
          </div>
          {browserRenderingBlock}
        </details>
      ) : null}
      {/* browser block for nextRequestLayout is inside plugin-editor-request-json-stack above */}
    </>
  );
}
