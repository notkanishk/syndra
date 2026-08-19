import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api-client";
import {
  OUTCOME_LABEL,
  OUTCOME_TONE,
  PLAN_EFFECT_LABEL,
  outcomeFromError,
  sentence,
  statesNothingChanged,
} from "@/lib/outcome";

describe("the outcome vocabulary", () => {
  // The single most consequential rule in the product's reporting: everything
  // Syndra sends to an add-on is queued and dispatched later, and the response
  // literally reports `succeeded: 0` so a client cannot default it into
  // success. If queued ever wears the accent tone it reads as done, and a UI
  // that says done about a door that still opens is the failure the whole
  // queue design exists to prevent.
  it("never lets queued wear the tone of applied", () => {
    expect(OUTCOME_TONE.queued).not.toBe(OUTCOME_TONE.applied);
    expect(OUTCOME_TONE.queued).toContain("warn");
    expect(OUTCOME_LABEL.queued).toBe("Queued");
  });

  // Past tense for results, future tense for plans. A plan says what would
  // happen; collapsing the two is how "will change" gets reported as though it
  // already had.
  it("keeps the plan's future tense out of the result vocabulary", () => {
    expect(PLAN_EFFECT_LABEL.apply).toBe("Will change");
    expect(Object.keys(OUTCOME_LABEL)).not.toContain("apply");
  });

  it("labels every outcome it can carry", () => {
    for (const kind of ["applied", "queued", "no_change", "refused", "failed"] as const) {
      expect(OUTCOME_LABEL[kind], `${kind} needs a word`).toBeTruthy();
      expect(OUTCOME_TONE[kind], `${kind} needs a tone`).toBeTruthy();
    }
  });
});

describe("an error becomes an outcome", () => {
  // Refused and failed send an operator to different places: a refusal is
  // Syndra declining for a reason somebody wrote and can be acted on, a
  // failure is the machinery not answering and can only be quoted.
  it("reads a 4xx as a refusal, carrying the reason", () => {
    const outcome = outcomeFromError(
      new ApiError(409, { message: "That plan is no longer current.", error: "PLAN_STALE" }),
    );
    expect(outcome.kind).toBe("refused");
    expect(outcome.message).toBe("That plan is no longer current.");
  });

  it("reads a 5xx as a failure", () => {
    expect(outcomeFromError(new ApiError(503, { message: "upstream unavailable" })).kind).toBe(
      "failed",
    );
  });

  it("reads a transport error, which carries no status, as a failure", () => {
    expect(outcomeFromError(new TypeError("Failed to fetch")).kind).toBe("failed");
  });

  it("carries the request id when the backend sent one", () => {
    const outcome = outcomeFromError(
      new ApiError(500, { message: "boom", details: { request_id: "req_9c14e" } }),
    );
    expect(outcome.requestId).toBe("req_9c14e");
  });

  it("says the two things an operator already knows in the product's words", () => {
    expect(outcomeFromError(new ApiError(403, { message: "nope" })).message).toBe(
      "You don't have access to this.",
    );
    expect(outcomeFromError(new ApiError(404, { message: "nope" })).message).toBe(
      "It no longer exists.",
    );
  });

  // The sentence an operator most needs after something went wrong is that the
  // state they were looking at is the state that still holds.
  it("requires 'Nothing was changed.' on exactly the two outcomes that did not change anything", () => {
    expect(statesNothingChanged("refused")).toBe(true);
    expect(statesNothingChanged("failed")).toBe(true);
    expect(statesNothingChanged("applied")).toBe(false);
    expect(statesNothingChanged("queued")).toBe(false);
    expect(statesNothingChanged("no_change")).toBe(false);
  });
});

describe("sentence", () => {
  it("closes a fragment so it does not run into the next one", () => {
    expect(sentence("Nothing answered")).toBe("Nothing answered.");
  });

  it("leaves punctuation the backend already wrote", () => {
    expect(sentence("Is that plan still current?")).toBe("Is that plan still current?");
  });

  it("returns nothing for nothing, rather than a bare full stop", () => {
    expect(sentence("   ")).toBe("");
  });
});
