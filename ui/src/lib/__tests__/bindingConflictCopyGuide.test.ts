import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import {
  CONFLICT_NEXT_FALLBACK,
  OTHER_PERSON,
  STOP_TRACKING,
  adoptInstead,
  conflictDetailFallback,
  holdersLine,
  stopTrackingConsequence,
} from "@/components/targets/bindingConflict";
import { inventoryNotRead } from "@/lib/format";
import { targetRefusedLine } from "@/lib/wait-reason";

/**
 * The copy guide's section 16 rows for the binding conflict (addon-contract 7t-4)
 * and the sentences the renderers say are the same words. If one moves, the
 * other fails, so the guide is never last month's copy.
 */
const guide = readFileSync(resolve(__dirname, "../../../../openspec/changes/plain-language-copy/design.md"), "utf8");
const section = guide.slice(guide.indexOf("## 16. The system page, after the visual check"));
const S = "{system}";

describe("the binding conflict's copy guide rows", () => {
  it("quote every sentence the page says around the add-on's words", () => {
    const said = [
      conflictDetailFallback(S),
      CONFLICT_NEXT_FALLBACK,
      `${OTHER_PERSON} {name}.`,
      holdersLine(S, ["{a}", "{b}"]),
      STOP_TRACKING,
      stopTrackingConsequence(S),
      adoptInstead("{holder}"),
      targetRefusedLine(S, "{its words}").replace("{its words}.", "{its words}"),
      targetRefusedLine(S, `${S} gave no reason.`),
      inventoryNotRead(undefined, undefined).split(" It could not")[0],
    ];
    const missing = said.filter((line) => !section.includes(line));
    expect(missing, "design.md §16 does not carry these words").toEqual([]);
  });

  it("says Stop tracking, the system page's own words for account.release", () => {
    expect(STOP_TRACKING).toBe("Stop tracking");
    expect(readFileSync(resolve(__dirname, "../../components/targets/TargetOverview.tsx"), "utf8")).toContain("Stop tracking this account");
  });

  it("never lets the system's name be said twice before its own reason", () => {
    expect(targetRefusedLine("TrueNAS", "TrueNAS gave no reason.")).toBe("TrueNAS refused it and gave no reason, so Syndra stopped sending it.");
    expect(targetRefusedLine("TrueNAS", "It is held.")).toBe("TrueNAS refused it: It is held. Syndra stopped sending it.");
  });
});
