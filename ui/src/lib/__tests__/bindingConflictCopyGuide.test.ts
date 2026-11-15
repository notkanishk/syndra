import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  CONFLICT_NEXT_FALLBACK,
  OTHER_PERSON,
  STOP_TRACKING,
  conflictDetailFallback,
  holdersLine,
  stopTrackingConsequence,
} from "@/components/targets/bindingConflict";
import { inventoryNotRead, whyNotChecked } from "@/lib/format";
import { BINDING_CONFLICT, LOOKING_UP_NEXT, TARGET_REFUSED, refusalCause, stoppedLine } from "@/lib/wait-reason";

/**
 * The copy guide's section 16 rows for the binding conflict (addon-contract 7t-4)
 * and the sentences the renderers say are the same words, whole sentences and not
 * their first halves. If one moves, the other fails, so the guide is never last
 * month's copy.
 */
const guide = readFileSync(resolve(__dirname, "../../../../openspec/changes/plain-language-copy/design.md"), "utf8");
const section = guide.slice(guide.indexOf("## 16. The system page, after the visual check"));
const S = "{system}";
const row = { attempts: 5 };

describe("the binding conflict's copy guide rows", () => {
  it("quote every whole sentence the page says around the add-on's words", () => {
    const said = [
      `${conflictDetailFallback(S)} ${CONFLICT_NEXT_FALLBACK}`,
      `${OTHER_PERSON} {name}.`,
      holdersLine(S, ["{a}", "{b}"]),
      STOP_TRACKING,
      stopTrackingConsequence(S),
      LOOKING_UP_NEXT,
      stoppedLine(S, { ...row, last_error: "out of retries after 5 attempts: x" }).replace("5 tries", "{n} tries"),
      stoppedLine(S, { ...row, wait_reason: TARGET_REFUSED, last_error: "{its words}." }).replace("{its words}.", "{its words}"),
      stoppedLine(S, { ...row, wait_reason: TARGET_REFUSED }),
      stoppedLine(S, { ...row, wait_reason: "refused:not_found" }).replace(refusalCause("refused:not_found"), "{cause in words}"),      stoppedLine(S, { ...row, wait_reason: "halted:x" }),
      inventoryNotRead(S, undefined, "target_unreachable"),
      whyNotChecked("a_reason_nobody_worded"),
      whyNotChecked("a_reason_nobody_worded", S),
    ];
    const missing = said.filter((line) => !section.includes(line.replace("The account list could not be read. ", "").replace(" Read again once that is fixed.", "")) );
    expect(missing, "design.md §16 does not carry these words").toEqual([]);
  });

  it("quotes the inventory's stop with its whole lead and tail", () => {
    expect(inventoryNotRead(S, undefined, "target_unreachable")).toBe(
      "The account list could not be read. {system} did not answer when Syndra last tried. Read again once that is fixed.",
    );
    expect(section).toContain("The account list could not be read.");
    expect(section).toContain("Read again once that is fixed.");
    expect(section).toContain("{system} did not answer when Syndra last tried.");
  });

  it("says Syndra's own disagreement sentence the way the backend stores it, and the pages lead it with the system", () => {
    const backend = readFileSync(resolve(__dirname, "../../../../backend/internal/services/propagation/addon.go"), "utf8");
    // The whole stored sentence, both halves, against the Go source and the guide.
    const first = "Two of Syndra's records disagree about whose account";
    const last = "Decide whose it is before changing anything else for them.";
    expect(backend).toContain(first);
    expect(backend).toContain(last);
    expect(section).toContain(`${first} {name} is. ${last}`);
    expect(section).toContain("{system} applied the change. Two of Syndra's records disagree");
    // The row's wait_reason decides, never wording: a renamed system, or one the page names another way,
    // still reads as the applied change and never as a refusal.
    const stored = `${first} rey is. ${last}`;
    for (const system of ["TrueNAS", "Makerspace NAS"]) {
      expect(stoppedLine(system, { attempts: 0, wait_reason: BINDING_CONFLICT, last_error: stored })).toBe(`${system} applied the change. ${stored}`);
    }
    // And a refusal whose words happen to open the same way is still a refusal.
    expect(stoppedLine("TrueNAS", { attempts: 0, wait_reason: TARGET_REFUSED, last_error: "TrueNAS applied the change" })).toBe(
      "TrueNAS refused it: TrueNAS applied the change. Syndra stopped sending it.",
    );
  });

  it("quotes the sentences the pages and the card say that are not built from a helper", () => {
    const read = (path: string) => readFileSync(resolve(__dirname, path), "utf8");
    const findings = read("../../components/targets/MergeFindings.tsx");
    const pending = read("../../app/governance/pending/page.tsx");
    for (const [source, text] of [
      [findings, "Finish: stop tracking it"],
      [pending, "Decide it under Waiting on you"],
      [pending, "Waiting on you is on ${system}'s page."],
    ] as const) {
      expect(source, `the page no longer says ${text}`).toContain(text);
      expect(section, `design.md §16 does not carry ${text}`).toContain(text.replace("${system}", "{system}"));
    }
  });

  it("never echoes a reason's code", () => {
    expect(whyNotChecked("some_new_reason")).not.toContain("some_new_reason");
    expect(inventoryNotRead(S, undefined, "some_new_reason")).not.toContain("some_new_reason");
  });

  it("says Stop tracking, the system page's own words for account.release", () => {
    expect(STOP_TRACKING).toBe("Stop tracking");
    expect(readFileSync(resolve(__dirname, "../../components/targets/TargetOverview.tsx"), "utf8")).toContain("Stop tracking this account");
  });

  it("never lets the system's name be said twice before its own reason", () => {
    // No words is the row's wait_reason with no last_error, never a sentence to be matched.
    expect(stoppedLine("TrueNAS", { attempts: 0, wait_reason: TARGET_REFUSED })).toBe(
      "TrueNAS refused it and gave no reason, so Syndra stopped sending it.",
    );
    expect(stoppedLine("TrueNAS", { attempts: 0, wait_reason: TARGET_REFUSED, last_error: "TrueNAS gave no reason." })).toBe(
      "TrueNAS refused it: TrueNAS gave no reason. Syndra stopped sending it.",
    );
    expect(stoppedLine("TrueNAS", { attempts: 0, wait_reason: TARGET_REFUSED, last_error: "It is held." })).toBe(
      "TrueNAS refused it: It is held. Syndra stopped sending it.",
    );
  });
});
