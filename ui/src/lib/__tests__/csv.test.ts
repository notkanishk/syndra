import { describe, expect, it } from "vitest";

import { csvCell, toCsv } from "@/lib/csv";

describe("csv", () => {
  it.each(["=1+1", "+1", "-1", "@SUM(A1)", "\tx", "\rx"])("defuses a cell that starts as a formula: %j", (cell) => {
    expect(csvCell(cell)).toBe(`"'${cell}"`);
  });

  it("leaves ordinary cells alone and still doubles quotes", () => {
    expect(csvCell("Ada Lin")).toBe('"Ada Lin"');
    expect(csvCell("2026-10-27T10:00:00Z")).toBe('"2026-10-27T10:00:00Z"');
    expect(csvCell('say "hi"')).toBe('"say ""hi"""');
    expect(csvCell(null)).toBe('""');
  });

  it("only the start counts: a minus or equals mid-cell is not a formula", () => {
    expect(csvCell("a=b-c")).toBe('"a=b-c"');
  });

  it("joins rows and cells", () => {
    expect(toCsv([["a", "=x"], ["b", "c"]])).toBe(`"a","'=x"\n"b","c"`);
  });
});
