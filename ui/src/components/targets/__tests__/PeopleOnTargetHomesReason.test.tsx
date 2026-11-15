// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";

// 7t-3: when the system answered but whose each home folder is could not be
// said (an add-on older than this Syndra, or the verdict not learned), the
// card says why in words (whyNotChecked), never homes shown empty as though
// they were nobody's, and never the reason's code.

const homes = vi.hoisted(() => ({ reason: "addon_outdated" }));

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set<string>(),
}));

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: {
      bound: 1,
      accounts: [{ subject_id: "u1", username: "charlotte.l", account_id: "200001", bound_at: "2026-10-15T10:00:00Z" }],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({
    data: { read: false, usage_read: false, accounts: [], reason: homes.reason },
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

describe("home folders whose owner could not be said", () => {
  it("says an outdated add-on in words", () => {
    homes.reason = "addon_outdated";
    render(<PeopleOnTarget target="truenas" />);

    expect(
      screen.getByText(/Home folders aren't shown\. Its add-on is older than this Syndra.*Update the add-on/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/addon_outdated/)).toBeNull();
  });

  it("says a verdict not learned in words", () => {
    homes.reason = "target_unreachable";
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getByText(/Home folders aren't shown\. It did not answer when Syndra last tried\./)).toBeInTheDocument();
    expect(screen.queryByText(/target_unreachable/)).toBeNull();
  });
});
