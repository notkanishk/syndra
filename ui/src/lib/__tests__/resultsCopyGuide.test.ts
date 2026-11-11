import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  ALSO_SAYS,
  NO_LIMIT,
  NOT_READABLE_NEXT,
  emptyEvents,
  emptyMeters,
  emptyRecords,
  noMoreLines,
  notReadableBody,
  notReadableTitle,
  uncoveredNote,
  usageUnread,
} from "@/components/results/copy";
import { NOTHING_TO_DO } from "@/lib/outcome";

/**
 * The copy guide's section 12 and the read renderers' sentences say the same
 * words. If one moves the other fails, so the guide is never last month's copy.
 */
const guide = readFileSync(resolve(__dirname, "../../../../openspec/changes/plain-language-copy/design.md"), "utf8");
const section = guide.slice(guide.indexOf("## 12. What an add-on says about a person"));

const N = "{name}";
const SAID = [
  emptyRecords(N),
  emptyMeters(N),
  emptyEvents(N),
  notReadableTitle(N),
  notReadableBody(N),
  NOT_READABLE_NEXT,
  usageUnread(N, true),
  usageUnread(N, false),
  NO_LIMIT,
  ALSO_SAYS,
  noMoreLines(0).replace("0", "{n}"),
  uncoveredNote("{places}", N),
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
    for (const s of [emptyRecords(N), emptyMeters(N), emptyEvents(N)]) expect(s.endsWith(NOTHING_TO_DO)).toBe(true);
    for (const s of SAID) expect(s).not.toMatch(/[—!]/);
  });
});
