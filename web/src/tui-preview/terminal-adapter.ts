import type { Terminal } from "ghostty-web";

type Disposable = { dispose(): void };

export type PtyTerminal = {
  write(data: string | Uint8Array, callback?: () => void): void;
  onData(listener: (data: string) => void): Disposable;
  onBinary(listener: (data: string) => void): Disposable;
  onResize(listener: (size: { cols: number; rows: number }) => void): Disposable;
};

/** Adds the one xterm.js event that ghostty-web does not currently expose. */
export function createPtyTerminal(terminal: Terminal): PtyTerminal {
  return {
    write: (data, callback) => terminal.write(data, callback),
    onData: (listener) => terminal.onData(listener),
    onBinary: () => ({ dispose() {} }),
    onResize: (listener) => terminal.onResize(listener),
  };
}

export function resolveRuntimeURL(pageURL: string): string {
  return new URL("../tui/out.wasm", pageURL).href;
}
