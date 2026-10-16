import { describe, expect, it } from "vitest";

import { decodePkce, encodePkce } from "@/lib/oidc";

// N21: "then=close" rides in the PKCE cookie like `next`, and nothing else is accepted there.
describe("the PKCE cookie's then", () => {
  const base = { state: "s", verifier: "v", createdAt: 1 };

  it("carries close through the round trip", () => {
    expect(decodePkce(encodePkce({ ...base, then: "close" }))?.then).toBe("close");
    expect(decodePkce(encodePkce(base))?.then).toBeUndefined();
  });

  it("refuses any other value", () => {
    const forged = Buffer.from(JSON.stringify({ ...base, then: "https://elsewhere.example" })).toString("base64url");
    expect(decodePkce(forged)).toBeNull();
  });
});
