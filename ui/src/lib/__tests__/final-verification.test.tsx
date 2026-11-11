// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, renderHook, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";

import { Retries } from "@/components/home/Home";
import { removalOutcome } from "@/components/people/RemovalDialog";
import { offlineNext } from "@/components/people/PersonSystems";
import { decidedByBulk } from "@/components/requests/RequestsScreen";
import { revokeOutcome } from "@/components/review/UnexplainedAccess";
import { waitsBeforeLine } from "@/components/states/useWaitsBeforeLine";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { queuedStep, refusedRows, resultMessage, splitQueued } from "@/components/ui/RehearsalDialog";
import { groupPlan, resultOutcome } from "@/lib/plan-groups";
import { describeDrain } from "@/lib/drain-outcome";
import {
  DRAIN_UNKNOWN_NEXT,
  nextStepOf,
  revokeQueued,
  sendsItselfWord,
  succeeded,
  waitsForSend,
  type ActionOutcome as Outcome,
} from "@/lib/outcome";
import type { BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";
import { useAutoRetry } from "@/lib/queries/useIndicators";
import type { AccessRequest } from "@/lib/queries/useRequests";
import { approvalOutcome, withRuleWaits } from "@/lib/request-decisions";
import { refusedOutcome, retireOutcome, retryingOutcome } from "@/lib/retry";
import { refusalCause, waitReasonLine } from "@/lib/wait-reason";

/**
 * Final verification, 6 Nov 2026 (owner: "fix everything the audit finds
 * before shipping more"). N1: a change Zitadel refused outright read as done,
 * as on its way, or as waiting for Send, on every result surface. N3: copy
 * promised "on its own" before the drain switch was read. N4: bulk results
 * blamed the act for a rule's wait.
 */

const NOT_ALLOWED = "refused:not_allowed";
const NOT_FOUND = "refused:not_found";

/** Every word a result shows, as one string. */
const said = (o: Outcome) => [o.message, o.detail, nextStepOf(o)].filter(Boolean).join(" ");

/** The owner's rules for any refusal: says it, says why, ends on a true step, never "done", "on its way" or Send. */
function expectRefusal(o: Outcome, cause = "it no longer has this person, project or role") {
  const text = said(o);
  expect(o.kind).toBe("failed");
  expect(o.recorded).toBe(true);
  expect(text).toContain("Zitadel refused this");
  expect(text).toContain(cause);
  expect(text).toContain("Syndra won't send it again");
  expect(o.sendsItself).toBeFalsy();
  expect(nextStepOf(o)).not.toMatch(/Nothing for you to do|Press Send/);
  expect(text).not.toMatch(/—|!/);
  expect(succeeded(o)).toBe(true); // the act's own button is spent
}

describe("N1: a refusal, with its plain cause, on every result surface", () => {
  it("names each cause in plain words", () => {
    expect(refusalCause(NOT_ALLOWED)).toBe("Syndra's service account isn't allowed to make this change");
    expect(refusalCause(NOT_FOUND)).toBe("it no longer has this person, project or role");
    expect(refusalCause("refused:invalid")).toBe("it said the change isn't valid");
    expect(refusalCause("refused:something_new")).toBe("it gave a reason Syndra can't put in plain words");
    expect(waitReasonLine(NOT_FOUND, "Laser")).toBe("Zitadel refused this: it no longer has this person, project or role");
  });

  it("give: given in Syndra, refused by Zitadel, and given again from their page", () => {
    const o = refusedOutcome("Laser is given to Randy in Syndra", NOT_FOUND);
    expectRefusal(o);
    expect(nextStepOf(o)).toBe("Open their page to see what they hold now, then give it again once the reason is fixed.");
  });

  it("a cause the operator can't fix themselves names who can", () => {
    expect(nextStepOf(refusedOutcome("Approved in Syndra", NOT_ALLOWED))).toMatch(/^Ask whoever runs Zitadel to let Syndra's service account change grants\./);
  });

  it("approve: never 'can use it now'", () => {
    const o = approvalOutcome(NOT_FOUND, "Randy", "Laser");
    expectRefusal(o);
    expect(o.message).not.toMatch(/can use it now/);
  });

  it("approve: a rule's role that waits stays one more step, never turning the refusal into a wait", () => {
    const o = withRuleWaits(refusedOutcome("Approved in Syndra", NOT_FOUND), 1);
    expect(o.kind).toBe("failed");
    expect(nextStepOf(o)).toMatch(/give it again .* Then press Send on Pending changes/);
  });

  it("removal: never 'the access is gone'; the step is Drift", () => {
    const o = removalOutcome({ wait_reason: NOT_FOUND, status: "failed" }, "Laser");
    expectRefusal(o);
    expect(said(o)).not.toMatch(/access is gone/);
    expect(nextStepOf(o)).toMatch(/Drift/);
  });

  it("drift revoke: never 'On its way'", () => {
    const o = revokeOutcome(NOT_FOUND, "Laser", { propagation: "failed" });
    expectRefusal(o);
    expect(said(o)).not.toMatch(/on its way|send on their own/i);
  });

  it("retire, bundle and rule: never 'went to Zitadel'", () => {
    const o = retireOutcome(3, NOT_ALLOWED, undefined, "", true);
    expect(o.kind).toBe("failed");
    expect(o.recorded).toBe(true);
    expect(said(o)).toContain("Zitadel refused this: Syndra's service account isn't allowed to make this change");
    expect(said(o)).not.toMatch(/went to Zitadel|nothing waits for you/);
    expect(nextStepOf(o)).toMatch(/Drift/);
  });

  it("a refusal Syndra recorded never ends on 'Nothing was changed.'", () => {
    render(<ActionOutcome outcome={refusedOutcome("Approved in Syndra", NOT_FOUND)} word="Refused by Zitadel" />);
    expect(screen.getByRole("alert").textContent).not.toMatch(/Nothing was changed/);
  });
});

const row = (o: Partial<BulkOutcome>): BulkOutcome => ({ user_id: "u", name: "Randy", email: "randy@example.org", effect: "applied", detail: "", ...o });
const plan = (op: BulkPlan["op"], outcomes: BulkOutcome[]): BulkPlan => {
  const count = (e: string) => outcomes.filter((o) => o.effect === e).length;
  return {
    op,
    outcomes,
    summary: { total: outcomes.length, apply: 0, succeeded: count("applied"), failed: count("failed"), queued: count("queued"), no_change: 0, blocked: 0 },
  } as unknown as BulkPlan;
};

describe("N1 and N4: bulk rows", () => {
  it("a refused row is said as a refusal, with its own step on the row", () => {
    const p = plan("assign_role", [row({}), row({ user_id: "v", effect: "failed", wait_reason: NOT_FOUND, detail: "Recorded in Syndra. Zitadel refused this: ..." })]);
    expect(refusedRows(p)).toBe(1);
    expect(resultMessage(p, ["person", "people"])).toBe("Applied to 1 person. Zitadel refused this for 1 person; each says why below.");
  });

  it("a bulk approval Zitadel refused reports in the row's seat, not as still waiting", () => {
    const entry = { id: "r1", requester_id: "u1", kind: "new" } as unknown as AccessRequest;
    const decided = decidedByBulk([entry], ["r1"], "approved", plan("approve" as BulkPlan["op"], [row({ user_id: "u1", effect: "failed", wait_reason: NOT_ALLOWED })]));
    expect(decided).toHaveLength(1);
    expect(decided[0][1].word).toBe("Refused by Zitadel");
    expectRefusal(decided[0][1].outcome, "Syndra's service account isn't allowed");
  });

  it("N4: an approval that went is Approved, and a rule's waiting role is said apart, ending on Send", () => {
    const entry = { id: "r1", requester_id: "u1", kind: "new" } as unknown as AccessRequest;
    const [[, decided]] = decidedByBulk([entry], ["r1"], "approved", plan("approve" as BulkPlan["op"], [row({ user_id: "u1", effect: "applied", rule_waiting: 1 })]));
    expect(decided.outcome.message).toMatch(/^Approved\. One role an automatic rule also gives always waits for you/);
    expect(decided.outcome.message).not.toMatch(/It goes when you send it/);
    expect(nextStepOf(decided.outcome)).toBe("Press Send on Pending changes.");
  });

  // A-21 §4.2: the rules' rows are no longer counted apart. A rule's role
  // that waits is the person's step left, so the row sits under the wait fold
  // and the result ends on Send.
  it("N4: a bulk adoption that took effect, with a rule's role waiting, sits under Waiting and ends on Send", () => {
    const p = { ...plan("adopt" as BulkPlan["op"], [row({ effect: "applied", rule_waiting: 1, delivery: "waits_for_you" })]), applied: true };
    expect(groupPlan(p, ["item", "items"])?.map((g) => g.key)).toEqual(["wait"]);
    const r = resultOutcome(p, ["item", "items"]);
    expect(r?.outcome.message).toBe("It waits for you to send. Nothing has reached Zitadel yet.");
    expect(r?.outcome.message).not.toMatch(/automatic rules/);
    expect(waitsForSend(r!.outcome)).toBe(true); // the Send link
  });
});

describe("N3: no promise either way before the drain switch is read", () => {
  const promise = /on its own|on their own|until you send|until someone presses Send|waits for you to press Send/;

  it("useAutoRetry is null under a provider until the indicators answer", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, enabled: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
    const { result, rerender } = renderHook(() => useAutoRetry(), { wrapper });
    expect(result.current).toBeNull();
    client.setQueryData(["governance", "indicators"], { auto_retry: false });
    rerender();
    expect(result.current).toBe(false);
  });

  it("each shared sentence is neutral and ends on Pending changes", () => {
    for (const o of [
      revokeQueued(null),
      retryingOutcome("Approved", undefined),
      retireOutcome(2, "halted:zitadel_offline", undefined, "", null),
      removalOutcome({ wait_reason: "halted:zitadel_offline" }, "Laser", undefined, null),
    ]) {
      expect(said(o)).not.toMatch(promise);
      expect(nextStepOf(o)).toBe(DRAIN_UNKNOWN_NEXT);
      expect(sendsItselfWord(o)).toBe("Not in Zitadel yet");
    }
    expect(waitsBeforeLine("give", null)).not.toMatch(promise);
    expect(offlineNext("TrueNAS", null)).not.toMatch(promise);
    expect(describeDrain({ applied: 0, failed: 0, requeued: 2, errored: 0 } as never, null).detail).not.toMatch(promise);
  });

  it("a bulk revoke before the switch is read is neither on its way nor waiting for Send", () => {
    const p = plan("remove_role", [row({ effect: "queued", wait_reason: "halted:zitadel_offline" })]);
    expect(splitQueued(p, null)).toMatchObject({ waiting: 0, going: 0, unsure: 1 });
    expect(queuedStep(p, null)).toEqual({ sendsItself: true, next: DRAIN_UNKNOWN_NEXT });
  });

  it("Home's retry card names no rule it hasn't read", () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <Retries retrying={2} gaveUp={0} auto={null} />
      </QueryClientProvider>,
    );
    expect(document.body.textContent).not.toMatch(/again on its own and says here|doesn't send changes again/);
    expect(document.body.textContent).toMatch(/whether Syndra sends it again on its own or it waits for you/);
  });
});
