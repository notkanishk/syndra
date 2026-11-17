import { describe, expect, it } from "vitest";

import { ago, GAP, GIB } from "@/components/usage/__tests__/fixtures";
import type { StatsGap } from "@/lib/queries/useStats";
import { capText, change, daysSince, dimensionWords, formatValue, gapIsOn, gapLines, gapRange, gapWhere, plot, quoted, seriesName, windowDays, WINDOW_DAYS } from "@/lib/usage";

describe("a value in its unit", () => {
  it("says each registry unit in its own words", () => {
    expect(formatValue(7, "count")).toBe("7");
    expect(formatValue(1218, "count")).toBe("1,218");
    expect(formatValue(1.5 * GIB, "bytes")).toBe("1.5 GiB");
    expect(formatValue(3, "items")).toBe("3 items");
    expect(formatValue(1, "items")).toBe("1 item");
    expect(formatValue(5400, "seconds")).toBe("1.5 hours");
    expect(formatValue(1800, "seconds")).toBe("30 minutes");
    expect(formatValue(45, "seconds")).toBe("45 seconds");
    expect(formatValue(2500, "grams")).toBe("2.5 kg");
  });
});

describe("a gap's range, `end` being exclusive", () => {
  it("is one day, a run in a month, or two named ends", () => {
    expect(gapRange({ start: "2026-11-08", end: "2026-11-09" })).toBe("8 Nov");
    expect(gapRange({ start: "2026-11-08", end: "2026-11-10" })).toBe("8–9 Nov");
    expect(gapRange({ start: "2026-10-30", end: "2026-11-03" })).toBe("30 Oct to 2 Nov");
  });
});

describe("add-on text is capped and quoted", () => {
  it("cuts at the cap with an ellipsis and leaves short text whole", () => {
    expect(capText("short", 10)).toBe("short");
    expect(capText("x".repeat(50), 10)).toBe(`${"x".repeat(9)}…`);
    expect(Array.from(capText("😀".repeat(30), 10))).toHaveLength(10);
    expect(quoted("Auditing is off", 40)).toBe("“Auditing is off”");
  });
  it("words a series' dimensions from the descriptor's labels, the values quoted", () => {
    expect(dimensionWords({ dimensions: [{ name: "area", label: "Area" }] }, { area: "ring" })).toBe("Area: “ring”");
    expect(dimensionWords({}, {})).toBe("");
  });
});

describe("a gap belongs to the series it names", () => {
  it("matches every key it names; a gap naming none is the whole metric's", () => {
    expect(gapIsOn({}, { dimensions: { area: "ring" } })).toBe(true);
    expect(gapIsOn({ dimensions: { area: "ring" } }, { dimensions: { area: "ring" } })).toBe(true);
    expect(gapIsOn({ dimensions: { area: "ring" } }, { dimensions: { area: "mat" } })).toBe(false);
  });
});

describe("the plot of one series", () => {
  const days = windowDays();
  const series = { dimensions: { area: "ring" }, days: [{ day: ago(1), value: 3 }] };

  it("is 30 makerspace days ending today", () => {
    expect(days).toHaveLength(WINDOW_DAYS);
    expect(days[WINDOW_DAYS - 1]).toBe(ago(0));
  });

  it("makes a counter's missing day a zero and a gauge's missing day no value", () => {
    expect(plot(series, [], "counter").cells.find((c) => c.day === ago(2))?.value).toBe(0);
    expect(plot(series, [], "gauge").cells.find((c) => c.day === ago(2))?.value).toBeUndefined();
  });

  it("never gives a gap day a value, even when the point is there", () => {
    const withPoint = { ...series, days: [{ day: ago(6), value: 9 }, ...series.days] };
    for (const instrument of ["counter", "gauge"] as const) {
      const { cells, gapRuns } = plot(withPoint, [GAP], instrument);
      const inGap = cells.filter((c) => c.day >= GAP.start && c.day < GAP.end);
      expect(inGap).toHaveLength(3);
      expect(inGap.every((c) => c.gap && c.value === undefined), instrument).toBe(true);
      expect(gapRuns).toHaveLength(1);
      expect(gapRuns[0][1] - gapRuns[0][0]).toBe(2);
    }
  });

  it("leaves a series a gap does not name alone", () => {
    const mat = { dimensions: { area: "mat" }, days: [] };
    expect(plot(mat, [GAP], "counter").cells.every((c) => !c.gap && c.value === 0)).toBe(true);
  });
});

describe("add-on text cannot reorder or hide Syndra's line", () => {
  it("strips control and bidi characters, and keeps a control space a space", () => {
    expect(capText("a\u202Eb\u2066c\u200Bd", 40)).toBe("abcd");
    expect(capText("one\ntwo\tthree", 40)).toBe("one two three");
    expect(quoted("\u202Eevil", 40)).toBe("“evil”");
  });
});

describe("days Syndra no longer keeps", () => {
  const series = { dimensions: {}, days: [{ day: ago(1), value: 3 }] };
  it("counts a window's days from its start, today included: N days kept start N − 1 days ago", () => {
    expect(daysSince(ago(89))).toBe(90);
    expect(daysSince(ago(0))).toBe(1);
    expect(daysSince(ago(29))).toBe(WINDOW_DAYS);
  });
  it("draws every day before the window's start as before the record, for a counter and a gauge: never hatched, never a zero", () => {
    for (const instrument of ["counter", "gauge"] as const) {
      const { cells, gapRuns, recordFrom } = plot(series, [], instrument, undefined, { from: ago(9) });
      const gone = cells.filter((c) => c.day < ago(9));
      expect(gone).toHaveLength(WINDOW_DAYS - 10);
      expect(gone.every((c) => !c.gap && c.before === "kept" && c.value === undefined), instrument).toBe(true);
      expect(gapRuns).toEqual([]);
      // The window's first day kept is an ordinary day, and the record starts there.
      expect(cells.find((c) => c.day === ago(9))?.before).toBeUndefined();
      expect(recordFrom).toBe(ago(9));
    }
  });
  it("hatches nothing and starts the record at the window when the window starts where it always does", () => {
    const p = plot(series, [], "counter", undefined, { from: ago(29) });
    expect(p.gapRuns).toEqual([]);
    expect(p.recordFrom).toBe(ago(29));
  });
});

describe("days not yet counted", () => {
  const series = { dimensions: {}, days: [{ day: ago(3), value: 5 }, { day: ago(0), value: 2 }] };
  it("leaves a counter's day after the last final day blank when it has no point, and keeps a point it has", () => {
    const { cells } = plot(series, [], "counter", undefined, { countedThrough: ago(2) });
    const at = (n: number) => cells.find((c) => c.day === ago(n))!;
    expect(at(2).value).toBe(0); // final, nothing happened: a real zero
    expect(at(1).value).toBeUndefined();
    expect(at(1).pending).toBe(true);
    expect(at(0).value).toBe(2); // provisional, but it is a number
    expect(at(0).pending).toBeUndefined();
  });
  it("treats a null counted_through (nothing collected yet) as no day named", () => {
    expect(plot(series, [], "counter", undefined, { countedThrough: null }).cells.find((c) => c.day === ago(1))?.value).toBe(0);
  });
  it("is today's old behaviour when the backend names no day", () => {
    expect(plot(series, [], "counter").cells.find((c) => c.day === ago(1))?.value).toBe(0);
  });
});

describe("gaps said as lines", () => {
  const dims = (share: string) => ({ share });
  const audit = "Auditing does not record everyone on this share";

  it("makes one line of the same reason over the same days on several shares, sorted by date", () => {
    const lines = gapLines(
      [
        { start: "2026-11-14", end: "2026-11-16", dimensions: dims("main"), reason: audit },
        { start: "2026-11-14", end: "2026-11-16", dimensions: dims("gitlab_data"), reason: audit },
        { start: "2026-11-02", end: "2026-11-03", reason: "The NAS could not be read" },
      ],
      "2026-10-20",
      "2026-11-20",
    );
    expect(lines.map((l) => [l.start, l.end, l.reason])).toEqual([
      ["2026-11-02", "2026-11-03", "The NAS could not be read"],
      ["2026-11-14", "2026-11-16", audit],
    ]);
    const metric = { dimensions: [{ name: "share", label: "Share" }] } as Parameters<typeof gapWhere>[0];
    expect(gapWhere(metric, lines[1])).toBe("Share: \u201cgitlab_data\u201d and \u201cmain\u201d");
    expect(gapRange(lines[1])).toBe("14–15 Nov");
  });

  it("cuts what ends after the window, and lists three or more series as a list", () => {
    const lines = gapLines(
      ["a", "b", "c"].map((share) => ({ start: "2026-11-18", end: "2026-11-30", dimensions: dims(share), reason: audit })),
      "2026-10-20",
      "2026-11-20",
    );
    expect(lines.map((l) => [l.start, l.end])).toEqual([["2026-11-18", "2026-11-20"]]);
    const metric = { dimensions: [{ name: "share", label: "Share" }] } as Parameters<typeof gapWhere>[0];
    expect(gapWhere(metric, lines[0])).toBe("Share: \u201ca\u201d, \u201cb\u201d and \u201cc\u201d");
  });

  it("joins consecutive days of one series", () => {
    const lines = gapLines(
      [
        { start: "2026-11-10", end: "2026-11-11", dimensions: dims("main"), reason: audit },
        { start: "2026-11-11", end: "2026-11-12", dimensions: dims("main"), reason: audit },
      ],
      "2026-10-20",
      "2026-11-20",
    );
    expect(lines).toHaveLength(1);
    expect([lines[0].start, lines[0].end]).toEqual(["2026-11-10", "2026-11-12"]);
  });

  it("drops what is outside the window shown and cuts what starts before it", () => {
    const lines = gapLines(
      [
        { start: "2026-08-01", end: "2026-08-05", reason: "gone" },
        { start: "2026-09-01", end: "2026-11-01", reason: "Not collected yet: counting began on 1 Nov" },
        { start: "2026-11-25", end: "2026-11-26", reason: "future" },
      ],
      "2026-10-20",
      "2026-11-20",
    );
    expect(lines.map((l) => [l.start, l.end])).toEqual([["2026-10-20", "2026-11-01"]]);
  });
});

describe("the record's start is the add-on's (A-23 Q1, a gap's kind)", () => {
  const series = (days: Array<[number, number]>) => ({ dimensions: {}, days: days.map(([d, value]) => ({ day: ago(d), value })) });
  const gap = (from: number, to: number, kind: StatsGap["kind"]): StatsGap => ({ start: ago(from), end: ago(to), kind, reason: "a reason" });

  it("draws the days before the began day as before the record: not hatched, not a zero", () => {
    const p = plot(series([[3, 2]]), [], "counter", undefined, { began: ago(9) });
    const early = p.cells.filter((c) => c.day < ago(9));
    expect(early).toHaveLength(WINDOW_DAYS - 10);
    expect(early.every((c) => c.before === "began" && !c.gap && c.value === undefined)).toBe(true);
    expect(p.gapRuns).toEqual([]);
    expect(p.recordFrom).toBe(ago(9));
    // Inside the record a counter's quiet final day is still a real zero.
    expect(p.cells.find((c) => c.day === ago(8))?.value).toBe(0);
  });

  it("reads a began gap on the series the same way, and hatches every other kind, an outage at the window's start included", () => {
    for (const kind of ["unread", "dropped", "unobserved"] as const) {
      const p = plot(series([]), [gap(40, 20, kind)], "gauge");
      expect(p.gapRuns, kind).toEqual([[0, 8]]);
      expect(p.cells.slice(0, 9).every((c) => c.gap && !c.before), kind).toBe(true);
    }
    const began = plot(series([]), [gap(40, 20, "began")], "counter");
    expect(began.gapRuns).toEqual([]);
    expect(began.cells.slice(0, 9).every((c) => c.before === "began" && c.value === undefined)).toBe(true);
    expect(began.recordFrom).toBe(ago(20));
  });

  it("a year on, the began day still holds and a later whole-metric outage stays an unread gap: the tick does not move", () => {
    // The began gap ended a year ago (kept by the backend as `began`); an outage crossed the window's first day on a quiet counter.
    const p = plot(series([[3, 2]]), [gap(35, 20, "unread")], "counter", undefined, { began: ago(365) });
    expect(p.recordFrom).toBe(ago(29));
    expect(p.cells.some((c) => c.before)).toBe(false);
    expect(p.gapRuns).toEqual([[0, 8]]);
  });

  it("a short buffer: days before the began day are never a counter's zero", () => {
    const p = plot(series([]), [gap(36, 7, "began")], "counter", undefined, { began: ago(7) });
    expect(p.cells.filter((c) => c.value === 0).every((c) => c.day >= ago(7))).toBe(true);
    expect(p.cells.filter((c) => c.day < ago(7)).every((c) => c.before === "began")).toBe(true);
  });
});

describe("a level's change over the record", () => {
  it("is its first and its latest counted day, and none with fewer than two", () => {
    const p = plot({ dimensions: {}, days: [{ day: ago(6), value: 4 }, { day: ago(2), value: 9 }] }, [], "gauge");
    expect(change(p)).toEqual({ from: ago(6), first: 4, last: 9 });
    expect(change(plot({ dimensions: {}, days: [{ day: ago(2), value: 9 }] }, [], "gauge"))).toBeNull();
  });
});

describe("a row's name", () => {
  it("is the dimension values joined, capped, with no label and no quotes", () => {
    const metric = { dimensions: [{ name: "share", label: "Share" }, { name: "kind", label: "Kind" }] };
    expect(seriesName(metric, { share: "makers", kind: "smb" })).toBe("makers · smb");
    expect(seriesName(metric, { share: "x".repeat(80) })).toBe(`${"x".repeat(39)}…`);
    expect(seriesName({}, {})).toBe("");
  });
});
