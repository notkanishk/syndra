// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DormantAccounts } from "@/components/targets/DormantAccounts";
import type { DormantReport } from "@/lib/queries/useDormant";

/**
 * §29 — the only bulk action in the product, and the two things that keep it
 * principled: it can only ever touch accounts that grant nobody anything, and
 * the acknowledgement names what is actually irreversible.
 */

const state: {
  report: DormantReport;
  swept: Array<{ accounts: string[]; elevatedKey: string }>;
} = {
  report: { target: "truenas", state_read_at: new Date().toISOString(), truncated: false, accounts: [] },
  swept: [],
};

vi.mock("@/lib/queries/useDormant", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useDormant")>(
    "@/lib/queries/useDormant",
  );
  return {
    ...actual,
    useDormantAccounts: () => ({
      data: state.report,
      isLoading: false,
      isFetching: false,
      error: null,
      refetch: vi.fn(),
    }),
    useSweepDormant: () => ({
      mutate: (input: { accounts: string[]; elevatedKey: string }) => state.swept.push(input),
      isPending: false,
      error: null,
    }),
  };
});

function renderDormant() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DormantAccounts target="truenas" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.swept = [];
  state.report = {
    target: "truenas",
    state_read_at: new Date().toISOString(),
    truncated: false,
    accounts: [
      {
        account: "former-member",
        subject_id: "u1",
        reason: "membership_ended",
        subject_still_member: false,
        last_seen_at: "2026-05-01T00:00:00Z",
      },
      {
        account: "locked-out",
        subject_id: "u2",
        reason: "role_deleted",
        subject_still_member: true,
        last_seen_at: "2026-07-01T00:00:00Z",
      },
    ],
  };
});

describe("dormant accounts", () => {
  // Same dormancy, opposite action: a former member's account is housekeeping,
  // and a current member's is somebody who may be quietly locked out.
  it("groups by cause and says what each cause means", () => {
    renderDormant();

    expect(screen.getByText(/Their membership ended/)).toBeInTheDocument();
    expect(screen.getByText(/Still a member, and nothing reaches here/)).toBeInTheDocument();
    expect(screen.getByText(/locks the person out rather than tidying up/i)).toBeInTheDocument();
  });

  // The bulk count can never include them, so the one bulk action in the
  // product can only ever touch accounts that grant nobody anything.
  it("makes a still-a-member row unselectable, with the reason in the row", () => {
    renderDormant();

    expect(screen.getByLabelText("Select former-member")).toBeInTheDocument();
    expect(screen.queryByLabelText("Select locked-out")).toBeNull();
    expect(screen.getByText(/they are still a member/i)).toBeInTheDocument();
  });

  it("will not remove anything until the number is ticked and a credential is given", () => {
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));

    const remove = screen.getByRole("button", { name: /remove 1 account/i });
    expect(remove).toBeDisabled();

    fireEvent.click(screen.getByRole("checkbox", { name: /I understand/i }));
    expect(remove).toBeDisabled(); // still no credential

    fireEvent.change(screen.getByLabelText(/credential that may delete/i), {
      target: { value: "k" },
    });
    expect(remove).toBeEnabled();

    fireEvent.click(remove);
    expect(state.swept).toEqual([{ accounts: ["former-member"], elevatedKey: "k" }]);
  });

  // The irreversible part is the data, not the row — and where the size is
  // unknown the sentence says so rather than claiming zero.
  it("names the data rather than the people, and admits an unknown size", () => {
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));

    expect(screen.getByText(/home directories and everything in them/i)).toBeInTheDocument();
    expect(screen.queryByText(/0 bytes/)).toBeNull();
  });

  it("states the size when the target reported one", () => {
    state.report.accounts[0].bytes_held = 41_231_686_042;
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));

    expect(screen.getByText(/38\.4 GB of their files/)).toBeInTheDocument();
  });

  // Everything else on these screens queues. This one does not, and saying so
  // is the difference between an operator waiting for a drain that will never
  // run and one who knows it is already done.
  it("says plainly that this action does not queue", () => {
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));

    expect(screen.getByText(/does not queue/i)).toBeInTheDocument();
  });
});
