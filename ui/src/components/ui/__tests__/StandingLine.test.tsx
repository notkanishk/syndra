// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StandingLine } from "@/components/ui/StandingLine";
import type { Standing } from "@/lib/queries/useTargets";

// The member's card and the operator's list read one standing. An off account whose intent the
// add-on does not know says so once, and each audience gets the step that is theirs.
const unknown: Standing = { usable: false, waiting_on: "none", note: "Switched off on TrueNAS." };

describe("an account that is switched off, intent unknown", () => {
  it("sends a member to makerspace staff, never to a page they cannot use", () => {
    render(<StandingLine standing={unknown} system="TrueNAS" audience="member" />);
    expect(screen.getByText("Ask makerspace staff if you did not expect this.")).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/Check for differences|Mappings/);
  });

  it("sends an operator to Syndra's own comparison", () => {
    render(<StandingLine standing={unknown} system="TrueNAS" audience="operator" />);
    expect(
      screen.getByText("If no one in Syndra asked for this, use Check for differences on the system's Mappings page."),
    ).toBeInTheDocument();
  });
});
