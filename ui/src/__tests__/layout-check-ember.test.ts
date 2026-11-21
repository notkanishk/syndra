// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { detect } from "../../scripts/layout-check/detect.js";

// The layout check's ember rule (A-25 §2 rule 1): a scroller is a box that must not cut a light, on each side
// it cannot scroll past. jsdom lays nothing out, so each box is given its place.
const rect = (left: number, top: number, right: number, bottom: number) =>
  ({ left, top, right, bottom, width: right - left, height: bottom - top, x: left, y: top, toJSON() {} }) as DOMRect;

function scene({ scrollTop, scrollHeight }: { scrollTop: number; scrollHeight: number }) {
  document.body.innerHTML = '<div id="sc" style="overflow-x:auto;overflow-y:auto"><span id="dot" data-ember></span></div>';
  const sc = document.getElementById("sc")!;
  const dot = document.getElementById("dot")!;
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    return this === sc ? rect(0, 0, 400, 120) : this === dot ? rect(100, 100, 106, 106) : rect(0, 0, 0, 0);
  });
  // The glow reaches 17px out from the dot: 123 down, past the scroller's 120.
  const real = window.getComputedStyle.bind(window);
  vi.spyOn(window, "getComputedStyle").mockImplementation((el: Element, pseudo?: string | null) => {
    if (pseudo === "::before") return { content: '""', display: "block", left: "-17px", top: "-17px", right: "-17px", bottom: "-17px" } as unknown as CSSStyleDeclaration;
    return real(el);
  });
  Object.defineProperty(sc, "scrollTop", { value: scrollTop, configurable: true });
  Object.defineProperty(sc, "clientHeight", { value: 120, configurable: true });
  Object.defineProperty(sc, "scrollHeight", { value: scrollHeight, configurable: true });
  Object.defineProperty(sc, "scrollWidth", { value: 400, configurable: true });
  Object.defineProperty(sc, "clientWidth", { value: 400, configurable: true });
  Object.defineProperty(window, "innerWidth", { value: 1000, configurable: true });
}

afterEach(() => vi.restoreAllMocks());

describe("the ember rule reads a scroller as a clip only where it cannot scroll past", () => {
  it("reports the light cut at the foot of a scroller that is at its end", () => {
    scene({ scrollTop: 0, scrollHeight: 120 });
    expect(detect().ember.join("\n")).toMatch(/cut 3\.0px at its bottom/);
  });

  it("is quiet at the foot of a scroller that goes on below, since the content scrolls", () => {
    scene({ scrollTop: 0, scrollHeight: 900 });
    expect(detect().ember).toEqual([]);
  });
});
