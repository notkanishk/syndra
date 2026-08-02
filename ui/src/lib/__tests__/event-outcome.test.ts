import { describe, expect, it } from "vitest";

import { outcomeOf } from "@/lib/event-outcome";

describe("outcomeOf", () => {
  it("reconciles the two tables' vocabularies into one bucket", () => {
    expect(outcomeOf("processed")).toBe("done"); // webhook events
    expect(outcomeOf("completed")).toBe("done"); // onboarding triggers
    expect(outcomeOf("succeeded")).toBe("done");
  });

  it("buckets a deliberate non-action apart from a success", () => {
    expect(outcomeOf("dropped_enrichment_incomplete")).toBe("dropped");
    expect(outcomeOf("dropped_enrichment_incomplete")).not.toBe(outcomeOf("processed"));
  });

  it("keeps in-progress work out of done", () => {
    expect(outcomeOf("pending")).toBe("waiting");
    expect(outcomeOf("in_flight")).toBe("waiting");
  });

  it("never reports an unrecognised status as done", () => {
    expect(outcomeOf("some_new_backend_status")).toBe("some_new_backend_status");
  });

  it("classifies failure", () => {
    expect(outcomeOf("failed")).toBe("failed");
  });
});
