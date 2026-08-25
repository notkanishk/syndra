import { describe, expect, it } from "vitest";

import { safeReturnPath } from "@/lib/return-path";

/**
 * A `next` parameter is attacker-composable by construction: it arrives in a
 * URL anybody can write and hand to a member. Unvalidated it is an open
 * redirect — a link that begins on this deployment's real sign-in page and
 * ends on somebody else's, which is the cheapest phishing there is.
 *
 * So these are the attacks, not the happy path.
 */
describe("a return path is an allowlist of shapes", () => {
  it("keeps a real destination", () => {
    expect(safeReturnPath("/storage")).toBe("/storage");
    expect(safeReturnPath("/users/u_1")).toBe("/users/u_1");
  });

  it("refuses an absolute URL", () => {
    expect(safeReturnPath("https://elsewhere.example/harvest")).toBe("/");
    expect(safeReturnPath("http://elsewhere.example")).toBe("/");
  });

  // The one people forget. A browser reads `//host/path` as protocol-relative
  // and follows it off-site, and it passes a naive "starts with /" check.
  it("refuses a protocol-relative URL", () => {
    expect(safeReturnPath("//elsewhere.example/harvest")).toBe("/");
  });

  // Some browsers normalise a backslash to a forward slash while parsing, so
  // `/\evil.example` becomes `//evil.example`.
  it("refuses a backslash the browser may normalise into one", () => {
    expect(safeReturnPath("/\\elsewhere.example")).toBe("/");
  });

  it("refuses a scheme that is not a URL at all", () => {
    expect(safeReturnPath("javascript:alert(1)")).toBe("/");
    expect(safeReturnPath("data:text/html,x")).toBe("/");
  });

  // Signing in and landing back on sign-in is a loop, not a destination.
  it("refuses to return to the sign-in page", () => {
    expect(safeReturnPath("/login")).toBe("/");
    expect(safeReturnPath("/login?next=%2Fstorage")).toBe("/");
  });

  it("falls back for anything absent", () => {
    expect(safeReturnPath(null)).toBe("/");
    expect(safeReturnPath(undefined)).toBe("/");
    expect(safeReturnPath("")).toBe("/");
  });

  it("drops a fragment rather than carrying it into a Location header", () => {
    expect(safeReturnPath("/storage#anchor")).toBe("/storage");
  });

  it("keeps a query, which is where a filtered view lives", () => {
    expect(safeReturnPath("/governance/drift?tab=reconciliation")).toBe(
      "/governance/drift?tab=reconciliation",
    );
  });
});
