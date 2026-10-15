// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PlanReview } from "@/components/ui/PlanReview";

vi.mock("@/components/names/UserName", () => ({ UserName: () => <>Kanishk Singh</> }));

// #32: a bulk removal never takes access from the requester; the review says so.
describe("a plan that leaves the requester out", () => {
  it("names them, says why, and points to their own page", () => {
    render(
      <PlanReview
        plan={{
          op: "remove_role",
          applied: false,
          outcomes: [],
          excluded: [{ user_id: "u_me", reason: "self" }],
          summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
        }}
      />,
    );
    expect(screen.getByText("Kanishk Singh")).toBeInTheDocument();
    expect(screen.getByText(/Left out · this is you\. Change your own access from/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "your page" })).toHaveAttribute("href", "/users/u_me");
  });
});
