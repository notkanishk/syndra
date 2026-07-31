// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  AccessSourceList,
  orderedSources,
  sourceQualifier,
  type RoleReason,
} from "@/components/access/AccessSource";

const direct: RoleReason = { kind: "direct" };
const bundle: RoleReason = { kind: "bundle", bundle_id: "b1", bundle_name: "Lab Tech" };
const mapping: RoleReason = { kind: "mapping", trigger_project: "3D Lab", trigger_role: "operator" };

describe("access source ordering", () => {
  // The order is fixed everywhere so a scanning eye learns one sequence:
  // Direct → Via bundle → Automatic.
  it("always reads Direct, Via bundle, Automatic", () => {
    const shuffled = [mapping, bundle, direct];
    expect(orderedSources(shuffled).map((source) => source.kind)).toEqual([
      "direct",
      "bundle",
      "mapping",
    ]);
  });

  it("drops kinds that are not access sources", () => {
    // views.go also emits project/role/contains/application/rule for the
    // topology graph. Those are a separate namespace and must never render as
    // an access source.
    const mixed = [direct, { kind: "project" }, { kind: "contains" }] as RoleReason[];
    expect(orderedSources(mixed).map((source) => source.kind)).toEqual(["direct"]);
  });

  it("survives an absent reason list", () => {
    expect(orderedSources(undefined)).toEqual([]);
    expect(orderedSources(null)).toEqual([]);
  });

  it("names the bundle or the rule input as the qualifier", () => {
    expect(sourceQualifier(bundle)).toBe("Lab Tech");
    expect(sourceQualifier(mapping)).toBe("3D Lab / operator");
    expect(sourceQualifier(direct)).toBeUndefined();
  });
});

describe("<AccessSourceList/>", () => {
  it("renders the strongest source and collapses the rest behind a count", () => {
    render(<AccessSourceList reasons={[bundle, mapping, direct]} />);

    // Direct is strongest, so it is the chip that gets shown in full.
    expect(screen.getByText("Direct")).toBeInTheDocument();
    expect(screen.queryByText("Via bundle")).toBeNull();
    expect(screen.queryByText("Automatic")).toBeNull();
    // Never a wall of chips.
    expect(screen.getByText("+2 more")).toBeInTheDocument();
  });

  it("shows the qualifier beside a single source", () => {
    render(<AccessSourceList reasons={[bundle]} />);
    expect(screen.getByText("Via bundle")).toBeInTheDocument();
    expect(screen.getByText("Lab Tech")).toBeInTheDocument();
    expect(screen.queryByText(/more$/)).toBeNull();
  });

  it("says so plainly when there is no recorded source", () => {
    render(<AccessSourceList reasons={[]} />);
    expect(screen.getByText("No recorded source")).toBeInTheDocument();
  });
});
