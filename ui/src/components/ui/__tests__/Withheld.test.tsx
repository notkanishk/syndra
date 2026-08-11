// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import { Withheld, WithheldInline } from "@/components/ui/Withheld";

/**
 * §6 — the carve-out has to be visible everywhere the role appears.
 *
 * Two densities of one object. The banner is right on a page about one person
 * and wrong repeated forty times down a table, where a warning on most rows is
 * a background colour by the second visit. Both have to say the REASON: an
 * operator who reads "withheld" and no why has had the question moved rather
 * than answered.
 */

const ITEM = {
  field: "group",
  value: "lab_makers",
  reason: "safety review",
  target: "truenas",
  actorId: "op_1",
};

describe("the two densities say the same thing", () => {
  it("names the value, the system and the reason inline", () => {
    render(<WithheldInline items={[ITEM]} />);
    expect(screen.getByText("lab_makers")).toBeInTheDocument();
    expect(screen.getByText(/on truenas/)).toBeInTheDocument();
    expect(screen.getByText(/safety review/)).toBeInTheDocument();
  });

  it("renders nothing at all when nothing is withheld", () => {
    const { container } = render(<WithheldInline items={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  // The word is fixed. "Suspended" and "allowance" are the two it must never
  // become: one is gone from the interface, the other is the API's name for an
  // object that reads as permission granted when it does the opposite.
  it("uses the one word, in both forms", () => {
    const { container } = render(
      <>
        <WithheldInline items={[ITEM]} />
        <Withheld items={[ITEM]} audience="operator" />
      </>,
    );
    expect(container.textContent).toMatch(/Withheld/);
    expect(container.textContent).not.toMatch(/suspend/i);
    expect(container.textContent).not.toMatch(/allowance/i);
  });
});

/**
 * And the holder list actually passes it.
 *
 * A source guard, because this component having no caller is exactly the shape
 * of bug this whole pass has been about — `ListTargetBindings`, `DrainAddon`,
 * `ListCompromisedLogs`. Rendering correctly for a test and for nobody else is
 * the failure mode a rendering test cannot see.
 */
describe("the role holder list is a caller", () => {
  it("passes each member's withheld set to the inline form", () => {
    const page = readFileSync(
      resolve(__dirname, "../../../app/projects/[id]/roles/[key]/page.tsx"),
      "utf8",
    );
    expect(page).toContain("WithheldInline");
    expect(page).toContain("member.withheld");
    // And the count is stated before the list rather than left to be
    // discovered in it.
    expect(page).toContain("withheld_count");
    // And the degradation, which is the half that reads as good news when it
    // is dropped: a zero count from a read that failed says "nobody has one".
    expect(page).toContain("withheld_unavailable");
    expect(page).toMatch(/could not be read/);
  });
});
