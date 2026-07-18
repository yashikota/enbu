import { describe, expect, it } from "vite-plus/test";
import { collectPollEvents } from "./poll-events";

describe("collectPollEvents", () => {
  it("returns a clock event when input is pending during a clock-only wait", () => {
    expect(collectPollEvents(undefined, 42n, true)).toEqual([{ userdata: 42n, type: 0 }]);
  });

  it("prefers readable stdin before a clock deadline", () => {
    expect(collectPollEvents(7n, 42n, true)).toEqual([{ userdata: 7n, type: 1 }]);
  });
});
