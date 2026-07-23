import { useRef } from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import { afterEach, beforeEach, describe, expect, it } from "vite-plus/test";
import { useFocusTrap } from "./use-focus-trap";

function Harness({ active, restoreToTrigger }: { active: boolean; restoreToTrigger: boolean }) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  useFocusTrap(active, dialogRef, restoreToTrigger ? triggerRef : undefined);
  return (
    <>
      <button ref={triggerRef}>Trigger</button>
      {active && (
        <div ref={dialogRef}>
          <button>First</button>
          <button>Last</button>
        </div>
      )}
    </>
  );
}

describe("useFocusTrap", () => {
  let container: HTMLDivElement;
  let root: ReturnType<typeof createRoot>;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it("focuses the first control and wraps Tab in both directions", () => {
    act(() => root.render(<Harness active restoreToTrigger />));
    const buttons = container.querySelectorAll("button");
    const first = buttons[1] as HTMLButtonElement;
    const last = buttons[2] as HTMLButtonElement;
    expect(document.activeElement).toBe(first);

    last.focus();
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", bubbles: true }));
    expect(document.activeElement).toBe(first);

    first.focus();
    document.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Tab", shiftKey: true, bubbles: true }),
    );
    expect(document.activeElement).toBe(last);
  });

  it("restores the trigger or previously focused element on cleanup", () => {
    act(() => root.render(<Harness active restoreToTrigger />));
    const trigger = container.querySelector("button") as HTMLButtonElement;
    act(() => root.render(<Harness active={false} restoreToTrigger />));
    expect(document.activeElement).toBe(trigger);

    const previous = document.createElement("button");
    document.body.appendChild(previous);
    previous.focus();
    act(() => root.render(<Harness active restoreToTrigger={false} />));
    act(() => root.render(<Harness active={false} restoreToTrigger={false} />));
    expect(document.activeElement).toBe(previous);
    previous.remove();
  });
});
