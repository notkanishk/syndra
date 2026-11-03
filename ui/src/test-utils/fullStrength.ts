import { expect } from "vitest";

/**
 * Stale data is never dimmed (A-13 (d), RULES §2): a card drawn from the last
 * read recedes to the page and keeps its words at full strength. Fails if the
 * element, or anything inside it, carries an opacity utility or an inline
 * opacity below 1 — the shape the 0.62 dim shipped in, twice.
 */
export function expectFullStrength(root: Element): void {
  for (const el of [root, ...Array.from(root.querySelectorAll("*"))]) {
    const cls = el.getAttribute("class") ?? "";
    expect(cls, `${el.tagName} is dimmed: ${cls}`).not.toMatch(/(^|\s)opacity-(?!100\b)\S+/);
    const inline = (el as HTMLElement).style?.opacity;
    if (inline) expect(Number(inline), `${el.tagName} has opacity ${inline}`).toBe(1);
  }
}
