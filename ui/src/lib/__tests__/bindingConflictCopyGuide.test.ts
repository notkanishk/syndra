import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  ADOPT_IT,
  CHECKED_UPDATING,
  CHECK_AGAIN,
  FINISH_AGAIN_NOTE,
  finishLabel,
  CONFLICT_NEXT_FALLBACK,
  OTHER_PERSON,
  STOP_TRACKING,
  adoptConsequence,
  checkAgainConsequence,
  checkStopped,
  closesAtNextCheck,
  conflictDetailFallback,
  holdersLine,
  keepConsequence,
  keepFor,
  stillReported,
  stopTrackingConsequence,
} from "@/components/targets/bindingConflict";
import { inventoryNotRead, whyNotChecked } from "@/lib/format";
import { BINDING_CONFLICT, DECIDE_IT, LOOKING_UP_NEXT, PLAN_STALE, TARGET_REFUSED, decideAt, decideStep, planStaleStep, refusalCause, stoppedLine } from "@/lib/wait-reason";

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
      // 7t-7: the other answers, and Check again.
      ADOPT_IT,
      adoptConsequence(S, "{holder}"),
      adoptConsequence(S, "{holder}", "name_held"),
      CHECKED_UPDATING,
      FINISH_AGAIN_NOTE,
      finishLabel("adopted"),
      finishLabel("kept"),
      keepFor("{name}"),
      keepConsequence(S, "{holder}", "{keeper}", "{other}"),
      CHECK_AGAIN,
      checkAgainConsequence(S),
      stillReported(S),
      checkStopped("{why}"),
      closesAtNextCheck(S),
      decideStep(S),
      DECIDE_IT,
      LOOKING_UP_NEXT,
      planStaleStep(S, true),
      planStaleStep(S, false),
      stoppedLine(S, { ...row, last_error: "out of retries after 5 attempts: x" }).replace("5 tries", "{n} tries"),
      stoppedLine(S, { ...row, wait_reason: TARGET_REFUSED, last_error: "{its words}." }).replace("{its words}.", "{its words}"),
      stoppedLine(S, { ...row, wait_reason: TARGET_REFUSED }),
      stoppedLine(S, { ...row, wait_reason: "refused:not_found" }).replace(refusalCause("refused:not_found"), "{cause in words}"),
      stoppedLine(S, { ...row, wait_reason: PLAN_STALE }),
      stoppedLine(S, { ...row, wait_reason: "halted:x" }),
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

  it("words a binding conflict's row as the refusal it is, and sends it to Waiting on you while the conflict stands", () => {
    // 7t-6: the row is the add-on's refusal or block on the person's binding conflict; its words are the
    // add-on's, after "{system} refused it:", exactly as a target_refused row's.
    for (const system of ["TrueNAS", "Makerspace NAS"]) {
      expect(stoppedLine(system, { attempts: 0, wait_reason: BINDING_CONFLICT, last_error: "It is held." })).toBe(
        stoppedLine(system, { attempts: 0, wait_reason: TARGET_REFUSED, last_error: "It is held." }),
      );
    }
    // Sent to Waiting on you only while the person's binding_conflict still stands on that system.
    const standing = (target: string | undefined, subject: string) => target === "truenas" && subject === "u1";
    expect(decideAt({ wait_reason: BINDING_CONFLICT, target: "truenas", user_id: "u1" }, standing)).toBe("/system/targets/truenas#waiting");
    expect(decideAt({ wait_reason: BINDING_CONFLICT, target: "truenas", user_id: "u2" }, standing)).toBeNull();
    expect(decideAt({ wait_reason: BINDING_CONFLICT, target: "unifi", user_id: "u1" }, standing)).toBeNull();
    expect(decideAt({ wait_reason: TARGET_REFUSED, target: "truenas", user_id: "u1" }, standing)).toBeNull();
    expect(decideAt({ wait_reason: PLAN_STALE, target: "truenas", user_id: "u1" }, standing)).toBeNull();
    // The backend's own sentence for the retired applied-but-unattributed row is gone everywhere.
    const backend = readFileSync(resolve(__dirname, "../../../../backend/internal/services/propagation/addon.go"), "utf8");
    expect(backend).not.toContain("Two of Syndra's records disagree");
    expect(section).not.toContain("Two of Syndra's records disagree");
  });

  it("says a stale plan in plain words, the stored sentence as the backend stores it", () => {
    const backend = readFileSync(resolve(__dirname, "../../../../backend/internal/services/propagation/addon.go"), "utf8");
    // Both stored sentences, as the Go source joins them, and in the guide.
    for (const next of [
      "Syndra sends it again on its own after its next check, or asks you under Waiting on you if the change there touched the same thing.",
      "Syndra's next check puts it back in Pending changes, or asks you under Waiting on you if the change there touched the same thing.",
    ]) {
      expect(backend).toContain(`"Their account changed after this change was approved, so it was not sent. " +\n\t\t"${next}"`);
      expect(section).toContain(`Their account changed after this change was approved, so it was not sent. ${next}`);
    }
    expect(backend).not.toContain("Make the change again");
    const stored = "Their account changed after this change was approved, so it was not sent.";
    expect(backend).not.toContain('"PLAN_STALE: ');
    expect(stoppedLine("TrueNAS", { attempts: 0, wait_reason: PLAN_STALE, last_error: stored })).not.toContain("PLAN_STALE");
  });

  it("quotes the sentences the pages and the card say that are not built from a helper", () => {
    const read = (path: string) => readFileSync(resolve(__dirname, path), "utf8");
    const findings = read("../../components/targets/MergeFindings.tsx");
    for (const [source, text] of [
      [findings, "Finish: stop tracking it"],
    ] as const) {
      expect(source, `the page no longer says ${text}`).toContain(text);
      expect(section, `design.md §16 does not carry ${text}`).toContain(text.replace("${system}", "{system}"));
    }
  });

  it("quotes the add-on's adoptable next steps and its stale refusals as the Go source says them", () => {
    const read = (path: string) => readFileSync(resolve(__dirname, path), "utf8");
    const apply = read("../../../../addons/truenas/apply.go");
    const operations = read("../../../../addons/truenas/operations.go");
    for (const [source, go] of [
      [apply, "Adopt %s if it is theirs. If it is not, stop tracking this person's old account; the next change creates a new one for them."],
      [apply, "Adopt %s if it is theirs. If it is not, stop tracking this person's old account, or rename %s on TrueNAS."],
      [operations, "%s on TrueNAS is no longer the account you were shown. Check again to see it as it is now."],
      [operations, "%s can no longer be adopted for this person: TrueNAS changed since it was checked. Check again to see it as it is now."],
      [apply, "Another person's record points to %s too, so it can't be adopted for either yet. Stop tracking the record of whichever of the two it is not, then adopt %s for the other."],
      [operations, "Moving this person to %s needs the account you were shown, \"+\n\t\t\t\t\t\"so that a name given to another account since is never adopted. Adopt it from the conflict under Waiting on you."],
      [operations, "This person's record on TrueNAS now names a different account \" +\n\t\t\t\t\"from the one Syndra meant, so nothing was released. Check again to see it as it is now."],
    ] as const) {
      expect(source, `the add-on no longer says ${go}`).toContain(go);
      expect(section, `design.md §16 does not carry ${go}`).toContain(go.replaceAll("%s", "{name}").replace(/" ?\+\n\t+"/, ""));
    }
  });

  it("quotes Waiting on you's lede, which no longer names the retired disagreement", () => {
    const overview = readFileSync(resolve(__dirname, "../../components/targets/TargetOverview.tsx"), "utf8");
    const lede = "Things Syndra found and will not settle by itself: an account that differs from what Syndra expects, a change log on ${name} that was altered, or a change ${name} never answered. Each needs a decision from you.";
    expect(overview).toContain(lede);
    expect(section).toContain(lede.replaceAll("${name}", "{system}"));
    expect(overview).not.toContain("two records disagreeing");
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
