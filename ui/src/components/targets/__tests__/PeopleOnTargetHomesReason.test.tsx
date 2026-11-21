// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { asRead } from "@/test-utils/wire";
import { describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";

// 7t-3, design §13.6: when the accounts were read and their home folders were
// not, the card says why in §13.1's words, never homes shown empty as though
// they were nobody's, and never the reason's code.

const homes = vi.hoisted(() => ({ read: { state: "none", reason: "addon_outdated" } as Record<string, unknown> }));

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set<string>(),
}));

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: asRead({
      bound: 1,
      accounts: [{ subject_id: "u1", username: "charlotte.l", account_id: "200001", bound_at: "2026-10-15T10:00:00Z" }],
    }),
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({
    data: asRead({ read: homes.read as never, accounts: [] }),
    isLoading: false,
    error: null,
  }),
}));
vi.mock("@/components/names", () => ({
  UserName: () => <span>Charlotte Flair</span>,
  UserAvatar: () => null,
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveUser: () => ({ value: { display_name: "Charlotte Flair" }, resolved: true }) }),
}));
vi.mock("@/components/kinds/storage/HomeLimit", async (original) => ({
  ...(await original<typeof import("@/components/kinds/storage/HomeLimit")>()),
  HomeLine: () => null,
  LimitDialog: () => null,
  limitVerb: () => undefined,
}));
vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);

describe("home folders that were not read", () => {
  it("says an outdated add-on in words", () => {
    homes.read = { state: "none", reason: "addon_outdated" };
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getByText("TrueNAS's add-on doesn't offer this. Update the add-on.")).toBeInTheDocument();
    expect(screen.queryByText(/addon_outdated/)).toBeNull();
  });

  it("says the add-on not answering in words, with what to check", () => {
    homes.read = { state: "none", reason: "addon_unreachable", cause: "no_answer" };
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getByText(/Syndra can't reach TrueNAS's add-on\. Check that its container is running\./)).toBeInTheDocument();
    expect(screen.queryByText(/addon_unreachable/)).toBeNull();
  });
});
