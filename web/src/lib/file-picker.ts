import { describeActiveElement, logUploadDebug } from "@/lib/upload-debug";

let pickerInput: HTMLInputElement | null = null;
let pendingResolve: ((file: File | null) => void) | null = null;
let cleanupPendingPicker: (() => void) | null = null;
let pendingRestoreTimer = 0;

function ensurePickerInput() {
  if (pickerInput || typeof document === "undefined") {
    return pickerInput;
  }

  const input = document.createElement("input");
  input.type = "file";
  input.accept = "image/*";
  input.tabIndex = -1;
  input.setAttribute("aria-hidden", "true");
  input.style.position = "fixed";
  input.style.left = "-9999px";
  input.style.top = "0";
  input.style.width = "1px";
  input.style.height = "1px";
  input.style.opacity = "0";
  document.body.appendChild(input);
  pickerInput = input;
  return input;
}

function settlePending(file: File | null) {
  const resolve = pendingResolve;
  pendingResolve = null;
  cleanupPendingPicker?.();
  cleanupPendingPicker = null;
  logUploadDebug("picker", "settle", {
    hasFile: Boolean(file),
    fileName: file?.name ?? null,
    activeElement: describeActiveElement(),
    hasFocus: typeof document !== "undefined" ? document.hasFocus() : null,
  });
  resolve?.(file);
}

function clearRestoreTimer() {
  if (typeof window === "undefined" || !pendingRestoreTimer) {
    return;
  }
  window.clearTimeout(pendingRestoreTimer);
  pendingRestoreTimer = 0;
}

function restoreFocus(target: HTMLElement | null) {
  if (typeof window === "undefined") {
    return;
  }
  clearRestoreTimer();
  pendingRestoreTimer = window.setTimeout(() => {
    pendingRestoreTimer = 0;
    if (pickerInput && document.activeElement === pickerInput) {
      pickerInput.blur();
    }
    if (target && document.contains(target)) {
      target.focus({ preventScroll: true });
      logUploadDebug("picker", "restore-focus", {
        restored: true,
        target: describeActiveElement(),
        hasFocus: document.hasFocus(),
      });
      return;
    }
    window.focus();
    logUploadDebug("picker", "restore-focus", {
      restored: false,
      target: describeActiveElement(),
      hasFocus: document.hasFocus(),
    });
  }, 80);
}

export function pickImageFile() {
  if (typeof window === "undefined") {
    return Promise.resolve<File | null>(null);
  }

  const input = ensurePickerInput();
  if (!input) {
    return Promise.resolve<File | null>(null);
  }

  logUploadDebug("picker", "open", {
    activeElement: describeActiveElement(),
    hasFocus: typeof document !== "undefined" ? document.hasFocus() : null,
  });
  settlePending(null);

  return new Promise<File | null>((resolve) => {
    const previousActiveElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;

    const handleChange = () => {
      logUploadDebug("picker", "change", {
        files: input.files ? Array.from(input.files).map((item) => item.name) : [],
        activeElement: describeActiveElement(),
        hasFocus: typeof document !== "undefined" ? document.hasFocus() : null,
      });
      restoreFocus(previousActiveElement);
      settlePending(input.files?.[0] ?? null);
    };

    const handleCancel = () => {
      logUploadDebug("picker", "cancel", {
        activeElement: describeActiveElement(),
        hasFocus: typeof document !== "undefined" ? document.hasFocus() : null,
      });
      restoreFocus(previousActiveElement);
      settlePending(null);
    };

    pendingResolve = resolve;
    cleanupPendingPicker = () => {
      input.removeEventListener("change", handleChange);
      input.removeEventListener("cancel", handleCancel);
      input.value = "";
    };

    input.addEventListener("change", handleChange, { once: true });
    input.addEventListener("cancel", handleCancel, { once: true });
    input.value = "";
    logUploadDebug("picker", "click-input", {
      previousActiveElement: previousActiveElement ? previousActiveElement.tagName.toLowerCase() : null,
      activeElement: describeActiveElement(),
      hasFocus: typeof document !== "undefined" ? document.hasFocus() : null,
    });
    input.click();
  });
}
