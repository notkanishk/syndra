// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { detect } from "../../scripts/layout-check/detect.js";

// The layout check's `older` rule (A-25 Q10): the lit card's pool and a flash are spent before their boxes
// and the scrollers that hold them. jsdom lays nothing out, so each box and each computed style is given.
const rect = (left: number, top: number, right: number, bottom: number) =>
  ({ left, top, right, bottom, width: right - left, height: bottom - top, x: left, y: top, toJSON() {} }) as DOMRect;

type Pool = { left: string; top: string; width: string; height: string };
const POOL: Pool = { left: "0px", top: "0px", width: "280px", height: "120px" };

function scene(
  html: string,
  boxes: Record<string, DOMRect>,
  { pool = POOL, image = "radial-gradient(closest-side at 50% 50%, red, blue)" }: { pool?: Pool; image?: string } = {},
) {
  document.body.innerHTML = html;
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    return boxes[this.id] ?? rect(0, 0, 0, 0);
  });
  const real = window.getComputedStyle.bind(window);
  vi.spyOn(window, "getComputedStyle").mockImplementation((el: Element, pseudo?: string | null) => {
    if (pseudo === "::before") return { content: '""', display: "block", ...pool } as unknown as CSSStyleDeclaration;
    const s = real(el);
    if (!el.classList.contains("flash")) return s;
    return { ...s, backgroundImage: image, display: "block", visibility: "visible", opacity: "1", overflowX: "visible", overflowY: "visible" } as unknown as CSSStyleDeclaration;
  });
  // The card's inner box: jsdom has no client size, so it is the rect's.
  const card = document.getElementById("card");
  if (card) {
    Object.defineProperty(card, "clientWidth", { value: boxes.card.width, configurable: true });
    Object.defineProperty(card, "clientHeight", { value: boxes.card.height, configurable: true });
  }
  Object.defineProperty(window, "innerWidth", { value: 1000, configurable: true });
  window.matchMedia = ((q: string) => ({ matches: false, media: q })) as unknown as typeof window.matchMedia;
}

afterEach(() => vi.restoreAllMocks());

describe("the older rule: the lit card", () => {
  const html = '<div id="card" class="lit-card"></div>';

  it("is quiet when the pool's box lies inside the card", () => {
    scene(html, { card: rect(100, 100, 500, 300) });
    expect(detect().older).toEqual([]);
  });

  it("reports a pool cut by its own card, as the old one centred on the card's top edge was", () => {
    // The old pool: 60% x 90px centred at (18%, 0): reaches 90px above the card and 168px past its left.
    scene(html, { card: rect(100, 100, 500, 300) }, { pool: { left: "-168px", top: "-90px", width: "480px", height: "180px" } });
    const found = detect().older.join("\n");
    expect(found).toMatch(/lit card's light is cut 90\.0px at its top/);
    expect(found).toMatch(/cut 168\.0px at its left/);
  });

  it("reports a pool taller than a short card at its foot", () => {
    scene(html, { card: rect(100, 100, 500, 160) });
    expect(detect().older.join("\n")).toMatch(/cut 60\.0px at its bottom/);
  });

  it("reports a pool that measures nothing", () => {
    scene(html, { card: rect(100, 100, 500, 300) }, { pool: { ...POOL, width: "0px" } });
    expect(detect().older.join("\n")).toMatch(/light measures nothing/);
  });
});

describe("the older rule: flash", () => {
  const html = '<div id="sc" style="overflow-x:auto;overflow-y:auto"><p id="row" class="flash"></p></div>';
  const scroller = (extra = {}) => {
    const el = document.getElementById("sc")!;
    for (const [k, v] of Object.entries({ scrollTop: 0, clientHeight: 100, scrollHeight: 100, scrollWidth: 400, clientWidth: 400, ...extra })) {
      Object.defineProperty(el, k, { value: v, configurable: true });
    }
  };

  it("is quiet for a pool inside its scroller", () => {
    scene(html, { sc: rect(0, 0, 400, 100), row: rect(0, 10, 400, 50) });
    scroller();
    expect(detect().older).toEqual([]);
  });

  it("reports a flat wash: a rectangle of light, not a pool in its box", () => {
    scene(html, { sc: rect(0, 0, 400, 100), row: rect(0, 10, 400, 50) }, { image: "none" });
    scroller();
    expect(detect().older.join("\n")).toMatch(/flash that is not a pool spent in its own box/);
  });

  it("reports its box cut at the foot of a scroller at its end, and is quiet where the scroller goes on", () => {
    scene(html, { sc: rect(0, 0, 400, 100), row: rect(0, 80, 400, 130) });
    scroller();
    expect(detect().older.join("\n")).toMatch(/flash's light is cut 30\.0px at its bottom/);
    scroller({ scrollHeight: 900 });
    expect(detect().older).toEqual([]);
  });

  it("is not read under forced colours, where no light is drawn", () => {
    scene(html, { sc: rect(0, 0, 400, 100), row: rect(0, 80, 400, 130) }, { image: "none" });
    window.matchMedia = ((q: string) => ({ matches: q.includes("forced-colors"), media: q })) as unknown as typeof window.matchMedia;
    scroller();
    expect(detect().older).toEqual([]);
  });
});
