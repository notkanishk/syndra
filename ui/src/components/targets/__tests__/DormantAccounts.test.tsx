// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { act } from "@testing-library/react";
import { ApiError } from "@/lib/api-client";
import { DormantAccounts, sweepResult } from "@/components/targets/DormantAccounts";
import type { DormantReport, SweepResult } from "@/lib/queries/useDormant";
import type { OneShotSecret } from "@/lib/secret";

/**
 * §29 — the only bulk action in the product, and the two things that keep it
 * principled: it can only ever touch accounts that grant nobody anything, and
 * the acknowledgement names what is actually irreversible.
 */

const state: {
  report: DormantReport;
  swept: Array<{ accounts: string[]; elevatedKey: OneShotSecret }>;
  answer?: SweepResult;
  error?: unknown;
  errorAt?: number;
  /** A refetch that failed with the list still on hand, as react-query keeps it. */
  keepData?: boolean;
  sweepError?: unknown;
  refetch: ReturnType<typeof vi.fn>;
} = {
  refetch: vi.fn(),
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
      data: state.error && !state.keepData ? undefined : state.report,
      isLoading: false,
      isFetching: false,
      error: state.error ?? null,
      errorUpdatedAt: state.errorAt,
      refetch: state.refetch,
    }),
    useSweepDormant: () => ({
      mutate: (
        input: { accounts: string[]; elevatedKey: OneShotSecret },
        opts?: { onSuccess?: (r: SweepResult) => void; onError?: (e: unknown) => void },
      ) => {
        state.swept.push(input);
        if (state.sweepError) opts?.onError?.(state.sweepError);
        if (state.answer) opts?.onSuccess?.(state.answer);
      },
      isPending: false,
      error: null,
    }),
  };
});

const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
// A new element each time, so a rerender really draws again (an identical one is bailed out of).
const tree = () => (
  <QueryClientProvider client={client}>
    <DormantAccounts target="truenas" />
  </QueryClientProvider>
);
const renderDormant = () => render(tree());

beforeEach(() => {
  state.swept = [];
  state.answer = undefined;
  state.error = undefined;
  state.errorAt = undefined;
  state.keepData = undefined;
  state.sweepError = undefined;
  state.refetch = vi.fn();
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
  it("makes a still-a-member row unselectable, with the reason said once in its heading", () => {
    renderDormant();

    expect(screen.getByLabelText("Select former-member")).toBeInTheDocument();
    expect(screen.getByText("locked-out")).toBeInTheDocument();
    expect(screen.queryByLabelText("Select locked-out")).toBeNull();
    // Not repeated under every row.
    expect(screen.queryByText(/they are still a member/i)).toBeNull();
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
    // Re-queried: the button drops its "check the box"/"paste a key" reason
    // the instant it becomes pressable, and Button.tsx renders that as a
    // different element (bare `<button>` instead of a `<span>` wrapping one),
    // which replaces the DOM node rather than just its attributes.
    const armed = screen.getByRole("button", { name: /remove 1 account/i });
    expect(armed).toBeEnabled();

    fireEvent.click(armed);
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

// A-22: the sweep's result is one result in one slot, in the act's own words,
// ending on a step; there is no "Dismiss this result".
describe("the sweep result", () => {
  const pressRemove = () => {
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));
    fireEvent.click(screen.getByRole("checkbox", { name: /I understand/i }));
    fireEvent.change(screen.getByLabelText(/API key that is allowed to delete/i), { target: { value: "k" } });
    fireEvent.click(screen.getByRole("button", { name: /remove 1 account/i }));
  };

  it("says Removed, and that there is nothing left to do", () => {
    state.answer = { target: "truenas", removed: 1, outcomes: [{ account: "former-member", outcome: "removed" }] };
    pressRemove();
    expect(screen.getByText("Removed")).toBeInTheDocument();
    expect(screen.getByText("1 account removed.")).toBeInTheDocument();
    expect(screen.getByText("Nothing for you to do.")).toBeInTheDocument();
    expect(screen.queryByText(/dismiss this result/i)).toBeNull();
  });

  it("says Not removed with the account's own sentence and a step", () => {
    const failed = sweepResult({ target: "truenas", removed: 0, outcomes: [{ account: "former-member", outcome: "refused" }] }, "TrueNAS");
    expect(failed.word).toBe("Not removed");
    expect(failed.outcome).toMatchObject({ kind: "failed", next: "Refresh this list to see what is left." });
    expect(failed.outcome.message).toContain("former-member: refused. It gained a role since you loaded this list, so it was left alone.");
    const unsure = sweepResult({ target: "truenas", removed: 1, outcomes: [{ account: "a", outcome: "removed" }, { account: "b", outcome: "indeterminate" }] }, "TrueNAS");
    expect(unsure.word).toBe("Removed 1 of 2");
    expect(unsure.outcome.next).toBe("Check TrueNAS before you try again.");
  });
});

// Both the list and the removal always read the target itself, so inside the
// add-on's cooldown the backend refuses them (429, nothing read, nothing removed).
describe("inside the add-on's cooldown", () => {
  const cooling = (wait: number) => new ApiError(429, { error: "FRESH_READ_COOLDOWN", message: "the server's words", retry_after: wait });

  it("a removal says Not removed with the countdown, keeps the selection and the key, and is pressed again the same way", () => {
    vi.useFakeTimers();
    state.sweepError = cooling(2);
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));
    fireEvent.click(screen.getByRole("checkbox", { name: /I understand/i }));
    fireEvent.change(screen.getByLabelText(/API key that is allowed to delete/i), { target: { value: "k" } });
    fireEvent.click(screen.getByRole("button", { name: /remove 1 account/i }));
    expect(screen.getByText("Not removed")).toBeInTheDocument();
    expect(screen.getByText("That system was read moments ago, so nothing was changed.")).toBeInTheDocument();
    expect(screen.getByText("You can try again in 2 s.")).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(2000));
    expect(screen.getByText("Press the same button once more.")).toBeInTheDocument();
    // Still armed: the same press, with the same key, is the next step.
    expect(screen.getByLabelText(/API key that is allowed to delete/i)).toHaveValue("k");
    expect(screen.getByRole("button", { name: /remove 1 account/i })).toBeEnabled();
    vi.useRealTimers();
  });

  it("a list load says so once, in the freshness line with its waiting button: no refusal block, no claim that nothing was changed", () => {
    vi.useFakeTimers();
    state.error = cooling(2);
    renderDormant();
    expect(screen.getByText(/can read again in 2 s/)).toBeInTheDocument();
    expect(screen.queryByText(/nothing was changed/)).toBeNull();
    expect(screen.queryByText(/You can try again in/)).toBeNull();
    expect(screen.queryByText("Not read")).toBeNull();
    expect(screen.queryByText("This list could not be read")).toBeNull();
    expect(screen.queryByText("Nothing to tidy.")).toBeNull();
    const button = screen.getByRole("button", { name: "Read again" });
    expect(button).toBeDisabled();
    act(() => void vi.advanceTimersByTime(2000));
    expect(button).toBeEnabled();
    fireEvent.click(button);
    expect(state.refetch).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
  });

  // Somebody else's wait is not the reader's doing, and only their own press into it is a warning.
  it("draws a cooldown the load ran into neutral, and amber only once the reader's own press meets it", () => {
    vi.useFakeTimers();
    state.error = cooling(2);
    state.errorAt = Date.now();
    const view = renderDormant();
    const dot = () => document.querySelector("[data-freshness-dot]") as HTMLElement;
    expect(dot().className).toContain("bg-faint");
    expect(dot().className).not.toContain("warn");
    act(() => void vi.advanceTimersByTime(2000));
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    state.error = cooling(30);
    state.errorAt = Date.now();
    view.rerender(tree());
        expect(dot().className).toContain("bg-warn");
    vi.useRealTimers();
  });

  it("keeps the list and its selection when a refetch is refused while it is on hand", async () => {
    const view = renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));
    expect(screen.getByLabelText("Select former-member")).toBeChecked();
    state.keepData = true;
    state.error = cooling(30);
    state.errorAt = Date.now();
    view.rerender(tree());
    expect(await screen.findByText(/can read again in/)).toBeInTheDocument();
    expect(screen.getByLabelText("Select former-member")).toBeChecked();
    expect(screen.queryByText("This list could not be read")).toBeNull();
  });
});
