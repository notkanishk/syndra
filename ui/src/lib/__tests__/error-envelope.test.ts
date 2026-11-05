import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api-client";
import { outcomeFromError } from "@/lib/outcome";

const raw = "rpc error: code = Unavailable desc = connection refused 192.0.2.7:8080";

describe("an upstream failure on the screen", () => {
  it("shows the envelope's sentence and keeps the reference, never the upstream text", () => {
    const e = new ApiError(502, {
      error: "ZITADEL_ERROR",
      message: "A service Syndra depends on gave an answer Syndra could not use. Try again in a moment.",
      details: { request_id: "ABCD1234" },
    });
    const o = outcomeFromError(e);
    expect(o.kind).toBe("failed");
    expect(o.requestId).toBe("ABCD1234");
    expect(o.message).not.toContain("rpc error");
  });

  it("does not show a bare string body at 5xx, and says something at 401", () => {
    expect(new ApiError(502, raw).message).not.toContain("rpc error");
    expect(new ApiError(503, "<html>192.0.2.7</html>").message).toBe("Syndra's server had a problem.");
    expect(new ApiError(401, {}).message).toBe("Your session ended before this ran.");
  });
});
