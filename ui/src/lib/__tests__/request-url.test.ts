import { describe, expect, it } from "vitest";

import { buildRedirectUrl, isSecureRequest, resolveRequestOrigin } from "../request-url";

// Next's standalone server builds request.url from the address it listens on,
// so in production every request arrives looking like http://localhost:3000/...
// regardless of what the browser asked for. That is the shape these cases use.
function req(url: string, headers: Record<string, string> = {}) {
  return { url, headers: new Headers(headers) };
}

const PROXIED = {
  "x-forwarded-host": "syndra.example.org",
  "x-forwarded-proto": "https",
};

describe("resolveRequestOrigin", () => {
  it("drops the container port the proxy never advertised", () => {
    // The regression. Assigning a bare host over the request URL left :3000 in
    // place, because the WHATWG host setter only replaces the port when the
    // value it is handed carries one.
    expect(resolveRequestOrigin(req("http://localhost:3000/anything", PROXIED))).toBe(
      "https://syndra.example.org"
    );
  });

  it("keeps a port the proxy does forward", () => {
    expect(
      resolveRequestOrigin(
        req("http://localhost:3000/x", { ...PROXIED, "x-forwarded-host": "syndra.internal:8443" })
      )
    ).toBe("https://syndra.internal:8443");
  });

  it("takes the first hop when headers are comma-joined", () => {
    expect(
      resolveRequestOrigin(
        req("http://localhost:3000/x", {
          "x-forwarded-host": "syndra.example.org, internal.proxy",
          "x-forwarded-proto": "https, http",
        })
      )
    ).toBe("https://syndra.example.org");
  });

  it("falls back to Host, then to the request URL", () => {
    expect(resolveRequestOrigin(req("http://localhost:3000/x", { host: "syndra.example.org" }))).toBe(
      "http://syndra.example.org"
    );
    expect(resolveRequestOrigin(req("http://localhost:3000/x"))).toBe("http://localhost:3000");
  });
});

describe("buildRedirectUrl", () => {
  it("builds every redirect target on the browser-facing origin", () => {
    // /login is the middleware target hit on every unauthenticated click.
    for (const path of ["/", "/login", "/auth/callback", "/users"]) {
      expect(buildRedirectUrl(req("http://localhost:3000/whatever", PROXIED), path).toString()).toBe(
        `https://syndra.example.org${path}`
      );
    }
  });

  it("drops the triggering request's query unless one is given", () => {
    expect(
      buildRedirectUrl(req("http://localhost:3000/auth/callback?code=secret", PROXIED), "/").toString()
    ).toBe("https://syndra.example.org/");
  });

  it("carries an explicit query through", () => {
    expect(
      buildRedirectUrl(req("http://localhost:3000/auth/callback", PROXIED), "/login", "?error=pkce_missing").toString()
    ).toBe("https://syndra.example.org/login?error=pkce_missing");
  });

  it("preserves host:port in direct local development", () => {
    expect(buildRedirectUrl(req("http://localhost:3000/auth/zitadel"), "/login").toString()).toBe(
      "http://localhost:3000/login"
    );
  });
});

describe("isSecureRequest", () => {
  it("trusts the forwarded scheme over the terminated connection", () => {
    // TLS ends at the proxy, so the request reaching this process is plain HTTP.
    // Reading its protocol marks production session cookies non-Secure.
    expect(isSecureRequest(req("http://localhost:3000/x", PROXIED))).toBe(true);
  });

  it("is false when the proxy forwarded plain http", () => {
    expect(isSecureRequest(req("http://localhost:3000/x", { "x-forwarded-proto": "http" }))).toBe(false);
  });

  it("takes the first hop of a comma-joined scheme", () => {
    expect(isSecureRequest(req("http://localhost:3000/x", { "x-forwarded-proto": "https, http" }))).toBe(true);
  });

  it("falls back to the request protocol with no proxy in front", () => {
    expect(isSecureRequest(req("https://syndra.example.org/x"))).toBe(true);
    expect(isSecureRequest(req("http://localhost:3000/x"))).toBe(false);
  });
});
