type WailsRuntime = {
  BrowserOpenURL?: (url: string) => void;
  LogDebug?: (message: string) => void;
  LogInfo?: (message: string) => void;
  LogWarning?: (message: string) => void;
  LogError?: (message: string) => void;
};

type WindowWithWails = Window & {
  runtime?: WailsRuntime;
};

let installed = false;

function runtime(): WailsRuntime | undefined {
  return (window as WindowWithWails).runtime;
}

function stringifyDetail(detail?: unknown): string {
  if (detail === undefined) return "";
  if (typeof detail === "string") return detail;
  try {
    return JSON.stringify(detail);
  } catch {
    return "[unserializable]";
  }
}

function log(level: "debug" | "info" | "warning" | "error", event: string, detail?: unknown): void {
  const message = `[frontend] ${event}${detail === undefined ? "" : ` ${stringifyDetail(detail)}`}`;
  const wailsRuntime = runtime();
  switch (level) {
    case "error":
      wailsRuntime?.LogError?.(message);
      break;
    case "warning":
      wailsRuntime?.LogWarning?.(message);
      break;
    case "info":
      wailsRuntime?.LogInfo?.(message);
      break;
    default:
      wailsRuntime?.LogDebug?.(message);
  }
}

function eventTargetName(target: EventTarget | null): string {
  if (!(target instanceof Element)) return "";
  return target.tagName.toLowerCase();
}

export function installWailsEventLogging(): void {
  if (installed) return;
  installed = true;

  log("info", "instrumentation-installed", {
    href: window.location.href,
    userAgent: navigator.userAgent,
  });

  window.addEventListener("focus", () => log("debug", "window-focus"));
  window.addEventListener("blur", () => log("debug", "window-blur"));
  window.addEventListener("pageshow", (event) =>
    log("debug", "page-show", { persisted: event.persisted }),
  );
  window.addEventListener("pagehide", (event) =>
    log("debug", "page-hide", { persisted: event.persisted }),
  );
  window.addEventListener("beforeunload", () => log("warning", "before-unload"));
  document.addEventListener("visibilitychange", () =>
    log("debug", "visibility-change", { state: document.visibilityState }),
  );
  document.addEventListener("click", (event) => {
    const anchor = (event.target as Element | null)?.closest?.(
      "a[target='_blank']",
    ) as HTMLAnchorElement | null;
    if (!anchor) {
      log("debug", "document-click", { target: eventTargetName(event.target) });
      return;
    }

    log("warning", "target-blank-click", { href: anchor.href });
    const wailsRuntime = runtime();
    if (!wailsRuntime?.BrowserOpenURL) return;
    event.preventDefault();
    wailsRuntime.BrowserOpenURL(anchor.href);
  });
  window.addEventListener("error", (event) =>
    log("error", "window-error", {
      message: event.message,
      filename: event.filename,
      lineno: event.lineno,
      colno: event.colno,
    }),
  );
  window.addEventListener("unhandledrejection", (event) =>
    log("error", "unhandled-rejection", stringifyDetail(event.reason)),
  );

  const originalOpen = window.open.bind(window);
  window.open = (url?: string | URL, target?: string, features?: string): WindowProxy | null => {
    const href = url?.toString() ?? "";
    log("warning", "window-open", { url: href, target, features });
    const wailsRuntime = runtime();
    if (href && wailsRuntime?.BrowserOpenURL) {
      wailsRuntime.BrowserOpenURL(href);
      return null;
    }
    return originalOpen(url, target, features);
  };
}
