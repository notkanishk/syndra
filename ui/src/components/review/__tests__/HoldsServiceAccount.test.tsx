// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { HoldsDueForReview } from "@/components/review/HoldsDueForReview";

const due = vi.hoisted(() => [
  { id: "h1", subject_id: "svc", target: "truenas", field: "enabled", value: "true", direction: "deny", actor_id: "op", reason: "", review_date: "2026-10-01T00:00:00Z", created_at: "2026-09-01T00:00:00Z" },
  { id: "h2", subject_id: "u1", target: "truenas", field: "enabled", value: "true", direction: "deny", actor_id: "op", reason: "", review_date: "2026-10-01T00:00:00Z", created_at: "2026-09-01T00:00:00Z" },
]);

vi.mock("@/lib/queries/useHolds", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useHolds")>()),
  useHoldsDueForReview: () => ({
    data: { due_for_review: due, count: due.length },
    dataUpdatedAt: Date.now(),
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
  useLiftHold: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set(["svc"]),
}));

// A-07 §29: a service account stays in Holds, described as one, its name a link to its page.
describe("Holds, a service account's row", () => {
  it("says Service account and links the name to the account's page", () => {
    render(<HoldsDueForReview />);

    expect(screen.getAllByText("Service account")).toHaveLength(1);
    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href"));
    expect(hrefs).toContain("/system/service-accounts/svc");
    expect(hrefs).toContain("/users/u1");
  });
});
