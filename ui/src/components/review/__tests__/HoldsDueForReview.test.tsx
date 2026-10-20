// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HoldsDueForReview } from "@/components/review/HoldsDueForReview";
import { ApiError } from "@/lib/api-client";
import type { Hold } from "@/lib/queries/useHolds";

const holds: Hold[] = [
  { id: "h1", subject_id: "u1", target: "truenas", field: "enabled", value: "true", direction: "deny", actor_id: "op", reason: "safety review", review_date: "2026-10-01T00:00:00Z", created_at: "2026-09-01T00:00:00Z" },
  { id: "h2", subject_id: "u2", target: "truenas", field: "home_quota", value: String(50 * 1024 ** 3), direction: "set", actor_id: "op", reason: "thesis data", review_date: "2026-10-01T00:00:00Z", created_at: "2026-09-01T00:00:00Z" },
];

const state = vi.hoisted(() => ({ due: [] as unknown[], lift: vi.fn() }));

vi.mock("@/lib/queries/useHolds", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useHolds")>()),
  useHoldsDueForReview: () => ({
    data: { due_for_review: state.due, count: state.due.length },
    dataUpdatedAt: Date.now(),
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
  useLiftHold: () => ({ mutateAsync: state.lift, isPending: false }),
}));

vi.mock("@/components/names", () => ({ UserName: ({ id }: { id: string }) => <span>{id}</span> }));

beforeEach(() => {
  state.due = holds;
  state.lift.mockReset().mockResolvedValue(undefined);
});

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

  it("reads the list's age under the title, not inside the card", () => {
    render(<HoldsDueForReview />);
    expect(screen.getByText(/This list/)).toBeTruthy();
  });
});

describe("holds due, empty (route-holds, empty)", () => {
  beforeEach(() => {
    state.due = [];
  });

  it("is one flat card with the board's words, not a card inside a card", () => {
    const { container } = render(<HoldsDueForReview />);
    expect(container.querySelectorAll(".card")).toHaveLength(1);
    expect(screen.getByText("No hold is due for review.")).toBeTruthy();
    expect(
      screen.getByText(
        "A hold appears here on the review date the person who placed it chose. Until then it stays in force without asking.",
      ),
    ).toBeTruthy();
    // The page title already says "Holds due" and counts them: no second heading with a 0.
    expect(screen.queryByTestId("card-count")).toBeNull();
    expect(screen.queryByText("Nothing due")).toBeNull();
    // The healthy dot of a resolved state.
    expect(container.querySelector(".bg-healthy")).not.toBeNull();
  });

  it("has no read line, since there is no list to have read", () => {
    render(<HoldsDueForReview />);
    expect(screen.queryByText(/This list/)).toBeNull();
  });
});

describe("a hold's row states", () => {
  const lift = () => screen.getByRole("button", { name: /Lift the hold on their TrueNAS account/ });

  it("Lifted takes the seat the button held, and says when the access returns", async () => {
    render(<HoldsDueForReview />);
    fireEvent.click(lift());

    const seat = await screen.findByRole("status");
    expect(seat.textContent).toContain("Lifted");
    expect(seat.textContent).toContain("returns when the connected system is next brought in line");
    expect(screen.queryByRole("button", { name: /Lift the hold/ })).toBeNull();
    // The row stays until the next read: nothing moves under the hand.
    expect(screen.getByText("safety review")).toBeTruthy();
  });

  it("a failed lift says nothing changed, quotes the reference, and offers Try again", async () => {
    state.lift.mockRejectedValueOnce(new ApiError(500, { message: "boom", details: { request_id: "req_2c71f0b9" } }));
    render(<HoldsDueForReview />);
    fireEvent.click(lift());

    const seat = await screen.findByRole("alert");
    expect(seat.textContent).toContain("Couldn't lift it");
    expect(seat.textContent).toContain("Nothing was changed.");
    expect(seat.textContent).toContain("req_2c71f0b9");

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect((await screen.findByRole("status")).textContent).toContain("Lifted");
    expect(state.lift).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
  });
});
