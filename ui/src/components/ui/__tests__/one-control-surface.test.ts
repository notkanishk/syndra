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

/**
 * The second pass (2026-08-27), after an adoption form shipped in the wrong
 * colour, in the wrong place, and without the motion everything else opens
 * with.
 *
 * Each rule below already existed — in `Button`'s doc comment, in the IA
 * design, in a review comment on a neighbouring file. None of them was checked
 * against the source, and every one of them had been broken somewhere.
 */
describe("a control states its role in the vocabulary the product already has", () => {
  /**
   * The first guard in this file catches a pill that ALSO claims the touch
   * floor — the careful copy. It could never catch the careless one, which is
   * the copy that matters: a hand-rolled pill that simply forgot the floor is
   * both a drifted surface AND a 30px target on a phone.
   */
  it("has no pill control that sits below the touch floor", () => {
    const offenders = relevant().flatMap(({ path, source }) =>
      [...source.matchAll(/<button\b/g)]
        .map((m) => ({ at: m.index!, tag: tagAt(source, m.index!) }))
        .filter(({ tag }) => /rounded-pill/.test(tag))
        .filter(({ tag }) => !/min-h-\[44px\]|min-h-11|PILL/.test(tag))
        .map(({ at }) => `${path}:${source.slice(0, at).split("\n").length}`),
    );

    expect(
      offenders,
      "import PILL, or use Button — 44px is the floor through the tablet range",
    ).toEqual([]);
  });

  /**
   * A `<label>` names its control. Everything inside it becomes that name, so
   * a hint written between the label's text and its closing tag makes the
   * control's accessible name the title plus a paragraph — read out in full,
   * every time, by the people who cannot see the layout that made it look
   * fine. `FieldLabel` + `FieldHint` put the hint outside, where it is still
   * read by everybody and named by nobody.
   */
  it("writes no field hint inside the label that names a control", () => {
    const offenders = relevant().flatMap(({ path, source }) =>
      [...source.matchAll(/<label\b[^>]*>([\s\S]*?)<\/label>/g)]
        .filter((m) => /<(Input|textarea|select|Select)\b/.test(m[1]))
        .filter((m) => {
          const control = /<(Input|textarea|select|Select)\b/.exec(m[1])!;
          const after = m[1].slice(control.index + control[0].length);
          return /text-faint|text-\[13px\]/.test(after);
        })
        .map((m) => `${path}:${source.slice(0, m.index!).split("\n").length}`),
    );

    expect(
      offenders,
      "use FieldLabel + FieldHint — inside the label, the hint becomes part of the control's name",
    ).toEqual([]);
  });

  /**
   * `dangerConfirm` is a solid red fill, and red is this product's word for a
   * click that TAKES SOMETHING AWAY. It is not a synonym for "irreversible":
   * adopting an account cannot be undone either, and it hands somebody a home
   * directory. Dressing a grant in red spends the colour on the wrong act, and
   * what it costs is the next real revocation, which then reads as routine.
   *
   * Checked by the button's own label, because the label is the sentence the
   * operator reads. Two confirms destroy something the verb does not name;
   * they are listed here with the argument, which is the point of listing them
   * rather than loosening the rule.
   */
  it("spends the destructive fill only on a destructive act", () => {
    const TAKES_AWAY = /remov|revok|revoc|delet|purg|take away|withdraw|drop|clear|end |stop/i;
    const ARGUED: Record<string, string> = {
      "Record the owner":
        "the other claimant stops holding the account in Syndra, immediately and without being told",
      "Accept this log and start over":
        "the only act in the product that discards evidence — the missing records stay missing and Syndra stops being able to say so",
    };

    const offenders = relevant().flatMap(({ path, source }) =>
      [...source.matchAll(/variant="dangerConfirm"/g)]
        .map((m) => ({ at: m.index!, label: confirmLabel(source, m.index!) }))
        .filter(({ label }) => !TAKES_AWAY.test(label) && !(label in ARGUED))
        .map(({ at, label }) => `${path}:${source.slice(0, at).split("\n").length} — "${label}"`),
    );

    expect(
      offenders,
      "a confirm that grants is `accent`; irreversibility is carried by the rung and the copy",
    ).toEqual([]);
  });

  /**
   * A borderless control is invisible as a control in a table row: it reads as
   * text until hovered, and hover does not exist on a touch device and does not
   * survive a screenshot sent to a colleague. `ghost` is for the quieter half
   * of a PAIR — a Cancel beside a confirm — so a row whose only affordance is
   * ghost has no visible affordance at all.
   */
  it("never makes a row's only action the borderless one", () => {
    const offenders = relevant().flatMap(({ path, source }) =>
      [...source.matchAll(/<CardRow\b/g)]
        .map((m) => ({ at: m.index!, end: source.indexOf("</CardRow>", m.index!) }))
        .filter(({ end }) => end > 0)
        .map(({ at, end }) => ({ at, variants: [...source.slice(at, end).matchAll(/variant="(\w+)"/g)].map((v) => v[1]) }))
        .filter(({ variants }) => variants.length > 0 && variants.every((v) => v === "ghost"))
        .map(({ at }) => `${path}:${source.slice(0, at).split("\n").length}`),
    );

    expect(offenders, "a row's one action is `outline`, or `danger` when it destroys access").toEqual(
      [],
    );
  });

  /**
   * A card's way onward — "See all", "Full audit log" — is `CardHeaderLink`.
   *
   * Written by hand it is a bare line of text, and the two that existed had
   * already disagreed about whether it was a control at all: one carried the
   * touch floor and the other was 20px of text in the corner of a card.
   */
  it("has no hand-written way out of a card header", () => {
    const offenders = relevant().flatMap(({ path, source }) =>
      [...source.matchAll(/action=\{/g)]
        // The action's own braces, not a fixed window: a link inside the card's
        // prose is a link in a sentence, and giving THAT a 44px box would break
        // the line it sits in.
        .map((m) => ({ at: m.index!, block: braced(source, m.index! + m[0].length - 1) }))
        .filter(({ block }) => /<Link\b[^>]*className="[^"]*text-accent-text/.test(block))
        .map(({ at }) => `${path}:${source.slice(0, at).split("\n").length}`),
    );

    expect(offenders, "use CardHeaderLink — it is this link, with the floor attached").toEqual([]);
  });
});

/** The full text of a JSX tag starting at `at`, balancing braces and strings. */
function tagAt(source: string, at: number): string {
  let i = source.indexOf(" ", at);
  let depth = 0;
  let quote: string | null = null;
  while (i < source.length) {
    const c = source[i];
    if (quote) {
      if (c === "\\") i += 1;
      else if (c === quote) quote = null;
    } else if (c === '"' || c === "'" || c === "`") quote = c;
    else if (c === "{") depth += 1;
    else if (c === "}") depth -= 1;
    else if (c === ">" && depth === 0) break;
    i += 1;
  }
  return source.slice(at, i + 1);
}

/**
 * The label a confirming button carries — the last string literal before its
 * closing tag, which is what the operator actually reads on the control.
 */
function confirmLabel(source: string, at: number): string {
  const close = source.indexOf("</Button>", at);
  const body = source.slice(at, close < 0 ? at + 600 : close);
  const literals = [...body.matchAll(/[>:]\s*\{?\s*["`]?([A-Z][^<>{}"`\n]{3,60})/g)].map((m) =>
    m[1].trim().replace(/[.…]+$/, ""),
  );
  return literals[literals.length - 1] ?? "";
}

/** The text of a `{...}` expression starting at the opening brace. */
function braced(source: string, open: number): string {
  let depth = 0;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === "{") depth += 1;
    else if (source[i] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(open, i + 1);
    }
  }
  return source.slice(open);
}
