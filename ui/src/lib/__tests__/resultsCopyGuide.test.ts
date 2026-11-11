import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  ALSO_SAYS,
  NO_LIMIT,
  UNCOVERED_GUIDANCE,
  dueDate,
  emptyEvents,
  emptyMeters,
  emptyRecords,
  noMoreLines,
  notReadableBody,
  notReadableNext,
  notReadableTitle,
  ofLimit,
  sinceDate,
  uncoveredNote,
  usageUnread,
  usageUnreadBody,
  type Reader,
} from "@/components/results/copy";
import { NOTHING_TO_DO } from "@/lib/outcome";

/**
 * The copy guide's section 12 and the read renderers' sentences say the same
 * words. If one moves the other fails, so the guide is never last month's copy.
 */
const guide = readFileSync(resolve(__dirname, "../../../../openspec/changes/plain-language-copy/design.md"), "utf8");
const section = guide.slice(guide.indexOf("## 12. What an add-on says about a person"));

const N = "{name}";
const OP: Reader = { audience: "operator", person: "{person}" };
const MEMBER: Reader = { audience: "member" };
const PLACES = [["{place}"], ["{place}", "{place}"]];
const SAID = [
  emptyRecords(N, MEMBER),
  emptyRecords(N, OP),
  emptyMeters(N),
  emptyEvents(N),
  notReadableTitle(N),
  notReadableBody(N),
  notReadableNext(MEMBER),
  notReadableNext(OP),
  usageUnread(N, true),
  usageUnread(N, false),
  usageUnreadBody(true),
  usageUnreadBody(false),
  NO_LIMIT,
  ofLimit("{limit}"),
  sinceDate("{date}"),
  dueDate("{date}"),
  ALSO_SAYS,
  noMoreLines(0).replace("0", "{n}"),
  ...[MEMBER, OP].flatMap((who) => PLACES.map((places) => uncoveredNote(places, who))),
  UNCOVERED_GUIDANCE,
];

describe("the copy guide's section on what an add-on says", () => {
  it("is in the guide", () => {
    expect(section.length).toBeGreaterThan(0);
    expect(guide).toContain("## 12. What an add-on says about a person");
  });

  for (const sentence of SAID) {
    it(`carries: ${sentence}`, () => {
      expect(section).toContain(`| ${sentence} |`);
    });
  }

  it("ends every empty state on a next step, and says each in plain words (no em dash, no exclamation)", () => {
    for (const s of [emptyRecords(N, MEMBER), emptyRecords(N, OP), emptyMeters(N), emptyEvents(N)]) expect(s.endsWith(NOTHING_TO_DO)).toBe(true);
    for (const s of SAID) expect(s).not.toMatch(/[—!]/);
  });
});

/**
 * No sentence lives in a renderer. A string literal or JSX text in
 * `results/*.tsx` that reads as words (two or more, starting with a capital) is
 * a sentence that belongs in `copy.ts`, where the guide holds it. Class names,
 * attributes and one-word labels are not sentences.
 */
const dir = resolve(__dirname, "../../components/results");
const sentence = /^[A-Z][a-z']+(?: [A-Za-z'.,…-]+)+$/;

function sentencesIn(source: string): string[] {
  const code = source
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:])\/\/.*$/gm, "$1")
    .replace(/\{\/\*[\s\S]*?\*\/\}/g, "");
  const found: string[] = [];
  for (const m of code.matchAll(/"([^"\n]*)"|`([^`\n]*)`|>([^<>{}\n]+)</g)) {
    const text = (m[1] ?? m[2] ?? m[3] ?? "").replace(/\$\{[^}]*\}/g, "x").trim();
    if (sentence.test(text)) found.push(text);
  }
  return found;
}

describe("no sentence lives in a results renderer", () => {
  for (const file of readdirSync(dir).filter((f) => f.endsWith(".tsx"))) {
    it(`${file} says everything through copy.ts`, () => {
      expect(sentencesIn(readFileSync(resolve(dir, file), "utf8"))).toEqual([]);
    });
  }

  it("does catch an inline sentence, a template one and JSX text, and passes a class name and a one-word label", () => {
    expect(sentencesIn('const a = "Some usage was missing.";')).toHaveLength(1);
    expect(sentencesIn("const a = `Since ${d} onwards`;")).toHaveLength(1);
    expect(sentencesIn("<p>Nothing for you to do.</p>")).toHaveLength(1);
    expect(sentencesIn('<b className="flex flex-col gap-3">Refused</b>')).toEqual([]);
  });
});
