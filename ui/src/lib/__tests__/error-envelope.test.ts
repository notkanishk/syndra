import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api-client";
import { describeFailure, outcomeFromError } from "@/lib/outcome";

const raw = "rpc error: code = Unavailable desc = connection refused 192.0.2.7:8080";

describe("an upstream failure on the screen", () => {
  it("shows the envelope's sentence and keeps the reference, never the upstream text", () => {
    const e = new ApiError(502, {
      error: "ZITADEL_ERROR",
      message: "Zitadel gave an answer Syndra could not use.",
      details: { request_id: "ref-test-0001" },
    });
    const o = outcomeFromError(e);
    expect(o.kind).toBe("failed");
    expect(o.requestId).toBe("ref-test-0001");
    expect(o.message).not.toContain("rpc error");
  });

  it("does not show a bare string body at 5xx, and says something at 401", () => {
    expect(new ApiError(502, raw).message).not.toContain("rpc error");
    expect(new ApiError(503, "<html>192.0.2.7</html>").message).toBe("Syndra's server had a problem.");
    expect(new ApiError(401, {}).message).toBe("Your session ended before this ran.");
  });
});

describe("a server fault says what to do and quotes its reference", () => {
  const fault = new ApiError(502, {
    error: "ZITADEL_ERROR",
    message: "Zitadel gave an answer Syndra could not use.",
    details: { request_id: "ref-test-0002" },
  });

  it("a bare line carries the next step and the whole reference", () => {
    const line = describeFailure(fault);
    expect(line).toContain("Zitadel gave an answer Syndra could not use.");
    expect(line).toContain("ask the person who runs the Syndra server");
    expect(line).toContain("ref-test-0002");
  });

  it("a card keeps the message plain and puts the step in its detail", () => {
    const o = outcomeFromError(fault);
    expect(o.message).toBe("Zitadel gave an answer Syndra could not use.");
    expect(o.detail).toContain("quote the reference below");
  });

  it("a refusal is untouched: no next step is invented for a 4xx", () => {
    const refused = new ApiError(409, { error: "STALE", message: "The plan went stale." });
    expect(describeFailure(refused)).toBe("The plan went stale.");
    expect(outcomeFromError(refused).detail).toBeUndefined();
  });

  it("a 429 shows the server's own sentence, as a refusal", () => {
    const limited = new ApiError(429, { error: "RATE_LIMITED", message: "Too many tries. Wait a minute and try again." });
    expect(describeFailure(limited)).toBe("Too many tries. Wait a minute and try again.");
    expect(outcomeFromError(limited).kind).toBe("refused");
  });
});
