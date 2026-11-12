import { describe, expect, it } from "vitest";

import { ago, GAP, GIB } from "@/components/usage/__tests__/fixtures";
import { capText, daysSince, dimensionWords, formatValue, gapIsOn, gapRange, plot, quoted, windowDays, WINDOW_DAYS } from "@/lib/usage";

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
  it("hatches every day before the window's start, for a counter and a gauge, and never makes one a zero", () => {
    for (const instrument of ["counter", "gauge"] as const) {
      const { cells, gapRuns } = plot(series, [], instrument, undefined, { from: ago(9) });
      const gone = cells.filter((c) => c.day < ago(9));
      expect(gone).toHaveLength(WINDOW_DAYS - 10);
      expect(gone.every((c) => c.gap && c.kept && c.value === undefined), instrument).toBe(true);
      expect(gapRuns).toEqual([[0, WINDOW_DAYS - 11]]);
      // The window's first day is an ordinary day again.
      expect(cells.find((c) => c.day === ago(9))?.gap).toBe(false);
    }
  });
  it("hatches nothing when the window starts where it always does", () => {
    expect(plot(series, [], "counter", undefined, { from: ago(29) }).gapRuns).toEqual([]);
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
