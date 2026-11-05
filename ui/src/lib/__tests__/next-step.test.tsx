// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { RetryStatus } from "@/components/ui/RetryStatus";
import { ApiError } from "@/lib/api-client";
import {
  NOTHING_TO_DO,
  nextStepOf,
  outcomeFromError,
  type ActionOutcome as Outcome,
  type OutcomeKind,
} from "@/lib/outcome";
import { retireOutcome, retryingOutcome, retryNextStep, type RetryState, type RetryStateName } from "@/lib/retry";

/**
 * Owner, 5 Nov 2026: "along with clear communication, there should be clear
 * next steps or lack thereof." Every outcome and every retry state ends on a
 * next step or an explicit "nothing to do", by construction in the shared
 * parts, so no screen can forget it.
 */
const KINDS: OutcomeKind[] = ["applied", "queued", "no_change", "refused", "failed"];
const STATES: RetryStateName[] = ["retrying", "waiting"];
const NEXT = "2099-11-05T08:35:00Z";

describe("every outcome kind maps to a next step", () => {
  for (const kind of KINDS) {
    for (const sendsItself of [false, true]) {
      for (const partial of [undefined, "Pending changes"]) {
        it(`${kind}${sendsItself ? ", sends itself" : ""}${partial ? ", partial" : ""}`, () => {
          const next = nextStepOf({ kind, message: "x", sendsItself, partial });
          expect(next.trim()).not.toBe("");
          expect(next).toMatch(/[.!?]$/);
        });
      }
    }
  }

  it("says there is nothing to do when nothing is left", () => {
    expect(nextStepOf({ kind: "applied", message: "Given" })).toBe(NOTHING_TO_DO);
    expect(nextStepOf({ kind: "queued", message: "On its way", sendsItself: true })).toMatch(/^Nothing for you to do\./);
  });

  it("names the step for a refusal by what refused it", () => {
    expect(nextStepOf(outcomeFromError(new ApiError(403, { error: "FORBIDDEN", message: "No." }, "POST")))).toMatch(/Ask someone/);
    expect(nextStepOf(outcomeFromError(new ApiError(404, { error: "NOT_FOUND", message: "Gone." }, "POST")))).toMatch(/Reload the page/);
    expect(nextStepOf(outcomeFromError(new ApiError(500, { error: "DB_ERROR", message: "x" }, "POST")))).toMatch(/Try again in a minute/);
  });

  it("a retrying change says when, and that there is nothing to do", () => {
    const retry: RetryState = { state: "retrying", try: 2, of: 6, next_try: NEXT };
    expect(nextStepOf(retryingOutcome("Given", retry))).toMatch(/^Nothing for you to do\. Syndra sends it again at .* and says on Home if it stops sending it\.$/);
    expect(nextStepOf(retireOutcome(2, "halted:retrying", retry))).toMatch(/sends them again at/);
  });
});

describe("every retry state maps to a next step", () => {
  for (const state of STATES) {
    for (const own of [false, true]) {
      it(`${state}${own ? ", a member's own" : ""}`, () => {
        const next = retryNextStep({ state, try: 2, of: 6, next_try: NEXT }, { own });
        expect(next.trim()).not.toBe("");
        expect(next).toMatch(/[.!?]$/);
      });
    }
  }
});

describe("the shared parts draw it", () => {
  for (const kind of KINDS) {
    for (const placement of ["block", "inline", "seat"] as const) {
      it(`ActionOutcome ${placement}, ${kind}`, () => {
        const outcome: Outcome = { kind, message: "Something happened" };
        const { container, getAllByText } = render(<ActionOutcome outcome={outcome} placement={placement} word="Word" />);
        if (kind === "queued") {
          // The step is the link to Pending changes.
          expect(getAllByText(/Waiting for you to send/).length).toBeGreaterThan(0);
        } else {
          expect(container.textContent).toContain(nextStepOf(outcome));
        }
      });
    }
  }

  it("says a step once, not twice, when the detail already holds it", () => {
    const outcome = outcomeFromError(new ApiError(500, { error: "DB_ERROR", message: "x", details: { request_id: "ABC" } }, "POST"));
    const { container } = render(<ActionOutcome outcome={outcome} word="Failed" />);
    const step = nextStepOf(outcome);
    expect(container.textContent?.split(step).length).toBe(2);
  });

  for (const state of STATES) {
    it(`RetryStatus, ${state}`, () => {
      const retry: RetryState = { state, try: 2, of: 6, next_try: NEXT };
      const { container } = render(<RetryStatus retry={retry} />);
      expect(container.textContent).toContain(retryNextStep(retry));
    });
  }
});
