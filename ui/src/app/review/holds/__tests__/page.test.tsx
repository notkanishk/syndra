// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import HoldsDuePage from "@/app/review/holds/page";

const state = vi.hoisted(() => ({ due: [] as unknown[] }));

vi.mock("@/lib/queries/useHolds", () => ({
  useHoldsDueForReview: () => ({ data: { due_for_review: state.due, count: state.due.length }, isLoading: false }),
}));
vi.mock("@/components/review/HoldsDueForReview", () => ({ HoldsDueForReview: () => <div /> }));

const hold = (id: string, review_date: string) => ({ id, review_date });

beforeEach(() => {
  state.due = [];
});

describe("Holds due, the title block", () => {
  it("counts the holds and dates the oldest, under the title", () => {
    state.due = [hold("a", "2026-10-01T12:00:00Z"), hold("b", "2026-09-18T12:00:00Z"), hold("c", "2026-10-04T12:00:00Z")];
    render(<HoldsDuePage />);
    expect(screen.getByText("3 holds")).toBeTruthy();
    expect(screen.getByText("the oldest due 18 Sept")).toBeTruthy();
  });

  it("says one hold in the singular", () => {
    state.due = [hold("a", "2026-10-01T12:00:00Z")];
    render(<HoldsDuePage />);
    expect(screen.getByText("1 hold")).toBeTruthy();
  });

  it("has nothing to count when nothing is due", () => {
    render(<HoldsDuePage />);
    expect(screen.queryByText(/holds?$/)).toBeNull();
    expect(screen.queryByText(/the oldest due/)).toBeNull();
  });

  it("explains lifting and how to keep a hold with a later date", () => {
    render(<HoldsDuePage />);
    expect(screen.getByText(/being listed here ends nothing/)).toBeTruthy();
    expect(screen.getByText(/To keep a hold with a later date, lift it and place a new one from the person's page\./)).toBeTruthy();
  });
});
