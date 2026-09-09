// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MemberCatalog } from "@/components/member/MemberCatalog";
import type { ProjectCatalog } from "@/lib/types";

function project(overrides: Partial<ProjectCatalog> = {}): ProjectCatalog {
  return {
    id: "p1",
    name: "Fabrication",
    kind: "space",
    description: "",
    roles: [{ key: "laser", label: "Laser cutter", description: "" }],
    ...overrides,
  };
}

vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({ isLoading: false, error: null, data: [project()] }),
}));

/**
 * The member Home's "What else is here" heading used to sit in a row with
 * its caption (`flex items-baseline gap-3`), which shrank both to fit side
 * by side instead of stacking full width like every other section on that
 * page. A row here is the bug, not a detail — assert it stayed stacked.
 */
describe("MemberCatalog header", () => {
  it("stacks the heading and caption instead of squeezing them into a row", () => {
    render(<MemberCatalog heldByProject={new Map()} />);

    const heading = screen.getByText("What else is here");
    const caption = screen.getByText(/Everything the makerspace offers/);

    expect(heading.parentElement).toBe(caption.parentElement);
    expect(heading.parentElement?.className ?? "").not.toMatch(/\bflex\b/);
  });
});
