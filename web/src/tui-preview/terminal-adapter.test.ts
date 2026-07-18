import { describe, expect, it, vi } from "vite-plus/test";
import type { Terminal } from "ghostty-web";
import { createPtyTerminal, resolveRuntimeURL } from "./terminal-adapter";

describe("createPtyTerminal", () => {
  it("forwards xterm-compatible operations and supplies onBinary", () => {
    const disposable = { dispose: vi.fn() };
    const write = vi.fn();
    const terminal = {
      write,
      onData: vi.fn(() => disposable),
      onResize: vi.fn(() => disposable),
    } as unknown as Terminal;
    const adapter = createPtyTerminal(terminal);
    const dataListener = vi.fn();
    const resizeListener = vi.fn();

    adapter.write("hello");
    expect(adapter.onData(dataListener)).toBe(disposable);
    expect(adapter.onResize(resizeListener)).toBe(disposable);
    expect(() => adapter.onBinary(vi.fn()).dispose()).not.toThrow();
    expect(write).toHaveBeenCalledWith("hello", undefined);
  });
});

describe("resolveRuntimeURL", () => {
  it("resolves the VM beside the preview root", () => {
    expect(resolveRuntimeURL("https://example.test/enbu/pr-35/web/tui.html")).toBe(
      "https://example.test/enbu/pr-35/tui/out.wasm.gz",
    );
  });
});
