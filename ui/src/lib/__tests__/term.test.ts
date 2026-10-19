import { describe, expect, it } from "vitest";

import { daysUntilTermEnd, keepOptions, nextTermEnd } from "@/lib/term";

describe("nextTermEnd", () => {
  it("returns the May boundary from earlier in the year", () => {
    expect(nextTermEnd(new Date(2026, 1, 3))).toEqual(new Date(2026, 4, 18));
  });

  it("returns the December boundary once May has passed", () => {
    expect(nextTermEnd(new Date(2026, 7, 2))).toEqual(new Date(2026, 11, 18));
  });

  it("rolls into next year once December has passed", () => {
    expect(nextTermEnd(new Date(2026, 11, 20))).toEqual(new Date(2027, 4, 18));
  });

  const setting = [
    { name: "Spring", start: "2026-01-05", end: "2026-04-30" },
    { name: "Monsoon", start: "2026-08-03", end: "2026-11-27" },
  ];

  it("reads the end of the term holding today from the setting", () => {
    expect(nextTermEnd(new Date(2026, 8, 1), setting)).toEqual(new Date(2026, 10, 27));
    expect(daysUntilTermEnd(new Date(2026, 10, 26, 12), setting)).toBe(1);
  });

  it("reads the next term's end when today falls between two", () => {
    expect(nextTermEnd(new Date(2026, 6, 1), setting)).toEqual(new Date(2026, 10, 27));
  });

  it("falls back to the usual dates when the setting is unread, empty or all past", () => {
    expect(nextTermEnd(new Date(2026, 7, 2))).toEqual(new Date(2026, 11, 18));
    expect(nextTermEnd(new Date(2026, 7, 2), [])).toEqual(new Date(2026, 11, 18));
    expect(nextTermEnd(new Date(2026, 11, 2), setting)).toEqual(new Date(2026, 11, 18));
  });

  it("never resolves to zero days, even on the boundary itself", () => {
    expect(daysUntilTermEnd(new Date(2026, 4, 18, 12))).toBeGreaterThanOrEqual(1);
  });
});

describe("keepOptions", () => {
  const terms = [
    { name: "Spring", start: "2026-01-01", end: "2026-05-31" },
    { name: "Summer", start: "2026-06-01", end: "2026-07-31" },
    { name: "Monsoon", start: "2026-08-01", end: "2026-12-31" },
  ];
  const noon = (y: number, m: number, d: number) => new Date(y, m - 1, d, 12).toISOString();

  it("another month counts 30 days from the current end", () => {
    const { month } = keepOptions(noon(2026, 12, 18), terms, new Date(2026, 9, 17));
    expect(month.days).toBe(30);
    expect(new Date(month.until).getDate()).toBe(17);
    expect(new Date(month.until).getMonth()).toBe(0);
  });

  it("the end of next term is the term after the one holding today, counted from the current end", () => {
    // Today 17 Oct is in Monsoon; the term after it isn't in the list.
    const none = keepOptions(noon(2026, 12, 18), terms, new Date(2026, 9, 17));
    expect("reason" in none.term).toBe(true);

    const withNext = [...terms, { name: "Spring", start: "2027-01-01", end: "2027-05-31" }];
    const { term } = keepOptions(noon(2026, 12, 18), withNext, new Date(2026, 9, 17));
    if ("reason" in term) throw new Error("expected a choice");
    // 18 Dec to 31 May is 164 days.
    expect(term.days).toBe(164);
    expect(new Date(term.until).getMonth()).toBe(4);
    expect(new Date(term.until).getDate()).toBe(31);
  });

  it("is off, with a reason, when the end of next term isn't later than the current end", () => {
    const withNext = [...terms, { name: "Spring", start: "2027-01-01", end: "2027-05-31" }];
    const { term } = keepOptions(noon(2027, 6, 15), withNext, new Date(2026, 9, 17));
    expect(term).toEqual({ reason: expect.stringMatching(/already ends after/) });
  });

  it("takes the next term to start when today falls between two", () => {
    const gap = [
      { name: "A", start: "2026-01-01", end: "2026-03-31" },
      { name: "B", start: "2026-06-01", end: "2026-07-31" },
    ];
    const { term } = keepOptions(noon(2026, 4, 20), gap, new Date(2026, 3, 10));
    if ("reason" in term) throw new Error("expected a choice");
    expect(new Date(term.until).getMonth()).toBe(6);
  });
});
