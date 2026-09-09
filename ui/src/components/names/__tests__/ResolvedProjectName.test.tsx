// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ResolvedProjectName } from "@/components/names/ProjectName";

/**
 * A person's access view groups roles under a project name the backend
 * already resolved — or fell back on. `upsertRole` falls back to the raw
 * project id on a directory miss, and it used to hand that back
 * indistinguishable from a real name, so a project heading could read
 * "382075332397630470". `project_name_resolved` says which case this is;
 * this component is what has to act on it honestly.
 */
describe("ResolvedProjectName", () => {
  it("renders a resolved name plainly", () => {
    render(<ResolvedProjectName name="Laser Lab" resolved id="p1" />);
    expect(screen.getByText("Laser Lab")).toBeInTheDocument();
    expect(screen.queryByText("Unknown project")).not.toBeInTheDocument();
  });

  it("labels an unresolved name instead of presenting the raw id as one", () => {
    render(<ResolvedProjectName name="382075332397630470" resolved={false} id="382075332397630470" />);
    expect(screen.getByText("Unknown project")).toBeInTheDocument();
    // The id stays on screen — an operator searches by it — just not alone
    // in the slot a name occupies.
    expect(screen.getByText("382075332397630470")).toBeInTheDocument();
  });
});
