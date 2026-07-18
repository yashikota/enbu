import { describe, expect, it, vi } from "vite-plus/test";
import { SessionController } from "./session-controller";

describe("SessionController", () => {
  it("does not overlap concurrent restarts", async () => {
    const controller = new SessionController();
    let finishStart: (() => void) | undefined;
    const first = controller.restart(
      () =>
        new Promise<void>((resolve) => {
          finishStart = resolve;
        }),
    );
    const overlappingStart = vi.fn(async () => {});

    await expect(controller.restart(overlappingStart)).resolves.toBe(false);
    expect(overlappingStart).not.toHaveBeenCalled();
    finishStart?.();
    await expect(first).resolves.toBe(true);
  });

  it("tears down partially initialized resources after failure", async () => {
    const controller = new SessionController();
    const cleanupOrder: number[] = [];

    await expect(
      controller.restart(async (register) => {
        register(() => cleanupOrder.push(1));
        register(() => cleanupOrder.push(2));
        throw new Error("startup failed");
      }),
    ).rejects.toThrow("startup failed");
    expect(cleanupOrder).toEqual([2, 1]);
  });
});
