import { describe, expect, it } from "vitest";

import { loginFailure } from "@/lib/login-error";

/**
 * The closed door has one picture and three things it can say. What it must
 * never do is tell a member the provider was silent when it refused them, or
 * echo a code out of a URL anyone can type.
 */
describe("loginFailure", () => {
  it("says nothing when there is nothing to say", () => {
    expect(loginFailure(undefined)).toBeNull();
    expect(loginFailure("")).toBeNull();
  });

  it("names a refusal as a refusal", () => {
    expect(loginFailure("access_denied")?.head).toBe("Zitadel didn't let you through.");
    expect(loginFailure("login_required")?.head).toBe("Zitadel didn't let you through.");
  });

  it("names a broken round trip as a broken round trip", () => {
    for (const code of ["state_mismatch", "pkce_expired", "pkce_missing", "invalid_token"]) {
      expect(loginFailure(code)?.head, code).toBe("The sign-in didn't complete.");
    }
  });

  it("falls back to silence for anything it cannot classify", () => {
    for (const code of ["misconfigured", "token_exchange_failed", "server_error", "🙃"]) {
      expect(loginFailure(code)?.head, code).toBe("Zitadel didn't answer.");
    }
  });

  it("always says nothing was signed in", () => {
    for (const code of ["access_denied", "state_mismatch", "misconfigured"]) {
      expect(loginFailure(code)?.sub, code).toMatch(/^Nothing was signed in\./);
    }
  });

  it("never echoes the code back to the page", () => {
    const injected = "<img src=x onerror=alert(1)>";
    const failure = loginFailure(injected)!;
    expect(failure.head + failure.sub).not.toContain(injected);
    expect(failure.head + failure.sub).not.toContain("<");
  });
});
