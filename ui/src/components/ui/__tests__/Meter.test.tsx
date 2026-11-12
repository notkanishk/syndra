// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import { StatusMeters } from "@/components/results/StatusMeters";
import { Meter, METER_WORD, meterState } from "@/components/ui/Meter";
import { UsageCard } from "@/components/usage/UsageCard";
import { GIB, homeBytes, windows } from "@/components/usage/__tests__/fixtures";
import { MEMBER } from "@/components/results/copy";
import type { StatusResult } from "@/lib/queries/useMyStorage";

/**
 * One meter, one threshold: the same ratio gives the same colour and the same
 * word wherever a meter is drawn, and the colour never stands alone.
 */
const fill = () => screen.getByRole("meter").firstElementChild as HTMLElement;

describe("the threshold", () => {
  it("is nearly full from 90% and full at the limit, and nothing for no limit", () => {
    expect(meterState(89, 100)).toBe("ok");
    expect(meterState(90, 100)).toBe("nearly");
    expect(meterState(99.9, 100)).toBe("nearly");
    expect(meterState(100, 100)).toBe("full");
    expect(meterState(150, 100)).toBe("full");
    expect(meterState(5, 0)).toBe("ok");
  });
});

describe("colour is never the only signal", () => {
  it.each([
    [50, "bg-accent", null],
    [90, "bg-warn", "nearly full"],
    [100, "bg-warn", "full"],
  ])("%i of 100 is %s with the word %s", (value, token, word) => {
    render(<Meter value={value} limit={100} label="Space" valueText={`${value} of 100`} />);
    expect(fill()).toHaveClass(token);
    if (word) {
      expect(screen.getByText(word)).toBeInTheDocument();
      expect(screen.getByRole("meter")).toHaveAttribute("aria-valuetext", `${value} of 100, ${word}`);
    } else {
      expect(screen.queryByText(METER_WORD.nearly)).toBeNull();
      expect(screen.queryByText(METER_WORD.full)).toBeNull();
      expect(screen.getByRole("meter")).toHaveAttribute("aria-valuetext", "50 of 100");
    }
  });

  it("never draws a bar past full", () => {
    render(<Meter value={250} limit={100} label="Space" />);
    expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "100");
    expect(fill().style.width).toBe("100%");
  });
});

describe("the same value looks the same everywhere", () => {
  const status = (used: number): StatusResult =>
    ({ usable: true, standing: { usable: true, waiting_on: "none" }, meters: [{ label: "makers", used, limit: 100, unit: "items" }], taken_at: "2026-11-10T09:30:00Z", current: true }) as StatusResult;

  it.each([
    [50, "bg-accent", null],
    [92, "bg-warn", "nearly full"],
    [100, "bg-warn", "full"],
  ])("%i of 100: a status read and a statistic agree on colour and word", (used, token, word) => {
    const said = () => [screen.queryByText("nearly full") !== null, screen.queryByText("full") !== null];
    const expected = [word === "nearly full", word === "full"];

    render(<StatusMeters result={status(used)} name="Parts shelf" reader={MEMBER} />);
    expect(fill()).toHaveClass(token);
    expect(said()).toEqual(expected);
    cleanup();

    const latest = { day: "2026-11-01", value: used * GIB };
    render(<UsageCard title="t" windows={windows()} metrics={[homeBytes({ series: [{ dimensions: {}, latest, days: [] }] })]} limits={{ quota_field: 100 * GIB }} />);
    expect(fill()).toHaveClass(token);
    expect(said()).toEqual(expected);
  });
});

describe("there is one meter", () => {
  it("is the only place that writes role=meter", () => {
    const root = resolve(__dirname, "../../..");
    const files = ["components/kinds/storage/MyStorage.tsx", "components/kinds/storage/PersonStorage.tsx", "components/results/StatusMeters.tsx", "components/usage/MetricBlock.tsx"];
    for (const f of files) {
      const code = readFileSync(resolve(root, f), "utf8");
      expect(code, f).not.toMatch(/role="meter"/);
      expect(code, f).toMatch(/<Meter\b/);
    }
  });
});
