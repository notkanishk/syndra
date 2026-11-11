// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it, vi } from "vitest";

import { truenasName } from "@/lib/__fixtures__/field-schema-mock";
import { TRUENAS_SCHEMA } from "@/lib/__fixtures__/truenas-schema";
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

/** The rows the page loaded: TrueNAS's fields and its name on the roster. */
const read = Object.assign(() => TRUENAS_SCHEMA, { loading: false, error: null, refetch: () => {}, nameOf: truenasName });

const ITEM = {
  field: "group",
  value: "lab_makers",
  reason: "safety review",
  target: "truenas",
  actorId: "op_1",
};

describe("the two densities say the same thing", () => {
  it("names the value, the system and the reason inline", () => {
    render(<WithheldInline items={[ITEM]} schemaOf={read} />);
    expect(screen.getByText("lab_makers")).toBeInTheDocument();
    expect(screen.getByText(/on TrueNAS/)).toBeInTheDocument();
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
    expect(container.textContent).toMatch(/On hold/);
    expect(container.textContent).not.toMatch(/suspend/i);
    expect(container.textContent).not.toMatch(/allowance/i);
  });
});

// A hold on a whole account carries the field "enabled" and the value "true".
// "true" is a switch's position, and the person's page opened its line with it
// ("true on truenas — put on hold by system").
describe("a hold on a whole account", () => {
  const schemaOf = read;
  const ACCOUNT = { field: "enabled", value: "true", reason: "left the team", target: "truenas", actorId: "system" };

  it("names the account, never the boolean, in every form", () => {
    const { container } = render(
      <>
        <Withheld items={[ACCOUNT]} audience="operator" schemaOf={schemaOf} />
        <Withheld items={[ACCOUNT]} audience="member" schemaOf={schemaOf} />
        <WithheldInline items={[ACCOUNT]} schemaOf={schemaOf} />
      </>,
    );
    expect(container.textContent).not.toMatch(/\btrue\b/);
    expect(screen.getAllByText(/Their TrueNAS account/)).toHaveLength(2);
    expect(screen.getByText(/Your TrueNAS account/)).toBeInTheDocument();
    expect(container.textContent).toContain("Their TrueNAS account, put on hold by ");
  });

  it("does not say the system twice", () => {
    const { container } = render(<Withheld items={[ACCOUNT]} audience="operator" schemaOf={schemaOf} />);
    expect(container.textContent).not.toMatch(/on truenas/);
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

describe("a hold is not worded before its system's fields are read", () => {
  const known = read;

  it("draws nothing of the hold while they are being read, and not the word true", () => {
    const item = { ...ITEM, field: "enabled", value: "true" };
    render(<Withheld items={[item]} audience="operator" schemaOf={Object.assign(() => undefined, { ...known, loading: true })} />);
    expect(screen.queryByText("true")).toBeNull();
    expect(screen.queryByText(/hold/)).toBeNull();
  });

  it("shows the failed read with Try again, which asks again", () => {
    const refetch = vi.fn();
    render(<Withheld items={[ITEM]} audience="operator" schemaOf={Object.assign(() => undefined, { ...known, error: new Error("down"), refetch })} />);
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(refetch).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("lab_makers")).toBeNull();
  });

  it("words the hold once they are read", () => {
    render(<Withheld items={[{ ...ITEM, field: "enabled", value: "true" }]} audience="operator" schemaOf={known} />);
    expect(screen.getByText(/Their TrueNAS account/)).toBeInTheDocument();
  });

  // The one-line form on the role page gets the banner's treatment: never the
  // raw "true on truenas" while the fields are read, or after the read failed.
  it("says, inline, that the fields are being read, and not the word true", () => {
    const item = { ...ITEM, field: "enabled", value: "true" };
    render(<WithheldInline items={[item]} schemaOf={Object.assign(() => undefined, { ...known, loading: true })} />);
    expect(screen.queryByText("true")).toBeNull();
    expect(screen.queryByText(/on truenas|on TrueNAS/i)).toBeNull();
    expect(screen.getByText(/still reading the connected system's fields/)).toBeInTheDocument();
  });

  it("says, inline, that the read failed, with the step, and not the value", () => {
    render(<WithheldInline items={[{ ...ITEM, field: "enabled", value: "true" }]} schemaOf={Object.assign(() => undefined, { ...known, error: new Error("down") })} />);
    expect(screen.queryByText("true")).toBeNull();
    expect(screen.getByText(/could not read the connected system's fields\. Reload the page/)).toBeInTheDocument();
  });
});
