import { describe, expect, it } from "vitest";

import { nameHint } from "@/components/targets/TargetName";
import type { TargetSummary } from "@/lib/queries/useTargets";

const named = (kind: string) => ({ target: "x", name: "Big mill", name_from: "operator", kind }) as TargetSummary;

describe("nameHint for a system the operator named", () => {
  it("says storage shows on the Network storage page, not on a menu row it has not got", () => {
    expect(nameHint(named("storage"))).toMatch(/^Members see this on their Network storage page\./);
  });

  it("keeps the menu-and-pages sentence for every other kind", () => {
    expect(nameHint(named("door"))).toMatch(/^Members see this on their menu and their pages\./);
  });
});
