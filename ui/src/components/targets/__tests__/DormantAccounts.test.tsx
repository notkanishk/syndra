// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DormantAccounts } from "@/components/targets/DormantAccounts";
import type { DormantReport } from "@/lib/queries/useDormant";
import type { OneShotSecret } from "@/lib/secret";

/**
 * §29 — the only bulk action in the product, and the two things that keep it
 * principled: it can only ever touch accounts that grant nobody anything, and
 * the acknowledgement names what is actually irreversible.
 */

const state: {
  report: DormantReport;
  swept: Array<{ accounts: string[]; elevatedKey: OneShotSecret }>;
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
      mutate: (input: { accounts: string[]; elevatedKey: OneShotSecret }) => state.swept.push(input),
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
    expect(screen.getByText(/Still a member, but no role gives them access here/)).toBeInTheDocument();
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

    fireEvent.change(screen.getByLabelText(/API key that is allowed to delete/i), {
      target: { value: "k" },
    });
    expect(remove).toBeEnabled();

    fireEvent.click(remove);
    expect(state.swept).toHaveLength(1);
    expect(state.swept[0].accounts).toEqual(["former-member"]);
    expect(state.swept[0].elevatedKey.take()).toBe("k");
  });

  // The delete-capable credential must not outlive its request. TanStack keeps
  // a mutation's variables in the MutationCache, so a plain string here would
  // be the one such key in the deployment, sitting in memory behind a docblock
  // saying it is kept nowhere.
  it("hands the elevated credential over in a box that empties on read", () => {
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));
    fireEvent.click(screen.getByRole("checkbox", { name: /I understand/i }));
    fireEvent.change(screen.getByLabelText(/API key that is allowed to delete/i), {
      target: { value: "super-secret" },
    });
    fireEvent.click(screen.getByRole("button", { name: /remove 1 account/i }));

    const sent = state.swept[0].elevatedKey;
    expect(sent.take()).toBe("super-secret");
    expect(sent.spent).toBe(true);
    // What the cache retains from here on holds nothing, and says so rather
    // than quietly handing back an empty string a retry would send.
    expect(() => sent.take()).toThrow(/already been sent/);
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

    // GiB, not GB. This divided by 1024 and printed a decimal unit name, which
    // by the terabyte is naming a quantity 10% smaller than the one shown.
    expect(screen.getByText(/38\.4 GiB of their files/)).toBeInTheDocument();
  });

  // Everything else on these screens queues. This one does not, and saying so
  // is the difference between an operator waiting for a drain that will never
  // run and one who knows it is already done.
  it("says plainly that this action does not queue", () => {
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));

    expect(screen.getByText(/happens at once, not from Pending changes/i)).toBeInTheDocument();
  });
});
