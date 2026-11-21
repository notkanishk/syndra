// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";

// Staging, add-on stopped: the inventory read nothing, yet the card said
// "Accounts Syndra manages 0 · Nobody yet." A read that is `none` (design §13.6)
// is no count and no empty state; the Accounts region says why, once.

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set<string>(),
}));

// The inventory the card reads; each test sets it.
const inv = vi.hoisted(() => ({ data: {} as unknown }));

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: inv.data,
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({
    data: { state: "now", takenAt: "2026-11-21T09:00:00Z", data: { accounts: [] } },
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

const count = () => screen.getByText("Accounts Syndra manages").parentElement!.querySelector("[data-testid=card-count]");

describe("an inventory with nothing to give", () => {
  it("shows no count and no empty state", () => {
    inv.data = { state: "none", why: { reason: "addon_unreachable", cause: "no_answer" } };
    render(<PeopleOnTarget target="truenas" />);
    expect(count()).toBeNull();
    expect(screen.queryByText("Nobody yet.")).toBeNull();
  });

  it("while a read that found nobody shows its 0 and says so", () => {
    inv.data = { state: "now", takenAt: "2026-11-21T09:00:00Z", data: { target: "truenas", bound: 0, accounts: [], unmanaged: [], read: { state: "now" } } };
    render(<PeopleOnTarget target="truenas" />);
    expect(count()).toHaveTextContent("0");
    expect(screen.getByText("Nobody yet.")).toBeInTheDocument();
  });
});
