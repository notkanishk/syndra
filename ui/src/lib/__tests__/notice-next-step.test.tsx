// @vitest-environment jsdom
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Notice } from "@/components/ui/Notice";
import { NOTHING_TO_DO } from "@/lib/outcome";

/**
 * Owner, 6 Nov 2026: every outcome ends on a clear next step or an explicit
 * "nothing for you to do". Notice states which through its types (a caller
 * that gives neither does not compile: `bun run typecheck`), and this guard
 * covers what types cannot: spread props, casts, and a blank sentence.
 */
describe("Notice carries its next step", () => {
  it("draws the next step as its last line", () => {
    const { container } = render(
      <Notice tone="danger" title="That didn't save." next="Try again.">
        The server did not answer.
      </Notice>,
    );
    expect(container.querySelector("[data-notice-next]")?.textContent).toBe("Try again.");
  });

  it("says 'Nothing for you to do.' and why, when there is nothing", () => {
    const { container } = render(
      <Notice tone="warn" title="Paused" nothingToDo="Syndra carries on by itself.">
        Account changes are paused.
      </Notice>,
    );
    expect(container.querySelector("[data-notice-next]")?.textContent).toBe(`${NOTHING_TO_DO} Syndra carries on by itself.`);
  });

  it("does not compile without one", () => {
    // @ts-expect-error neither `next` nor `nothingToDo`: a dead end
    const dead = <Notice tone="warn" title="Stale">x</Notice>;
    // @ts-expect-error both at once says two things
    const both = <Notice tone="warn" title="Stale" next="Reload." nothingToDo="It reloads.">x</Notice>;
    expect(dead).toBeTruthy();
    expect(both).toBeTruthy();
  });
});

function sources(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) {
      if (name !== "__tests__" && name !== "node_modules") sources(p, out);
    } else if (p.endsWith(".tsx")) out.push(p);
  }
  return out;
}

describe("every Notice in the app is given one", () => {
  const root = join(__dirname, "..", "..");
  let seen = 0;
  for (const file of sources(root)) {
    const text = readFileSync(file, "utf8");
    const parts = text.split("<Notice");
    for (let i = 1; i < parts.length; i++) {
      // Skip other components that start with the same letters (<Notices />).
      if (/^[A-Za-z]/.test(parts[i])) continue;
      seen++;
      const end = parts[i].indexOf("</Notice>");
      const tag = end === -1 ? parts[i] : parts[i].slice(0, end);
      it(`${file.slice(root.length + 1)} #${i}`, () => {
        expect(tag).toMatch(/\bnext[=:]|\bnothingToDo[=:]/);
        // Copy guide: no em dashes, no exclamation marks, no blank step.
        for (const m of tag.matchAll(/\b(?:next|nothingToDo)[=:]\s*(?:\{?\s*)?(?:"([^"]*)"|`([^`]*)`)/g)) {
          const said = m[1] ?? m[2];
          expect(said.trim()).not.toBe("");
          expect(said).not.toMatch(/[—!]/);
        }
      });
    }
  }
  it("finds the Notices", () => {
    expect(seen).toBeGreaterThan(40);
  });
});
