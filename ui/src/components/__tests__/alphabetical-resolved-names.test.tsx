// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

/**
 * "Alphabetical, always" for lists whose names are looked up separately: they
 * sort by the name shown, and one not yet resolved sorts by its id, so the
 * list settles at most once.
 */
const NAMES: Record<string, string> = { u1: "Kanishk Singh", u2: "amay Agarwal", u3: "Dhivya R" };

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set<string>(),
}));
vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: {
      bound: 4,
      // Shuffled; "u0-unnamed" has no name yet and sorts by its id.
      accounts: ["u1", "u0-unnamed", "u3", "u2"].map((id) => ({ subject_id: id, username: id, bound_at: "2026-10-15T10:00:00Z" })),
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({ data: { accounts: [] }, isLoading: false, error: null }),
}));
const resolver = {
  resolveUser: (id: string) => ({ value: NAMES[id] ? { display_name: NAMES[id] } : undefined, resolved: Boolean(NAMES[id]) }),
  resolveProject: () => ({ value: undefined, resolved: true }),
  resolveRole: () => ({ value: undefined, resolved: true }),
  resolveBundle: () => ({ value: undefined, resolved: true }),
};
vi.mock("@/lib/queries/useNameResolver", () => ({ useNameResolver: () => resolver }));
vi.mock("@/components/targets/HomeLimit", () => ({ HomeLine: () => null, LimitDialog: () => null, limitVerb: () => null }));

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";
import { Withheld } from "@/components/ui/Withheld";

describe("lists named by lookup are A to Z by the shown name", () => {
  it("accounts on a system: by the person's name, an unresolved one by its id", () => {
    render(<PeopleOnTarget target="truenas" />);
    const text = document.body.textContent ?? "";
    const at = ["amay Agarwal", "Dhivya R", "Kanishk Singh"].map((n) => text.indexOf(n));
    expect(at.every((i) => i >= 0)).toBe(true);
    expect([...at].sort((a, b) => a - b)).toEqual(at);
    // "u0-unnamed" has no name yet, so it sorts as its id: after "Kanishk".
    expect(text.indexOf("Kanishk Singh")).toBeLessThan(text.indexOf("u0-unnamed"));
  });

  it("holds: by what is held, whatever order they came in", () => {
    render(
      <Withheld
        audience="operator"
        items={[
          { field: "share", value: "studio", reason: "r" },
          { field: "share", value: "Archive", reason: "r" },
          { field: "group", value: "lab-2", reason: "r" },
        ]}
      />,
    );
    const rows = screen.getAllByRole("listitem").map((li) => li.textContent ?? "");
    expect(rows.map((t) => ["Archive", "lab-2", "studio"].find((v) => t.includes(v)))).toEqual(["Archive", "lab-2", "studio"]);
  });
});
