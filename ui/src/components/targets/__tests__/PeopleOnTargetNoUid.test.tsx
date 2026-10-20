// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: {
      bound: 1,
      accounts: [
        {
          subject_id: "u1",
          username: "ada.l",
          account_uid: 200001,
          bound_at: "2026-10-15T10:00:00Z",
        },
      ],
    },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({ data: { accounts: [] }, isLoading: false, error: null }),
}));
vi.mock("@/components/names", () => ({
  UserName: () => <span>Ada Lovelace</span>,
  UserAvatar: () => null,
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveUser: () => ({ value: { display_name: "Ada Lovelace" }, resolved: true }) }),
}));
vi.mock("@/components/targets/HomeLimit", () => ({
  HomeLine: () => null,
  LimitDialog: () => null,
  limitVerb: () => "Set a limit",
}));

// "id 200001" was the Unix uid, printed as if it said whose account this is. The row's labels are
// the person and the account name (route-system-targets-id); the number is neither.
describe("accounts on a connected system", () => {
  it("names the person and the account, and prints no numeric id", () => {
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText("ada.l")).toBeInTheDocument();
    expect(screen.queryByText(/200001/)).not.toBeInTheDocument();
  });
});
