// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { keepNote, keepResult } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

// A settled request to keep a role says what happened to the end from the ends it
// recorded, to whoever asked and to whoever decided; the grant's end today is only
// the fallback for one settled before they were recorded.

function keep(over: Partial<AccessRequest>): AccessRequest {
  return {
    id: "r1",
    requester_id: "u1",
    project_id: "p1",
    role_key: "op",
    justification: "",
    status: "approved",
    kind: "extend",
    created_at: "2026-10-01T09:00:00Z",
    ...over,
  };
}

describe("a settled keep, for the person who asked", () => {
  it("Kept: now ends instead of", () => {
    expect(keepNote(keep({ ended_before: "2099-12-18T12:00:00Z", ends_after: "2100-01-18T12:00:00Z" }), null)).toBe(
      "It now ends 18 Jan 2100 instead of 18 Dec 2099.",
    );
  });

  it("Kept, from a request that recorded neither: the end the grant has now", () => {
    expect(keepNote(keep({}), "2100-01-18T12:00:00Z")).toBe("It now ends 18 Jan 2100.");
    expect(keepNote(keep({}), null)).toBeUndefined();
  });

  it("Not approved: it still ends the day it did, from the request", () => {
    expect(keepNote(keep({ status: "rejected", ended_before: "2099-12-18T12:00:00Z" }), "2099-06-01T00:00:00Z")).toBe(
      "It still ends 18 Dec 2099.",
    );
    expect(keepNote(keep({ status: "rejected", ended_before: "2020-01-01T00:00:00Z" }), null)).toBeUndefined();
  });
});

describe("a settled keep, for the person who decided it", () => {
  it("Extended: now ends, not", () => {
    expect(keepResult(keep({ ended_before: "2099-12-18T12:00:00Z", ends_after: "2100-01-18T12:00:00Z" }))).toBe(
      "Now ends 18 Jan 2100, not 18 Dec 2099.",
    );
    expect(keepResult(keep({ ends_after: "2100-01-18T12:00:00Z" }))).toBe("Now ends 18 Jan 2100.");
  });

  it("Declined: nothing was sent, and it still ends", () => {
    expect(keepResult(keep({ status: "rejected", ended_before: "2099-12-18T12:00:00Z" }))).toBe(
      "Nothing was sent. It still ends 18 Dec 2099.",
    );
  });

  it("an ordinary request, or one that recorded nothing, has no keep line", () => {
    expect(keepResult(keep({ kind: "access", ends_after: "2100-01-18T12:00:00Z" }))).toBeUndefined();
    expect(keepResult(keep({}))).toBeUndefined();
  });
});
