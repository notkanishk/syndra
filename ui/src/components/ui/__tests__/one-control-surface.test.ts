import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * One surface per control, enforced against the source.
 *
 * Every finding this file guards was found by looking at a screen, not by
 * reading code — which is the problem. A hand-rolled pill is invisible in
 * review: it renders almost right, and the "almost" is a pixel of padding or a
 * press animation that does not fire. By the time anybody notices, there are
 * four copies and they disagree in four different ways. That is exactly what
 * had happened: three copies of the "load more" pill, two copies of the tab
 * row, and three copies of a status pill, one of which had lost its weight.
 *
 * So the rule is checked here rather than trusted: outside `components/ui`,
 * nothing rebuilds a control's surface by hand. The primitives are `Button`,
 * `ButtonLink`, `Tabs`, `FilterPills` and `Badge`.
 */

const SRC = join(import.meta.dirname, "../../..");

/**
 * The shell's own chrome and the error pages.
 *
 * The rail, the tab bar and the account sheet are one-off compositions with
 * their own layout and hit targets — they are not instances of a control that
 * exists elsewhere, and forcing them through `Button` would mean adding props
 * that only they use. `error.tsx` and `global-error.tsx` render when the app
 * has already failed, and they must not depend on anything that might be the
 * thing that failed.
 */
const EXEMPT = [
  "components/shell/",
  "components/ErrorBoundary.tsx",
  "app/error.tsx",
  "app/global-error.tsx",
  "app/not-found.tsx",
];

function tsxFiles(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      if (entry === "__tests__" || entry === "node_modules") continue;
      tsxFiles(full, out);
      continue;
    }
    if (entry.endsWith(".tsx")) out.push(full);
  }
  return out;
}

function relevant(): Array<{ path: string; source: string }> {
  return tsxFiles(join(SRC, "components"))
    .concat(tsxFiles(join(SRC, "app")))
    .map((path) => ({ path: path.slice(SRC.length + 1), source: readFileSync(path, "utf8") }))
    .filter(({ path }) => !path.startsWith("components/ui/"))
    .filter(({ path }) => !EXEMPT.some((prefix) => path.startsWith(prefix)));
}

describe("one surface per control", () => {
  // The signature of a hand-rolled control: a pill that also claims the touch
  // floor. Prose never does both.
  it("has no hand-rolled pill control outside the primitives", () => {
    const offenders = relevant()
      .filter(({ source }) => /rounded-pill/.test(source))
      .filter(({ source }) => /min-h-\[44px\]|min-h-11/.test(source))
      .flatMap(({ path, source }) =>
        source
          .split("\n")
          .map((line, i) => ({ path, line: i + 1, text: line.trim() }))
          .filter(
            (row) =>
              /rounded-pill/.test(row.text) && /min-h-\[44px\]|min-h-11/.test(row.text),
          ),
      )
      .map((row) => `${row.path}:${row.line}`);

    expect(
      offenders,
      "use Button / ButtonLink / Tabs / FilterPills — a copy of their classes drifts from them silently",
    ).toEqual([]);
  });

  // `Badge`'s box, written out by hand. Three of these existed in one function,
  // and one had lost `font-semibold`, so the same pill rendered at two weights
  // depending on which branch produced it.
  it("has no hand-rolled status pill outside Badge", () => {
    const offenders = relevant()
      .flatMap(({ path, source }) =>
        source
          .split("\n")
          .map((line, i) => ({ path, line: i + 1, text: line.trim() }))
          .filter(
            (row) =>
              /rounded-pill/.test(row.text) &&
              /px-2\.5/.test(row.text) &&
              /text-\[12\.5px\]/.test(row.text),
          ),
      )
      .map((row) => `${row.path}:${row.line}`);

    expect(offenders, "use Badge — it is this pill, with its tones named").toEqual([]);
  });
});
