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

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: { target: "truenas", current: false, halted: true, reason: "target_unreachable" },
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

describe("an inventory that halted before reading", () => {
  it("shows no count and no empty state", () => {
    render(<PeopleOnTarget target="truenas" />);
    expect(screen.getByText("Accounts Syndra manages").parentElement!.querySelector("[data-testid=card-count]")).toBeNull();
    expect(screen.queryByText("Nobody yet.")).toBeNull();
  });
});
