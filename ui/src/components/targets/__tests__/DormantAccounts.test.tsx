// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { act } from "@testing-library/react";
import { ApiError } from "@/lib/api-client";
import { gapSettled } from "@/test-utils/leaving";
import { motionInTests } from "@/test-utils/motion";
import { DormantAccounts, sweepResult } from "@/components/targets/DormantAccounts";
import type { DormantReport, SweepResult } from "@/lib/queries/useDormant";
import { readOf } from "@/lib/queries/read";
import type { OneShotSecret } from "@/lib/secret";

/** The wait is said once in its own status region too; these read the line on screen. */
const STATUS = '[role="status"]';

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
  report: { target: "truenas", read: { state: "now", taken_at: new Date().toISOString() }, truncated: false, accounts: [] },
  swept: [],
};

vi.mock("@/lib/queries/useDormant", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useDormant")>(
    "@/lib/queries/useDormant",
  );
  return {
    ...actual,
    useDormantAccounts: () => ({
      data: state.error && !state.keepData ? undefined : readOf(state.report.read, state.report, Date.now()),
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
    read: { state: "now", taken_at: new Date().toISOString() },
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

    expect(screen.getByText("The accounts cannot be brought back. Syndra cannot see how much they held on Truenas.")).toBeInTheDocument();
    expect(screen.queryByText(/0 bytes/)).toBeNull();
    // Generic surface: no promise or claim about files, whatever a system does with them.
    expect(document.body.textContent).not.toMatch(/their files|files stay|left as they are|does not delete/i);
  });

  it("states the size when the target reported one", () => {
    state.report.accounts[0].bytes_held = 41_231_686_042;
    renderDormant();
    fireEvent.click(screen.getByLabelText("Select former-member"));

    // GiB, not GB. This divided by 1024 and printed a decimal unit name, which
    // by the terabyte is naming a quantity 10% smaller than the one shown.
    expect(document.body.textContent).not.toMatch(/their files|files stay|left as they are|does not delete/i);
    expect(screen.getByText("The accounts cannot be brought back. They held 38.4 GiB on Truenas when it was last read.")).toBeInTheDocument();
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
    expect(failed.outcome).toMatchObject({ kind: "failed", next: "Read this list again to see what is left." });
    expect(failed.outcome.message).toContain("former-member: refused. It gained a role since you loaded this list, so it was left alone.");
    const unsure = sweepResult(
      { target: "truenas", removed: 1, outcomes: [{ account: "a", outcome: "removed" }, { account: "b", outcome: "unconfirmed", answer: { status: "unconfirmed", detail: "This may have been applied. Syndra is checking; don't send it again." } }] },
      "TrueNAS",
    );
    expect(unsure.word).toBe("Removed 1 of 2");
    expect(unsure.outcome.message).toContain("b: This may have been applied. Syndra is checking; don't send it again.");
    expect(unsure.outcome.next).toBe("Syndra is checking; don't remove it again.");
  });

  // Design §13.4: a row nothing went out for carries the write's own answer, its why and its step.
  it("says an account nothing went out for in the write's own words, with when to try again", () => {
    const unsent = sweepResult(
      {
        target: "truenas",
        removed: 0,
        outcomes: [{ account: "the-miz", outcome: "nothing_sent", answer: { error: "NOTHING_SENT", message: "Nothing went out. Syndra can't reach TrueNAS's add-on. Try again when TrueNAS answers." } }],
      },
      "TrueNAS",
    );
    expect(unsent.outcome.message).toContain("the-miz: Nothing went out. Syndra can't reach TrueNAS's add-on.");
    expect(unsent.outcome.next).toBe("Try again when TrueNAS answers.");
  });
});

// Both the list and the removal always read the target itself, so inside the
// add-on's cooldown the backend refuses them (429, nothing read, nothing removed).
describe("inside the add-on's cooldown", () => {
  const cooling = (wait: number) => new ApiError(429, { error: "FRESH_READ_COOLDOWN", message: "the server's words", retry_after: wait });

  const pressRemove = (key = "k") => {
    fireEvent.click(screen.getByLabelText("Select former-member"));
    fireEvent.click(screen.getByRole("checkbox", { name: /I understand/i }));
    fireEvent.change(screen.getByLabelText(/API key that is allowed to delete/i), { target: { value: key } });
    fireEvent.click(screen.getByRole("button", { name: /remove 1 account/i }));
  };

  // RFC 9110 §10.2.3: the press waits out retry_after and is sent once more by itself, so one press
  // removes on a live read (owner Q1), with a fresh box for the key.
  it("a removal refused inside the cooldown waits it out and is sent once more by itself", () => {
    vi.useFakeTimers();
    state.sweepError = cooling(2);
    renderDormant();
    pressRemove();
    expect(state.swept).toHaveLength(1);
    expect(screen.getByText(/^Truenas was read moments ago; trying again by itself in 2 s$/, { selector: '[role="status"]' })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Stop waiting" })).toHaveFocus();
    expect(screen.queryByText("Not removed")).toBeNull();
    state.sweepError = undefined;
    state.answer = { removed: 1, outcomes: [{ account: "former-member", outcome: "removed" }] } as SweepResult;
    act(() => void vi.advanceTimersByTime(2000));
    expect(state.swept).toHaveLength(2);
    expect(state.swept[1].accounts).toEqual(["former-member"]);
    expect(state.swept[1].elevatedKey.take()).toBe("k");
    expect(screen.getByText("Removed")).toBeInTheDocument();
    vi.useRealTimers();
  });

  it("retries once only: a second refusal says Not removed with its countdown, and nothing goes again", () => {
    vi.useFakeTimers();
    state.sweepError = cooling(2);
    renderDormant();
    pressRemove();
    act(() => void vi.advanceTimersByTime(2000));
    expect(state.swept).toHaveLength(2);
    expect(screen.getByText("Not removed")).toBeInTheDocument();
    expect(screen.getByText("That system was read moments ago, so nothing was changed.")).toBeInTheDocument();
    expect(screen.getAllByText("You can try again in 2 s.").length).toBeGreaterThan(0);
    // Off until its count ends, the count its linked reason.
    expect(screen.getByRole("button", { name: /remove 1 account/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /remove 1 account/i })).toHaveAccessibleDescription("You can try again in 2 s.");
    act(() => void vi.advanceTimersByTime(120_000));
    expect(state.swept).toHaveLength(2);
    expect(screen.getByText("Press the same button once more.")).toBeInTheDocument();
    // Still armed: the same press, with the same key, is the next step, and it may wait once again.
    expect(screen.getByLabelText(/API key that is allowed to delete/i)).toHaveValue("k");
    fireEvent.click(screen.getByRole("button", { name: /remove 1 account/i }));
    expect(state.swept).toHaveLength(3);
    expect(screen.getByText(/trying again by itself in 2 s/, { selector: '[role="status"]' })).toBeInTheDocument();
    vi.useRealTimers();
  });

  it("Stop waiting cancels the retry and shows the refusal it held, with its step; the wait never passes 60 s", () => {
    vi.useFakeTimers();
    state.sweepError = cooling(3600);
    renderDormant();
    pressRemove();
    expect(screen.getByText(/trying again by itself in 60 s/, { selector: '[role="status"]' })).toBeInTheDocument();
    // Off while it waits, the line its reason (§13).
    expect(screen.getByRole("button", { name: /remove 1 account/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /remove 1 account/i })).toHaveAccessibleDescription(/trying again by itself in 60 s/);
    act(() => void vi.advanceTimersByTime(20_000));
    fireEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
    expect(screen.getByText("Not removed")).toBeInTheDocument();
    expect(screen.getByText("That system was read moments ago, so nothing was changed.")).toBeInTheDocument();
    expect(screen.getAllByText("You can try again in 40 s.").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: /remove 1 account/i })).toBeDisabled();
    act(() => void vi.advanceTimersByTime(120_000));
    expect(state.swept).toHaveLength(1);
    expect(screen.queryByText(/trying again by itself/)).toBeNull();
    expect(screen.getByText("Press the same button once more.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /remove 1 account/i })).toBeEnabled();
    vi.useRealTimers();
  });

  it("leaving the page cancels the wait", () => {
    vi.useFakeTimers();
    state.sweepError = cooling(2);
    const page = renderDormant();
    pressRemove();
    page.unmount();
    act(() => void vi.advanceTimersByTime(60_000));
    expect(state.swept).toHaveLength(1);
    vi.useRealTimers();
  });

  // A list read inside the cooldown has nothing to give (503 TARGET_NOT_READ, reason cooldown): it is none.
  it("a list load says so once, in the read line with its waiting button: no refusal block, no claim that nothing was changed", () => {
    vi.useFakeTimers();
    state.report = { ...state.report, read: { state: "none", reason: "cooldown", retry_after: 2 } };
    renderDormant();
    expect(screen.getByText(/can read again in 2 s/, { ignore: STATUS })).toBeInTheDocument();
    expect(screen.queryByText(/nothing was changed/)).toBeNull();
    expect(screen.queryByText(/You can try again in/)).toBeNull();
    expect(screen.queryByText("Not read")).toBeNull();
    expect(screen.queryByText("This list could not be read")).toBeNull();
    expect(screen.queryByText("Nothing to tidy.")).toBeNull();
    const button = screen.getByRole("button", { name: "Read again" });
    expect(button).toHaveAttribute("aria-disabled", "true");
    act(() => void vi.advanceTimersByTime(2000));
    expect(button).not.toHaveAttribute("aria-disabled");
    fireEvent.click(button);
    expect(state.refetch).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
  });

  it("a Read again that fails for another reason says Not read beside the press and keeps the list", () => {
    state.keepData = true;
    state.error = new ApiError(500, { error: "INTERNAL", message: "the add-on fell over" });
    state.errorAt = Date.now();
    renderDormant();
    expect(screen.getByText("Not read")).toBeInTheDocument();
    expect(screen.getByLabelText("Select former-member")).toBeInTheDocument();
    expect(screen.queryByText("This list could not be read")).toBeNull();
  });

  // Steady under the hand: a Read again inside the cooldown is told only to wait, so the rows read
  // moments ago stay where they are and the strip counts the wait down.
  it("a Read again told to wait keeps the rows on screen, says their real age, and counts down", async () => {
    state.report = { ...state.report, read: { state: "now", taken_at: new Date(Date.now() - 5 * 60_000).toISOString() } };
    const view = renderDormant();
    state.report = { ...state.report, read: { state: "none", reason: "cooldown", retry_after: 30 } };
    view.rerender(tree());
    expect(await screen.findByText(/can read again in 30 s/, { ignore: STATUS })).toBeInTheDocument();
    expect(screen.getByText(/was read 5 min ago; you can read again in 30 s/, { ignore: STATUS })).toBeInTheDocument();
    expect(screen.queryByText(/moments ago/, { ignore: STATUS })).toBeNull();
    expect(screen.getByLabelText("Select former-member")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Read again" })).toHaveAttribute("aria-disabled", "true");
  });

  // Syndra keeps no copy (owner Q1): a Read again answered with nothing to give takes the list away,
  // and the line says why, never an empty list.
  it("a Read again answered with nothing to give says why in place of the list", async () => {
    const view = renderDormant();
    state.report = { ...state.report, read: { state: "none", reason: "addon_unreachable", cause: "no_answer" } };
    view.rerender(tree());
    expect(await screen.findByText(/add-on/, { ignore: STATUS })).toBeInTheDocument();
    expect(screen.queryByLabelText("Select former-member")).toBeNull();
    expect(screen.queryByText("Nothing to tidy.")).toBeNull();
    expect(screen.queryByTestId("card-count")).toBeNull();
  });
});

describe("the sign-in column", () => {
  // One rule for the column (A-10 §6): a 23-day-old sign-in is a date like a 40-day-old one, never
  // "23 days ago" beside "15 Oct".
  it("says every sign-in the same way, capitalised the same way", async () => {
    const ago = (days: number) => new Date(Date.now() - days * 86_400_000).toISOString();
    state.report!.accounts[0].last_seen_at = ago(23);
    state.report!.accounts[1].last_seen_at = ago(40);
    renderDormant();
    const lines = (await screen.findAllByText((_, el) => el?.tagName === "SPAN" && /^Last signed in /.test(el.textContent ?? ""))).filter(
      (el) => el.querySelector("time"),
    );
    expect(lines).toHaveLength(2);
    for (const line of lines) expect(line.textContent).toMatch(/^Last signed in \d{1,2} [A-Z][a-z]{2,4}( \d{4})?$/);
    // The date stays machine-readable (<time datetime>), as <Relative> made it.
    for (const line of lines) expect(line.querySelector("time[datetime]")).not.toBeNull();
  });
});


// D-1 M5: an account that leaves the list fades where it was and its gap closes after, as in every other table.
describe("a removed account leaves by fading and folding", () => {
  beforeEach(() => motionInTests());
  afterEach(() => motionInTests(false));

  it("is marked leaving, and still drawn, before it goes", () => {
    const { rerender, container } = renderDormant();
    state.report = { ...state.report, accounts: state.report.accounts.filter((a) => a.account !== "former-member") };
    rerender(tree());

    const row = container.querySelector("[data-leaving]");
    expect(row).not.toBeNull();
    expect(row!.textContent).toContain("former-member");
    // The group still has its heading while its last row goes, and the count has not moved yet.
    expect(screen.getByText(/Their membership ended/)).toBeInTheDocument();
    expect(screen.getByTestId("card-count").textContent).toBe("2");

    gapSettled(row!);
    expect(screen.queryByText("former-member")).toBeNull();
    expect(screen.queryByText(/Their membership ended/)).toBeNull();
    expect(screen.getByTestId("card-count").textContent).toContain("1");
  });

  // Staging, add-on stopped: a list that could not be read showed "No longer needed 0". A count is
  // observed: unread is no count, never a zero (holding is observed).
  it("shows no count when the list could not be read", () => {
    state.error = new Error("down");
    renderDormant();
    expect(screen.queryByTestId("card-count")).toBeNull();
  });

  it("does not offer a leaving row to be selected", () => {
    const { rerender } = renderDormant();
    state.report = { ...state.report, accounts: state.report.accounts.filter((a) => a.account !== "former-member") };
    rerender(tree());
    // Inert and hidden while it goes: nobody can tick what is already gone.
    expect(screen.queryByRole("checkbox", { name: "Select former-member" })).toBeNull();
    expect(screen.getByLabelText("Select former-member").closest("[data-leaving]")).toHaveAttribute("inert");
  });
});

