import { FitAddon, Terminal, init } from "ghostty-web";
import {
  ECHO,
  ECHONL,
  ICANON,
  ICRNL,
  IEXTEN,
  IGNCR,
  INLCR,
  ISIG,
  ISTRIP,
  IXON,
  OPOST,
  Termios,
  TtyServer,
  openpty,
} from "xterm-pty";
import { attachMouseReporting } from "./mouse-adapter";
import { createPtyTerminal, resolveRuntimeURL } from "./terminal-adapter";
import "./style.css";

function requiredElement<T extends Element>(selector: string): T {
  const element = document.querySelector<T>(selector);
  if (!element) throw new Error(`TUI preview is missing ${selector}`);
  return element;
}

const terminalElement = requiredElement<HTMLElement>("#terminal");
const statusElement = requiredElement<HTMLElement>("#status");
const errorElement = requiredElement<HTMLElement>("#error");
const restartButton = requiredElement<HTMLButtonElement>("#restart");

let cleanup: (() => void) | undefined;

function reportError(error: unknown): void {
  const message = error instanceof Error ? error.message : String(error);
  statusElement.textContent = "Failed to start";
  errorElement.textContent = `The browser VM could not start.\n${message}`;
  errorElement.hidden = false;
}

async function start(): Promise<void> {
  cleanup?.();
  terminalElement.replaceChildren();
  errorElement.hidden = true;
  statusElement.textContent = "Loading Ghostty…";

  if (!crossOriginIsolated) {
    throw new Error(
      "Cross-origin isolation is unavailable. Reload the preview once and try again.",
    );
  }

  await init();

  const terminal = new Terminal({
    cols: 100,
    rows: 30,
    cursorBlink: true,
    fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", monospace',
    fontSize: 14,
    theme: {
      background: "#0d1117",
      foreground: "#e6edf3",
      cursor: "#58a6ff",
      selectionBackground: "#264f78",
    },
  });
  const fit = new FitAddon();
  terminal.loadAddon(fit);
  terminal.open(terminalElement);
  // RunDemo requests all-motion + SGR mouse reporting. Prime ghostty-web's
  // input gate as well so early mode sequences emitted while the VM boots are
  // not missed.
  terminal.write("\x1b[?1003h\x1b[?1006h");
  terminal.attachCustomWheelEventHandler(() => terminal.hasMouseTracking());
  const detachMouseReporting = attachMouseReporting(terminal, terminalElement);
  fit.fit();
  fit.observeResize();

  const { master, slave } = openpty();
  const settings = slave.ioctl("TCGETS");
  slave.ioctl(
    "TCSETS",
    new Termios(
      settings.iflag & ~(ISTRIP | INLCR | IGNCR | ICRNL | IXON),
      settings.oflag & ~OPOST,
      settings.cflag,
      settings.lflag & ~(ECHO | ECHONL | ICANON | ISIG | IEXTEN),
      settings.cc,
    ),
  );
  master.activate(createPtyTerminal(terminal));

  statusElement.textContent = "Booting enbu…";
  const worker = new Worker(new URL("./worker.ts", import.meta.url), { type: "module" });
  worker.addEventListener("error", (event) => reportError(event.message));
  worker.addEventListener("message", (event: MessageEvent<unknown>) => {
    if (typeof event.data !== "object" || event.data === null) return;
    const message = event.data as { type?: string; error?: string };
    if (message.type === "ready") {
      window.setTimeout(() => {
        statusElement.textContent = terminal.hasMouseTracking()
          ? "Interactive · keyboard and mouse enabled"
          : "Interactive · keyboard enabled · mouse unavailable";
      }, 250);
    }
    if (message.type === "error") reportError(message.error ?? "Unknown worker error");
  });

  const tty = new TtyServer(slave);
  tty.start(worker);
  worker.postMessage({ type: "init", imageURL: resolveRuntimeURL(window.location.href) });
  terminal.focus();

  cleanup = () => {
    detachMouseReporting();
    tty.stop();
    worker.terminate();
    master.dispose();
    fit.dispose();
    terminal.dispose();
  };
}

restartButton.addEventListener("click", () => void start().catch(reportError));
void start().catch(reportError);
