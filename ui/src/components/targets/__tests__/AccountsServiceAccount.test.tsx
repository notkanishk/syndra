// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: {
      bound: 2,
      accounts: [
        { subject_id: "svc", username: "netbird", bound_at: "2026-10-15T10:00:00Z" },
        { subject_id: "u1", username: "ada.l", bound_at: "2026-10-15T10:00:00Z" },
      ],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({ data: { accounts: [] }, isLoading: false, error: null }),
}));
vi.mock("@/components/targets/HomeLimit", () => ({
  HomeLine: () => null,
  LimitDialog: () => null,
  limitVerb: () => null,
}));
vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set(["svc"]),
}));

// A-07 §29: a service account stays in the system's accounts table, described as one,
// its name a link to its page.
describe("a connected system's accounts, a service account's row", () => {
  it("says Service account and links the name to the account's page", () => {
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getAllByText("Service account")).toHaveLength(1);
    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href"));
    expect(hrefs).toContain("/system/service-accounts/svc");
    expect(hrefs).toContain("/users/u1");
  });
});
