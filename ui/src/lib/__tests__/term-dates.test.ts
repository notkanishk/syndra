import { describe, expect, it } from "vitest";

import { shortDay, termFieldErrors, validateTerms } from "@/lib/term-dates";

const t = (name: string, start: string, end: string) => ({ name, start, end });

// The server is the authority; these checks only say it before sending, so
// they mirror its rules and its field keys (`terms[1].start`).
describe("validateTerms", () => {
  it("accepts the usual three terms", () => {
    expect(
      validateTerms([
        t("Spring", "2026-01-01", "2026-05-31"),
        t("Summer", "2026-06-01", "2026-07-31"),
        t("Monsoon", "2026-08-01", "2026-12-31"),
      ]),
    ).toEqual({});
  });

  it("accepts a one-day term and an unsorted list", () => {
    expect(validateTerms([t("B", "2026-08-01", "2026-08-01"), t("A", "2026-01-01", "2026-01-02")])).toEqual({});
  });

  it("asks for a name, within 40 characters", () => {
    expect(validateTerms([t("  ", "2026-01-01", "2026-01-02")])["terms[0].name"]).toMatch(/name/i);
    expect(validateTerms([t("x".repeat(41), "2026-01-01", "2026-01-02")])["terms[0].name"]).toMatch(/40/);
    expect(validateTerms([t("x".repeat(40), "2026-01-01", "2026-01-02")])).toEqual({});
  });

  it("asks for both dates", () => {
    const e = validateTerms([t("Spring", "", "")]);
    expect(e["terms[0].start"]).toBeTruthy();
    expect(e["terms[0].end"]).toBeTruthy();
  });

  it("refuses a term that ends before it starts, on the end", () => {
    const e = validateTerms([t("Spring", "2026-05-31", "2026-01-01")]);
    expect(e["terms[0].end"]).toMatch(/before/i);
    expect(e["terms[0].start"]).toBeUndefined();
  });

  it("refuses an overlap, a shared day counting, on the later row's start", () => {
    const e = validateTerms([t("Spring", "2026-01-01", "2026-05-31"), t("Summer", "2026-05-31", "2026-07-31")]);
    expect(e["terms[1].start"]).toMatch(/Spring/);
    expect(e["terms[0].start"]).toBeUndefined();
    expect(validateTerms([t("Spring", "2026-01-01", "2026-05-30"), t("Summer", "2026-05-31", "2026-07-31")])).toEqual({});
  });

  it("does not call an overlap on a term whose own dates are wrong", () => {
    const e = validateTerms([t("Spring", "2026-01-01", "2026-05-31"), t("Summer", "2026-07-31", "2026-02-01")]);
    expect(Object.keys(e)).toEqual(["terms[1].end"]);
  });

  it("keeps between one and twelve terms", () => {
    expect(validateTerms([])["terms"]).toMatch(/at least one/i);
    const many = Array.from({ length: 13 }, (_, i) =>
      t(`T${i}`, `2026-${String(i + 1).padStart(2, "0")}-01`, `2026-${String(i + 1).padStart(2, "0")}-02`),
    );
    expect(validateTerms(many.slice(0, 12))).toEqual({});
    expect(validateTerms([...many.slice(0, 12), t("T12", "2027-01-01", "2027-01-02")])["terms"]).toMatch(/12/);
  });
});

describe("termFieldErrors", () => {
  it("keeps only the field keys of the server's 400 map, never its request id", () => {
    expect(termFieldErrors({ "terms[1].start": "overlaps", terms: "too many", request_id: "abc" })).toEqual({
      "terms[1].start": "Overlaps.",
      terms: "Too many.",
    });
    expect(termFieldErrors(undefined)).toEqual({});
  });
});

describe("shortDay", () => {
  it("reads a calendar day as itself, whatever the time zone", () => {
    const year = new Date().getFullYear();
    expect(shortDay(`${year}-08-01`)).toBe("1 Aug");
    expect(shortDay("1999-12-31")).toBe("31 Dec 1999");
  });
});
