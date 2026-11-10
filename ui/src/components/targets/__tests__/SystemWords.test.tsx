import { describe, expect, it } from "vitest";

import { reconcileLine } from "@/components/targets/TargetOverview";
import { SYSTEM_WORDS, targetReading, zitadelReading } from "@/components/targets/TargetReading";

/**
 * A-20 P5: one word per state across the System group. Zitadel, the Connected
 * systems row, a system's page and a member's own storage card read from one
 * table, so a state has one name wherever it is said.
 */
describe("one word per state", () => {
  const answered = ["answered", "answered", "answered"] as const;
  const system = (status: Parameters<typeof targetReading>[0]["status"], recent: Array<"answered" | "missed"> = [...answered]) =>
    targetReading({
      status,
      name: "TrueNAS",
      health: { status: "online", recent } as never,
    }).word;

  it("a connected system says only the table's words", () => {
    const words = new Set<string>(Object.values(SYSTEM_WORDS));
    for (const status of ["offline", "waiting_to_retry", undefined] as const) {
      expect(words.has(system(status))).toBe(true);
    }
  });

  it("answering, offline and waiting to retry are the same words Zitadel uses", () => {
    expect(system(undefined)).toBe(SYSTEM_WORDS.answering);
    expect(zitadelReading({ status: "ok", misses: 0 }).word).toBe(SYSTEM_WORDS.answering);
    expect(system("offline")).toBe(SYSTEM_WORDS.offline);
    expect(zitadelReading({ status: "error", misses: 3 }).word).toBe(SYSTEM_WORDS.offline);
    expect(system("waiting_to_retry")).toBe(SYSTEM_WORDS.waitingToRetry);
  });

  it("Zitadel: Answering, Not answering for one or two missed checks, Offline for three, Not connected when never set up", () => {
    expect(zitadelReading({ status: "ok", misses: 0 })).toMatchObject({ word: "Answering", tone: "healthy", offline: false });
    expect(zitadelReading({ status: "error", misses: 1 }).word).toBe("Not answering");
    expect(zitadelReading({ status: "error", misses: 2 }).word).toBe("Not answering");
    expect(zitadelReading({ status: "error", misses: 3 })).toMatchObject({ word: "Offline", offline: true });
    expect(zitadelReading({ status: "disabled", misses: 0 }).word).toBe("Not connected");
  });

  it("never says Unreachable, Online or Connected for a state", () => {
    const all = Object.values(SYSTEM_WORDS).join(" ");
    expect(all).not.toMatch(/Unreachable|Online|^Connected/);
  });

  it("a missed check does not make a system anything but Answering until the third", () => {
    const missed = ["answered", "missed", "missed"] as ("answered" | "missed")[];
    expect(system(undefined, missed)).toBe(SYSTEM_WORDS.answering);
  });
});

/** Counts of persons in the System group say "people" / "person", never "accounts". */
describe("counts of people", () => {
  it("say person for one and people for the rest", () => {
    expect(reconcileLine(1, 1, 1, "TrueNAS")).toBe(
      "1 person with a managed account · 1 fix waiting for you to send · 1 person with an account missing from TrueNAS",
    );
    expect(reconcileLine(0, 2, 3, "TrueNAS")).toBe(
      "0 people with a managed account · 2 fixes waiting for you to send · 3 people with an account missing from TrueNAS",
    );
  });

  it("never count people as accounts, users, members or holders", () => {
    expect(reconcileLine(5, 0, 2, "TrueNAS")).not.toMatch(/\d (accounts|users|members|holders)\b/);
  });
});
