import { describe, expect, it } from "vite-plus/test";
import { encodeMouseEvent, encodeWheelEvent } from "./mouse-adapter";

const position = { column: 12, row: 7 };

describe("encodeMouseEvent", () => {
  it("encodes press and release using the SGR mouse protocol", () => {
    const event = new MouseEvent("mousedown", { button: 0 });
    expect(encodeMouseEvent(event, position, "press")).toBe("\x1b[<0;12;7M");
    expect(encodeMouseEvent(event, position, "release")).toBe("\x1b[<0;12;7m");
  });

  it("encodes all-motion events and modifiers", () => {
    const event = new MouseEvent("mousemove", { buttons: 0, ctrlKey: true });
    expect(encodeMouseEvent(event, position, "move")).toBe("\x1b[<51;12;7M");
  });
});

describe("encodeWheelEvent", () => {
  it("encodes both wheel directions", () => {
    expect(encodeWheelEvent(new WheelEvent("wheel", { deltaY: -1 }), position)).toBe(
      "\x1b[<64;12;7M",
    );
    expect(encodeWheelEvent(new WheelEvent("wheel", { deltaY: 1 }), position)).toBe(
      "\x1b[<65;12;7M",
    );
  });
});
