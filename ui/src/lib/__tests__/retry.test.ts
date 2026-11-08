import { describe, expect, it } from "vitest";

import { removalOutcome } from "@/components/people/RemovalDialog";
import { memberRetryLine, retireOutcome, retryingOutcome, retryLine, type RetryState } from "@/lib/retry";

// One retry model (owner, 5 Nov 2026): a Zitadel change and a storage password
// arrive in the same shape and are said with the same words.
const NEXT = "2099-11-05T08:35:00Z"; // 14:05 in IST, whatever the browser's zone

describe("the one retry model's words", () => {
  it("say the same thing for Zitadel and a connected system, only the names differ", () => {
    const zitadel: RetryState = { state: "retrying", try: 2, of: 6, next_try: NEXT };
    const storage: RetryState = { state: "retrying", try: 2, of: 2, next_try: NEXT };
    expect(retryLine(zitadel)).toBe("Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6).");
    expect(retryLine(storage, "TrueNAS")).toBe("TrueNAS didn't answer. Syndra sends it again at 14:05 (try 2 of 2).");
  });

  it("say when a retry has ended, and never that it worked", () => {
    expect(retryLine({ state: "waiting", try: 2, of: 2 }, "TrueNAS")).toBe(
      "TrueNAS didn't answer after 2 tries, so Syndra won't send it again on its own.",
    );
    // With the background drain off a retried row waits after its one try (re-audit).
    expect(retryLine({ state: "waiting", try: 1, of: 6 })).toBe(
      "Zitadel didn't answer after one try, so Syndra won't send it again on its own.",
    );
    expect(retryingOutcome("Given", { state: "waiting", try: 1, of: 6 })).toMatchObject({
      kind: "queued",
      sendsItself: undefined,
      next: "Press Send on Pending changes to try again.",
    });
  });

  it("tell a person about their own password without ever saying done", () => {
    expect(memberRetryLine({ state: "retrying", try: 2, of: 2, next_try: NEXT })).toBe(
      "Not confirmed yet. Syndra is trying again at 14:05. Keep this page open.",
    );
  });

  it("say 'in a moment' once the next try has passed, not a time already gone", () => {
    expect(retryLine({ state: "retrying", try: 3, of: 6, next_try: "2000-01-01T00:00:00Z" })).toMatch(
      /sends it again in a moment \(try 3 of 6\)/,
    );
  });

  it("an action whose send is retrying is queued and goes by itself", () => {
    expect(retryingOutcome("Approved", undefined)).toMatchObject({ kind: "queued", sendsItself: true });
  });
});

describe("retiring a bundle or a rule (W1, W3)", () => {
  it("all went: the access it gave was revoked", () => {
    expect(retireOutcome(3, undefined)).toMatchObject({ kind: "applied", message: "3 changes went to Zitadel." });
  });
  it("a role that waits: the revocations wait for you", () => {
    expect(retireOutcome(2, "role_waits").detail).toMatch(/wait under Pending changes until you send them/);
  });
  it("Zitadel didn't take them: sent again on their own, never 'went to Zitadel'", () => {
    const out = retireOutcome(2, "halted:retrying", { state: "retrying", try: 2, of: 6, next_try: NEXT });
    expect(out).toMatchObject({ kind: "queued", sendsItself: true });
    expect(out.message).toBe("Not in Zitadel yet. Zitadel didn't answer. Syndra sends them again at 14:05 (try 2 of 6).");
    expect(out.detail).not.toMatch(/until you send them/);
  });
  it("a halted send: revocations go on their own, nothing waits for you", () => {
    expect(retireOutcome(2, "halted:zitadel_offline")).toMatchObject({ kind: "queued", sendsItself: true });
  });
});

describe("removing direct access (W7)", () => {
  it("gone only when nothing waits", () => {
    expect(removalOutcome({ retained_roles: [], status: "applied" }, "Laser")).toMatchObject({
      kind: "applied",
      detail: "Nothing else was supplying it, so the access is gone.",
    });
  });
  it("a role that waits: revoked in Syndra, leaves Zitadel when you send it", () => {
    const out = removalOutcome({ wait_reason: "role_waits", status: "pending" }, "Laser");
    expect(out.kind).toBe("queued");
    expect(out.message).toBe("Revoked in Syndra. Laser always waits for you, so it leaves Zitadel when you send it.");
  });
  it("Zitadel didn't take it: says when it is sent again, never that access is gone", () => {
    const out = removalOutcome(
      { wait_reason: "halted:retrying", status: "pending", retry: { state: "retrying", try: 2, of: 6, next_try: NEXT } },
      "Laser",
    );
    expect(out).toMatchObject({ kind: "queued", sendsItself: true });
    expect(out.message).toBe("Revoked in Syndra. Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6).");
    expect(`${out.message} ${out.detail}`).not.toMatch(/access is gone/);
  });
});

describe("what they still hold after a removal", () => {
  const names = {
    project: (id: string) => ({ pLab: "Lab" })[id] ?? id,
    role: (_p: string, key: string) => ({ trained: "Trained", admin: "Admin" })[key] ?? key,
  };
  it("names each role in its project, alphabetical, never the raw pair", () => {
    const out = removalOutcome({ retained_roles: ["pLab/trained", "pLab/admin"], status: "applied" }, "Laser", names);
    expect(out.detail).toBe("They still hold Admin in Lab, Trained in Lab, from a bundle or a rule.");
    expect(out.detail).not.toMatch(/pLab/);
  });
});
