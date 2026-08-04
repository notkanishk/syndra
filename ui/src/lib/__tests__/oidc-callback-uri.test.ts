import { describe, expect, it } from "vitest";

import { buildCallbackUri } from "../oidc";

// Zitadel matches redirect_uri byte-for-byte against the registered value, so
// every character this function emits is load-bearing. These cases are written
// against the shape a reverse proxy actually produces: the request URL carries
// the container's internal scheme and port, and only the x-forwarded-* headers
// know what the browser asked for.
function req(url: string, headers: Record<string, string> = {}): Request {
  return new Request(url, { headers });
}

describe("buildCallbackUri", () => {
  it("drops the container port when the proxy forwards a host without one", () => {
    // The regression. Assigning a bare host onto the request URL via the WHATWG
    // host setter leaves an existing port untouched, so this returned
    // https://syndra.example.org:3000/auth/callback in production and
    // Zitadel refused the login.
    const uri = buildCallbackUri(
      req("http://syndra.example.org:3000/auth/zitadel", {
        "x-forwarded-host": "syndra.example.org",
        "x-forwarded-proto": "https",
      })
    );
    expect(uri).toBe("https://syndra.example.org/auth/callback");
  });

  it("keeps an explicit port when the proxy forwards one", () => {
    const uri = buildCallbackUri(
      req("http://127.0.0.1:3000/auth/zitadel", {
        "x-forwarded-host": "syndra.internal:8443",
        "x-forwarded-proto": "https",
      })
    );
    expect(uri).toBe("https://syndra.internal:8443/auth/callback");
  });

  it("falls back to the Host header when x-forwarded-host is absent", () => {
    const uri = buildCallbackUri(
      req("http://127.0.0.1:3000/auth/zitadel", { host: "syndra.example.org" })
    );
    expect(uri).toBe("http://syndra.example.org/auth/callback");
  });

  it("preserves host:port in direct local development", () => {
    const uri = buildCallbackUri(req("http://localhost:3000/auth/zitadel"));
    expect(uri).toBe("http://localhost:3000/auth/callback");
  });

  it("discards any query and fragment from the initiating request", () => {
    const uri = buildCallbackUri(
      req("http://syndra.example.org:3000/auth/zitadel?next=/users#frag", {
        "x-forwarded-host": "syndra.example.org",
        "x-forwarded-proto": "https",
      })
    );
    expect(uri).toBe("https://syndra.example.org/auth/callback");
  });

  it("is byte-identical across the initiation and callback routes", () => {
    // The two routes build this independently; Zitadel rejects the exchange if
    // they disagree by even a trailing character.
    const headers = {
      "x-forwarded-host": "syndra.example.org",
      "x-forwarded-proto": "https",
    };
    expect(buildCallbackUri(req("http://syndra.example.org:3000/auth/zitadel", headers))).toBe(
      buildCallbackUri(req("http://syndra.example.org:3000/auth/callback?code=x", headers))
    );
  });
});
