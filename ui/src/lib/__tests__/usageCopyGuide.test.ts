import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import * as copy from "@/components/usage/copy";
import type { StatsUnit } from "@/lib/queries/useStats";

/**
 * The copy guide's section 15 and the statistics surfaces' sentences say the
 * same words: if one moves the other fails. And nothing on them ranks.
 */
const root = resolve(__dirname, "../../../..");
const guide = readFileSync(resolve(root, "openspec/changes/plain-language-copy/design.md"), "utf8");
const section = guide.slice(guide.indexOf("## 15. What has been used"));

const N = "{n}";
const sub = (s: string, from: string | number, to: string) => s.replace(String(from), to);

const SAID = [
  copy.MEMBER_TITLE,
  copy.operatorTitle("{person}"),
  copy.inDays("{value}"),
  copy.inMonths("{value}"),
  copy.NOTHING_COUNTED,
  copy.latestOn("{value}", "{day}"),
  copy.NO_RECENT_VALUE,
  copy.ofLimit("{limit}"),
  copy.NO_LIMIT,
  copy.LIMIT_UNREAD,
  copy.GAPS_HEADING,
  copy.gapLine("{range}", "{where}", "{reason}"),
  copy.gapLine("{range}", "", "{reason}"),
  copy.GAPS_NOTE,
  copy.NOT_COUNTED,
  copy.NOT_COUNTED_NEXT,
  copy.ABSENT,
  copy.UNAVAILABLE,
  copy.OVERVIEW_TITLE,
  copy.OVERVIEW_LEDE,
  copy.lastCounted("{age}"),
  sub(copy.unbound(1), 1, N),
  sub(copy.unbound(2), 2, N),
  sub(copy.retention(777, false), 777, "{days}"),
  sub(copy.retention(777, true), 777, "{days}"),
  copy.droppedLine(["{reason}"]),
  ...Object.values(copy.DROPPED).map((say) => sub(say(555, "{label}"), 555, N)),
  copy.INSIGHTS_TAB,
  copy.INSIGHTS_LEDE,
  copy.windowFrom("{day}"),
  copy.members(1),
  sub(copy.members(2), 2, N),
  copy.spread("{p50}", "{p90}"),
  copy.INSIGHTS_EMPTY,
  copy.INSIGHTS_NEXT,
  copy.INSIGHTS_NEXT_LINK,
  copy.INSIGHTS_LOADING,
  copy.INSIGHTS_ERROR,
  copy.INSIGHTS_SUBJECT,
  copy.dayValue("{day}", "{value}"),
  copy.dayGap("{day}"),
  copy.dayNoValue("{day}"),
  copy.sparkLabel("{what}", false),
  copy.sparkLabel("{what}", true),
  ...Object.values(copy.UNIT_HEADING),
];

describe("the copy guide's section on what has been used", () => {
  it("is in the guide", () => {
    expect(section.length).toBeGreaterThan(0);
  });

  for (const sentence of SAID) {
    it(`carries: ${sentence}`, () => {
      expect(section).toContain(`| ${sentence} |`);
    });
  }

  it("has a row for every unit and every reason the backend can give", () => {
    expect(Object.keys(copy.UNIT_HEADING).sort()).toEqual((["count", "items", "bytes", "seconds", "grams"] as StatsUnit[]).sort());
    expect(Object.keys(copy.DROPPED).sort()).toEqual(
      ["not_a_day", "bad_value", "not_per_subject", "unbound_subject", "undeclared_dimension", "person_in_dimension", "past_retention", "series"].sort(),
    );
  });

  it("ends every card on a next step, in plain words (no em dash, no exclamation, no 'UTC')", () => {
    for (const s of [copy.NOT_COUNTED_NEXT, copy.ABSENT, copy.INSIGHTS_EMPTY, copy.INSIGHTS_NEXT]) expect(s).toContain(copy.NOTHING_TO_DO);
    expect(copy.UNAVAILABLE).toMatch(/Reload this page/);
    for (const s of SAID) {
      expect(s).not.toMatch(/[—!]/);
      expect(s).not.toMatch(/\bUTC\b/);
    }
    expect(section).not.toMatch(/\bUTC\b.*\|$/m);
  });
});

/** Words that would rank a person or a system. */
const RANKING = /\b(top|most|worst|best|leaders?|leaderboards?|rank(s|ed|ing)?|highest|lowest|busiest|heaviest|biggest|hungriest)\b/i;

describe("nothing here ranks", () => {
  const files = [
    resolve(root, "ui/src/lib/usage.ts"),
    ...readdirSync(resolve(root, "ui/src/components/usage"))
      .filter((f) => /\.tsx?$/.test(f))
      .map((f) => resolve(root, "ui/src/components/usage", f)),
  ];
  const strip = (code: string) => code.replace(/\/\*[\s\S]*?\*\/|^\s*\/\/.*$/gm, "");

  it("finds the files it reads", () => {
    expect(files.length).toBeGreaterThan(5);
  });

  it("finds the words it is looking for", () => {
    for (const w of ["Top users", "most active", "the worst", "Leaderboard", "ranked", "highest use"]) expect(w).toMatch(RANKING);
    for (const w of ["Half are at", "nine in ten", "Sizes", "topic"]) expect(w).not.toMatch(RANKING);
  });

  it("is in no sentence", () => {
    expect(SAID.filter((s) => RANKING.test(s))).toEqual([]);
  });

  it("is in no source, outside comments", () => {
    for (const file of files) expect(strip(readFileSync(file, "utf8")), file).not.toMatch(RANKING);
  });

  it("is in no row of the guide's section", () => {
    const rows = section.split("\n").filter((l) => l.startsWith("| ") && !l.startsWith("| When"));
    expect(rows.filter((r) => RANKING.test(r))).toEqual([]);
  });
});
