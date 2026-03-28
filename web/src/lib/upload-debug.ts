type UploadDebugEntry = {
  ts: string;
  scope: string;
  event: string;
  data?: unknown;
};

type DebugWindow = Window & {
  __yamdcUploadDebug?: UploadDebugEntry[];
};

function getDebugStore() {
  if (typeof window === "undefined") {
    return null;
  }
  const debugWindow = window as DebugWindow;
  if (!debugWindow.__yamdcUploadDebug) {
    debugWindow.__yamdcUploadDebug = [];
  }
  return debugWindow.__yamdcUploadDebug;
}

export function describeActiveElement() {
  if (typeof document === "undefined") {
    return "document-unavailable";
  }
  const active = document.activeElement;
  if (!active) {
    return "none";
  }
  const tag = active.tagName.toLowerCase();
  const id = active.id ? `#${active.id}` : "";
  const className = typeof active.className === "string" && active.className.trim()
    ? `.${active.className.trim().split(/\s+/).join(".")}`
    : "";
  return `${tag}${id}${className}`;
}

export function logUploadDebug(scope: string, event: string, data?: unknown) {
  if (typeof window === "undefined") {
    return;
  }
  const entry: UploadDebugEntry = {
    ts: new Date().toISOString(),
    scope,
    event,
    data,
  };
  const store = getDebugStore();
  if (store) {
    store.push(entry);
    if (store.length > 400) {
      store.shift();
    }
  }
  console.debug(`[yamdc-upload][${scope}] ${event}`, data);
}
