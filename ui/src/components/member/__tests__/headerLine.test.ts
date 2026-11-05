import { describe, expect, it } from "vitest";

import { headerLine, type HeaderFacts, type NamedRole } from "@/components/member/headerLine";
import { formatShortDate } from "@/lib/format";

const NOW = new Date(2026, 10, 5, 10, 0, 0);
const inDays = (n: number) => new Date(NOW.getTime() + n * 864e5).toISOString();
const role = (label: string, projectId = "p1", project = "Laser cutting"): NamedRole => ({ label, projectId, project });

const calm: HeaderFacts = {
  loading: false,
  read: true,
  now: NOW,
  ending: [],
  missing: [],
  requests: [],
  onItsWay: [],
  startedToday: [],
};
const line = (over: Partial<HeaderFacts>) => headerLine({ ...calm, ...over });

describe("headerLine on a first visit (A-20 P2)", () => {
  it("never hides a rung: a role given today is still said", () => {
    const got = line({ firstVisit: true, startedToday: [{ project: "Laser cutting", projectId: "p1" }] });
    expect(got.text).toBe("From today, you can use Laser cutting.");
  });

  it("a role that ends still leads", () => {
    const got = line({ firstVisit: true, ending: [{ ...role("Operator in Laser cutting"), endsAt: inDays(3) }] });
    expect(got.tone).toBe("warn");
  });

  it("the calm rung says what Syndra is, with no dot", () => {
    expect(line({ firstVisit: true })).toEqual({
      tone: "none",
      text: "It keeps the makerspace\u2019s list of who can use what, and why.",
    });
  });
});

describe("headerLine, rung by rung", () => {
  it("loading: no dot, 'Checking what you can use.'", () => {
    expect(line({ loading: true })).toEqual({ tone: "none", text: "Checking what you can use." });
  });

  it("Zitadel not read: neutral, and nothing else is said even if records have news", () => {
    expect(line({ read: false, onItsWay: [role("X in Y")] })).toEqual({
      tone: "neutral",
      text: "Zitadel hasn't answered yet, so your roles aren't shown.",
    });
  });

  it("1 · a held role ending within 14 days leads, warn, with the jump", () => {
    const got = line({
      ending: [{ ...role("Operator in Laser cutting"), endsAt: inDays(7) }],
      missing: [role("Hand tools in Wood shop", "p2", "Wood shop")],
      requests: [{ at: inDays(-1) }],
    });
    expect(got).toEqual({
      tone: "warn",
      text: "Operator in Laser cutting ends in 7 days.",
      link: { label: "Laser cutting", projectId: "p1" },
    });
  });

  it("1 · says tomorrow and today, not 'in 1 days'", () => {
    expect(line({ ending: [{ ...role("A in B"), endsAt: inDays(1) }] }).text).toBe("A in B ends tomorrow.");
    expect(line({ ending: [{ ...role("A in B"), endsAt: NOW.toISOString() }] }).text).toBe("A in B ends today.");
  });

  it("1 · with two or more, names the first and counts only the roles that end soon", () => {
    const got = line({
      ending: [
        { ...role("Later in Wood shop", "p2", "Wood shop"), endsAt: inDays(12) },
        { ...role("Operator in Laser cutting"), endsAt: inDays(7) },
        { ...role("Far in Resin"), endsAt: inDays(40) },
      ],
    });
    expect(got.text).toBe("Two of your roles end soon. The first, Operator in Laser cutting, ends in 7 days.");
  });

  it("1 · a role ending after 14 days does not speak", () => {
    expect(line({ ending: [{ ...role("A in B"), endsAt: inDays(15) }] }).text).toBe("Nothing needs you right now.");
  });

  it("2 · a given role not on the account", () => {
    expect(line({ missing: [role("Hand tools in Wood shop", "p2", "Wood shop")] })).toEqual({
      tone: "warn",
      text: "Hand tools in Wood shop isn't on your account yet.",
      link: { label: "Wood shop", projectId: "p2" },
    });
  });

  it("3 · a storage step links to Network storage", () => {
    expect(line({ storage: { name: "TrueNAS" }, requests: [{ at: inDays(-1) }] })).toEqual({
      tone: "neutral",
      text: "Your TrueNAS account is ready. Set its password to connect.",
      link: { label: "Network storage", href: "/storage" },
    });
  });

  it("4 · one request names it and when; several name the newest, never a count", () => {
    const at = "2026-11-03T09:00:00Z";
    expect(line({ requests: [{ label: "Bandsaw in Wood shop", at }] }).text).toBe(
      `Makerspace staff have your request for Bandsaw in Wood shop. You asked on ${formatShortDate(at)}.`,
    );
    const many = line({
      requests: [
        { label: "Old in Resin", at: "2026-10-01T09:00:00Z" },
        { label: "Bandsaw in Wood shop", at },
      ],
    });
    expect(many.text).toBe(
      `Makerspace staff have your requests. The newest, for Bandsaw in Wood shop, is from ${formatShortDate(at)}.`,
    );
    expect(many.text).not.toMatch(/\d+ requests|two requests/i);
    expect(many.link).toEqual({ label: "Your requests", href: "/requests" });
  });

  it("5 · an approval on its way", () => {
    expect(line({ onItsWay: [role("Resin printer in 3D printing", "p3", "3D printing")] })).toEqual({
      tone: "neutral",
      text: "Resin printer in 3D printing is on its way to you.",
      link: { label: "3D printing", projectId: "p3" },
    });
  });

  it("6 · the day access starts, healthy, projects alphabetical", () => {
    expect(line({ startedToday: [{ project: "Laser cutting", projectId: "p1" }] })).toEqual({
      tone: "healthy",
      text: "From today, you can use Laser cutting.",
      link: { label: "Laser cutting", projectId: "p1" },
    });
    expect(
      line({
        startedToday: [
          { project: "Wood shop", projectId: "p2" },
          { project: "Laser cutting", projectId: "p1" },
        ],
      }).text,
    ).toBe("From today, you can use Laser cutting and Wood shop.");
  });

  it("7 · calm is the operator's Home sentence, with no link", () => {
    expect(line({})).toEqual({ tone: "healthy", text: "Nothing needs you right now." });
  });

  it("holds the ladder's order: each rung beats every one below it", () => {
    const all: HeaderFacts = {
      ...calm,
      ending: [{ ...role("E in P"), endsAt: inDays(3) }],
      missing: [role("M in P")],
      storage: { name: "TrueNAS" },
      requests: [{ at: inDays(-1) }],
      onItsWay: [role("W in P")],
      startedToday: [{ project: "P", projectId: "p1" }],
    };
    const drop: Array<keyof HeaderFacts> = ["ending", "missing", "storage", "requests", "onItsWay", "startedToday"];
    const seen: string[] = [];
    let facts = all;
    for (const key of drop) {
      seen.push(headerLine(facts).text);
      facts = { ...facts, [key]: Array.isArray(all[key]) ? [] : undefined };
    }
    seen.push(headerLine(facts).text);
    expect(seen).toEqual([
      "E in P ends in 3 days.",
      "M in P isn't on your account yet.",
      "Your TrueNAS account is ready. Set its password to connect.",
      "Makerspace staff have your request. You asked on " + formatShortDate(inDays(-1)) + ".",
      "W in P is on its way to you.",
      "From today, you can use P.",
      "Nothing needs you right now.",
    ]);
  });
});

describe("headerLine since the last sign-in (A-20 P18)", () => {
  it("the calm rung says nothing has changed since then", () => {
    expect(line({ quietSince: "on Tuesday" }).text).toBe("Nothing has changed since you were last here, on Tuesday.");
  });
  it("any rung above it still speaks first", () => {
    expect(line({ quietSince: "on Tuesday", onItsWay: [role("Operator in Laser cutting")] }).text).toMatch(/on its way/);
  });
  it("a first visit says what Syndra is, never a last time", () => {
    expect(line({ firstVisit: true, quietSince: "on Tuesday" }).text).toMatch(/keeps the makerspace/);
  });
});
