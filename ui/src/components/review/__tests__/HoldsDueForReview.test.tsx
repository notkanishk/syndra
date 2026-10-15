// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { HoldsDueForReview } from "@/components/review/HoldsDueForReview";
import type { Hold } from "@/lib/queries/useHolds";

const due: Hold[] = [
  { id: "h1", subject_id: "u1", target: "truenas", field: "enabled", value: "true", direction: "deny", actor_id: "op", reason: "safety review", review_date: "2026-10-01T00:00:00Z", created_at: "2026-09-01T00:00:00Z" },
  { id: "h2", subject_id: "u2", target: "truenas", field: "home_quota", value: String(50 * 1024 ** 3), direction: "set", actor_id: "op", reason: "thesis data", review_date: "2026-10-01T00:00:00Z", created_at: "2026-09-01T00:00:00Z" },
];

vi.mock("@/lib/queries/useHolds", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useHolds")>()),
  useHoldsDueForReview: () => ({ data: { due_for_review: due, count: 2 }, isLoading: false, error: null, refetch: vi.fn() }),
  useLiftHold: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/components/names", () => ({ UserName: ({ id }: { id: string }) => <span>{id}</span> }));

// A person's own home folder limit due for review is listed beside the holds,
// read-only: it is kept or ended on the system's page, its one write place.
describe("holds due", () => {
  it("lists an own limit read-only, with the way to where it's decided", () => {
    render(<HoldsDueForReview />);
    expect(screen.getByRole("button", { name: /Lift the hold on their TrueNAS account/ })).toBeTruthy();
    expect(screen.getByText("their own home folder limit, 50 GB")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Review it on TrueNAS's page" }).getAttribute("href")).toBe("/system/targets/truenas");
    expect(screen.getAllByRole("button")).toHaveLength(1);
    expect(screen.getByText(/It stays at 50 GB until it is kept or ended/)).toBeTruthy();
  });
});
