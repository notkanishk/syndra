// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: {
      bound: 1,
      accounts: [{ subject_id: "zq-9981", username: "ada.l", bound_at: "2026-10-15T10:00:00Z" }],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({ data: { accounts: [] }, isLoading: false, error: null }),
}));
// A name that has not resolved: the avatar must stay blank, not abbreviate the raw id.
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveUser: () => ({ value: undefined, resolved: false }) }),
}));
vi.mock("@/components/targets/HomeLimit", () => ({
  HomeLine: () => null,
  LimitDialog: () => null,
  limitVerb: () => null,
}));

describe("accounts on a connected system, before a name resolves", () => {
  it("never makes initials from a raw id", () => {
    const { container } = render(<PeopleOnTarget target="truenas" />);
    const avatar = container.querySelector(".avatar-fill");
    expect(avatar).not.toBeNull();
    expect(avatar?.textContent).toBe("");
  });
});
