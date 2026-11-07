// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { decidedByBulk } from "@/components/requests/RequestsScreen";
import { resolvedOutcome, revokeOutcome } from "@/components/review/UnexplainedAccess";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ApiError } from "@/lib/api-client";
import {
  NOTHING_TO_DO,
  nextStepOf,
  outcomeFromError,
  outcomeFromExtend,
  REVOKE_SENDS_ITSELF,
  SEND_IT,
  waitsForSend,
} from "@/lib/outcome";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";
import type { AccessRequest } from "@/lib/queries/useRequests";
import { approvalOutcome } from "@/lib/request-decisions";

/**
 * Re-audit, 6 Nov 2026: each outcome below said a next step that wasn't true.
 * Every one is held here by what it now says.
 */
const NEXT = "2099-11-05T08:35:00Z";
const retry = { state: "retrying" as const, try: 2, of: 6, next_try: NEXT };

describe("R1: an adoption whose rules queued rows ends on Send", () => {
  it("is queued, with the Send link, never 'Nothing for you to do'", () => {
    const o = resolvedOutcome(2);
    expect(o).toMatchObject({ kind: "queued", next: SEND_IT });
    expect(waitsForSend(o)).toBe(true);
    expect(nextStepOf(resolvedOutcome(0))).toBe(NOTHING_TO_DO);
  });
});

describe("R5: a drift revoke says where it stands", () => {
  it("went: Removed in Zitadel, not On its way", () => {
    expect(revokeOutcome(undefined, "Trained", { propagation: "applied" })).toMatchObject({ kind: "applied", message: "Removed in Zitadel" });
  });
  it("not taken: says when Syndra sends it again", () => {
    const o = revokeOutcome("halted:retrying", "Trained", { retry });
    expect(o.sendsItself).toBe(true);
    expect(o.message).toMatch(/Syndra sends it again at 14:05 \(try 2 of 6\)/);
  });
  it("still going: On its way", () => {
    expect(revokeOutcome(undefined, "Trained", {})).toBe(REVOKE_SENDS_ITSELF);
  });
});

describe("R14, task 1.2a: an extension sends nothing to Zitadel, so it never waits for Send", () => {
  it("is done, with no Send link, whatever the role", () => {
    const o = outcomeFromExtend(90);
    expect(o.kind).toBe("applied");
    expect(waitsForSend(o)).toBe(false);
  });
});

describe("R7, R17: a failed write says exactly what the server knows", () => {
  const fault = (code: string, status = 500) => new ApiError(status, { error: code, message: "x", details: { request_id: "ref-test-0007" } }, "POST");
  it("an unknown role write (Access › Roles) may have gone through: check Zitadel › Projects", () => {
    expect(outcomeFromError(fault("OUTCOME_UNKNOWN", 502)).partial).toBe("Zitadel › Projects");
  });
  it("a grant change that recorded some roles is partial, never 'Nothing was changed'", () => {
    const o = outcomeFromError(fault("PARTLY_RECORDED"));
    expect(o.partial).toBe("Pending changes and the person's page");
    const { container } = render(<ActionOutcome word="Not done" outcome={o} />);
    expect(container.textContent).not.toMatch(/Nothing was changed/);
  });
  it("a grant Syndra couldn't read changed nothing, and says so", () => {
    const o = outcomeFromError(fault("GRANT_NOT_READ", 502));
    expect(o.partial).toBeUndefined();
    const { container } = render(<ActionOutcome word="Not done" outcome={o} />);
    expect(container.textContent).toMatch(/Nothing was changed/);
    expect(container.textContent).not.toMatch(/may have gone through/);
  });
});

describe("R20: a refusal says 'Nothing was changed.' once", () => {
  it("is not appended to a message that already says it", () => {
    const o = outcomeFromError(new ApiError(409, { error: "CANCELS_QUEUED", message: "This would cancel 2 changes. Nothing was changed." }, "POST"));
    const { container } = render(<ActionOutcome word="Refused" outcome={o} />);
    expect(container.textContent?.match(/Nothing was changed/g)?.length).toBe(1);
  });
});

describe("R12: a rule-given role that waits is said, and ends on Send", () => {
  it("an approval that went says the rule's role waits for you", () => {
    const o = approvalOutcome(undefined, "Ada", "Trained", { rule_waiting: 1 }, [
      "Also gives Bench in Workshop, by the automatic rule from Trained in Workshop.",
    ]);
    expect(o.message).toMatch(/^Approved\. Ada can use it now\. Also gives Bench in Workshop, by the automatic rule from Trained in Workshop\. One role an automatic rule also gives always waits for you/);
    expect(waitsForSend(o)).toBe(true);
  });
  it("is never blamed on the asked-for role", () => {
    const o = approvalOutcome(undefined, "Ada", "Trained", { rule_waiting: 1 });
    expect(o.message).not.toMatch(/This role always waits/);
  });
});

describe("R2: a bulk approval Zitadel didn't take is retried, not 'goes when you send it'", () => {
  it("says when Syndra sends it again", () => {
    const entry = { id: "r1", requester_id: "u1", kind: "grant" } as unknown as AccessRequest;
    const plan = {
      op: "approve",
      applied: true,
      summary: { total: 1, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 1 },
      outcomes: [{ user_id: "u1", name: "Ada", email: "ada@example.org", effect: "queued", detail: "x", wait_reason: "halted:retrying", retry }],
    } as unknown as BulkPlan;
    const [[, decided]] = decidedByBulk([entry], ["r1"], "approved", plan);
    expect(decided.outcome.message).toMatch(/^Approved\. Zitadel didn't answer\. Syndra sends it again at 14:05/);
    expect(waitsForSend(decided.outcome)).toBe(false);
  });
});
