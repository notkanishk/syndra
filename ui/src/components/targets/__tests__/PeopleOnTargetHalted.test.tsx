// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";

// Staging, add-on stopped: the inventory's halted 503 read nothing, yet the card
// said "Accounts Syndra manages 0 · Nobody yet." Nothing read is no count and no
// empty state; the account list's own line beside it says it was not read.

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set<string>(),
}));

// The inventory the card reads; each test sets it.
const inv = vi.hoisted(() => ({ data: {} as Record<string, unknown> }));

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: inv.data,
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({
    data: { read: false, usage_read: false, accounts: [] },
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

describe("an inventory that halted before reading", () => {
  it("shows no count and no empty state, whatever count the body carries", () => {
    // `bound: 0` is what staging's halted body sent before the backend stopped counting on a halt:
    // the card keys its count on `halted`, the same signal as its empty state, never on the number.
    inv.data = { target: "truenas", bound: 0, current: false, halted: true, reason: "target_unreachable" };
    render(<PeopleOnTarget target="truenas" />);
    expect(count()).toBeNull();
    expect(screen.queryByText("Nobody yet.")).toBeNull();
  });

  it("while a read that found nobody shows its 0 and says so", () => {
    inv.data = { target: "truenas", bound: 0, accounts: [], unmanaged: [], current: true, read_at: "2026-11-21T09:00:00Z" };
    render(<PeopleOnTarget target="truenas" />);
    expect(count()).toHaveTextContent("0");
    expect(screen.getByText("Nobody yet.")).toBeInTheDocument();
  });
});
