import { describe, expect, it } from "vitest";

import { ago, GAP, GIB } from "@/components/usage/__tests__/fixtures";
import { capText, dimensionWords, formatValue, gapIsOn, gapRange, plot, quoted, windowDays, WINDOW_DAYS } from "@/lib/usage";

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
