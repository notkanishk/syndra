import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import * as copy from "@/components/actions/copy";
import { NOTHING_TO_DO } from "@/lib/outcome";

/**
 * The copy guide's section 14 and the sentences a member's physical action says
 * are the same words. If one moves the other fails.
 */
const root = resolve(__dirname, "../../../..");
const guide = readFileSync(resolve(root, "openspec/changes/plain-language-copy/design.md"), "utf8");
const section = guide.slice(guide.indexOf("## 14. Acting on things in the world"));

const stop = (s: string) => (/[.!?]$/.test(s) ? s : `${s}.`);
const NOTHING_CHANGED = "Nothing was changed.";
// A refusal is shown as its message, "Nothing was changed." unless it says so itself, then its step.
const refused = (code: string, details?: Record<string, string>) => {
  const r = copy.refusal(code, details)!;
  const said = stop(r.message);
  return [said, /nothing (was changed|went out)/i.test(r.message) ? "" : NOTHING_CHANGED, r.next].filter(Boolean).join(" ");
};

const DETAILS: Record<string, Record<string, string>> = {
  UNKNOWN_RESOURCE: { param: "{param}" },
  NOT_ENTITLED: { param: "{param}" },
  LIMIT_REACHED: { count: "{n}", limit: "{limit}", unit: "{unit}" },
};

const PLAIN = [
  copy.FORM_LEDE,
  copy.TARGET_ID_HINT,
  copy.SCAN_BUTTON,
  copy.SCAN_TITLE,
  copy.SCAN_LEDE,
  copy.SCAN_STARTING,
  copy.SCAN_DENIED,
  copy.PREVIEW_BUTTON,
  copy.PREVIEW_LEDE,
  copy.previewNote("{name}"),
  copy.PREVIEW_NO_CHANGE,
  copy.PREVIEW_UNREADABLE_TITLE,
  copy.previewUnreadableBody("{name}"),
  copy.PREVIEW_UNREADABLE_NEXT,
  copy.CONFIRM_BUTTON,
  copy.UNSETTLED_REASON,
  copy.DONE,
  copy.NO_CHANGE,
  copy.NOT_DONE,
  copy.NOT_DONE_NEXT,
  copy.CHECKING,
  copy.CHECKING_DETAIL,
  copy.CHECKING_NEXT,
  copy.CHECKING_LATE,
  copy.ASK_DESK,
  copy.ASK_DESK_DETAIL,
  copy.ASK_DESK_NEXT,
];
const REFUSALS = copy.REFUSAL_CODES.map((code) => refused(code, DETAILS[code]));
const WORDS = Object.values(copy.WORD);

describe("the copy guide's section on acting on things in the world", () => {
  it("is in the guide", () => expect(section.length).toBeGreaterThan(0));

  for (const sentence of [...PLAIN, ...REFUSALS]) {
    it(`carries: ${sentence}`, () => expect(section).toContain(`| ${sentence} |`));
  }

  it("names the five words and walks every code the page words", () => {
    for (const word of WORDS) expect(section, word).toContain(`**${word}**`);
    expect(new Set(copy.REFUSAL_CODES).size).toBe(15);
    for (const code of copy.REFUSAL_CODES) expect(copy.refusal(code, DETAILS[code]), code).toBeDefined();
    expect(copy.refusal("SOMETHING_NEW")).toBeUndefined();
  });

  it("is said in plain words (no em dash, no exclamation)", () => {
    for (const s of [...PLAIN, ...REFUSALS, ...WORDS]) expect(s).not.toMatch(/[—!]/);
  });

  it("never says the member caused what is not theirs", () => {
    for (const s of [...PLAIN, ...REFUSALS]) expect(s).not.toMatch(/\byou (failed|forgot|should have|did not|didn't|made a mistake|got)\b|\bwrong\b|\bfault\b|\binvalid\b/i);
  });

  it("every refusal ends on a step, and so does every state", () => {
    for (const code of copy.REFUSAL_CODES) {
      const r = copy.refusal(code, DETAILS[code])!;
      expect(r.next.length, code).toBeGreaterThan(8);
      expect(r.next, code).toMatch(/[.]$/);
    }
    // The ones with no step to take say so.
    for (const s of [copy.CHECKING_NEXT, copy.PREVIEW_NO_CHANGE]) expect(s).toContain(NOTHING_TO_DO);
    expect(copy.NOT_DONE_NEXT).toMatch(/try again/i);
    expect(copy.ASK_DESK_NEXT).toMatch(/desk/i);
  });

  it("carries the operator's row, one row per sentence", () => {
    const w = copy.checkoutWording("{item}", "{person}", "{name}");
    for (const row of [
      `Check out {item} to {person}`,
      w.question,
      w.tell,
      w.yes,
      w.no,
      w.done,
      w.notDone,
    ]) {
      expect(section, row).toContain(`| ${row} |`);
    }
    expect(w.tell).toContain("Ask at the desk whether {item} went out to {person}");
  });

  // One home each: a component that spells one out inline would drift from the guide without the table failing.
  it("is written nowhere but copy.ts", () => {
    const dir = resolve(root, "ui/src/components/actions");
    const found: string[] = [];
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isFile() || !/\.tsx?$/.test(entry.name) || entry.name === "copy.ts") continue;
      const code = readFileSync(resolve(dir, entry.name), "utf8").replace(/\/\*[\s\S]*?\*\/|^\s*\/\/.*$/gm, "");
      if (/still being checked|went through|did not happen|not answering|Nothing went out/i.test(code)) found.push(entry.name);
    }
    expect(found).toEqual([]);
  });
});
