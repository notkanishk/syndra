// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { TermLine } from "@/components/home/TermLine";
import { PLENTY_BELOW, roomLine } from "@/components/kinds/storage/captions";
import { CHECKING, personLine } from "@/components/people/personLine";
import { dayPhase, greeting, isQuietHour } from "@/lib/greeting";
import type { UserAccessView } from "@/lib/queries/useUsers";
import { termHolding } from "@/lib/term-dates";
import { inZone } from "@/test-utils/zone";

import { block, colour, css, over, ratio } from "./cssTokens";

/**
 * A-24 wave 3, places that know (§3.1, Q5, §9, Q18). Every line that knows
 * reads the same observed answer as the numbers beside it, and says less when
 * that answer is missing.
 */

const utility = (name: string) => {
  const start = css.indexOf(`@utility ${name} {`);
  let depth = 0;
  for (let i = css.indexOf("{", start); i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}" && --depth === 0) return css.slice(start, i + 1);
  }
  return "";
};

// The setup holds the zone from TZ for the whole file: each test that picks its own puts it back.
  afterEach(() => inZone(process.env.TZ));

describe("the hour's light (§3.1, Q2): the one exception, inside its bounds", () => {
  const light = utility("hour-light");
  const reaches = Array.from(light.matchAll(/--hour-reach:\s*(\d+)%/g)).map((m) => Number(m[1]));

  it("places the day's parts on the makerspace's clock, never the browser's, and the greeting says each in its word", () => {
    inZone("UTC");
    const at = (hour: number) => new Date(`2026-11-18T${String(hour).padStart(2, "0")}:30:00Z`);
    const expected: Array<[number, string, string]> = [
      [0, "quiet", "Evening"], [4, "quiet", "Evening"], [5, "morning", "Morning"], [11, "morning", "Morning"],
      [12, "afternoon", "Afternoon"], [16, "afternoon", "Afternoon"], [17, "evening", "Evening"], [21, "evening", "Evening"],
      [22, "quiet", "Evening"],
    ];
    for (const [hour, phase, word] of expected) {
      expect(dayPhase(at(hour)), String(hour)).toBe(phase);
      expect(greeting(at(hour)), String(hour)).toBe(word);
    }
    // One instant, two configured zones, two parts of the day: whatever the browser's clock says.
    const eight = new Date("2026-11-18T20:00:00Z");
    expect(dayPhase(eight)).toBe("evening");
    inZone("Asia/Kolkata"); // 01:30 there
    expect(dayPhase(eight)).toBe("quiet");
    expect(isQuietHour(eight)).toBe(true);
  });

  it("has a place for every part of the day, is still, is the accent, fades inside its box, and goes under forced colours", () => {
    for (const phase of ["morning", "afternoon", "evening", "quiet"]) expect(light).toContain(`&[data-hour="${phase}"]`);
    expect(reaches).toHaveLength(4);
    expect(light).not.toMatch(/animation|transition/);
    expect(light).toContain("var(--accent)");
    expect(light).not.toMatch(/--(healthy|warn|danger)\b|#[0-9a-f]{3,6}\b/i);
    expect(light).toMatch(/@media \(forced-colors: active\)\s*\{\s*display: none;/);
    // A phone's gutter is 16px: the pool reaches it and no further, so nothing scrolls sideways.
    expect(light).toMatch(/@media \(width < theme\(--breakpoint-tablet\)\)\s*\{\s*inset-inline: -16px;/);
    // No edge of its box is ever seen: a mask takes it to nothing on all four sides.
    expect(light).toMatch(/mask-image:\s*linear-gradient\(to right, transparent,[^)]*transparent\),\s*linear-gradient\(to bottom, transparent,[^)]*transparent\);/);
    expect(light).toContain("mask-composite: intersect;");
  });

  it("never takes the head's text under 4.5:1, at the pool's strongest, in both themes", () => {
    // The ramp the themes point into, then each theme's own tokens.
    const ramp = block(":root {\n    --violet-600");
    const themes = { dark: { ...ramp, ...block(':root,\n  [data-theme="dark"] {') }, light: { ...ramp, ...block('[data-theme="light"] {') } };
    const failures: string[] = [];
    for (const [theme, vars] of Object.entries(themes)) {
      const accent = colour(vars, "--accent");
      const weight = Number(vars["--hour-weight"]);
      expect(weight, `${theme} --hour-weight`).toBeGreaterThan(0);
      // On paper the pool stays under a quarter strength (A-24 frame 3): the margin the floor alone would not keep.
      if (theme === "light") for (const reach of reaches) expect(reach * weight, `light at ${reach}%`).toBeLessThan(25);
      for (const reach of reaches) {
        const lit = over([accent[0], accent[1], accent[2], (accent[3] ?? 1) * ((reach * weight) / 100)], colour(vars, "--canvas"));
        // The head sits on the app's canvas (`#app-root`), and its links end their hover on --accent-text.
        for (const token of ["--ink", "--muted", "--faint", "--accent-text"]) {
          const r = ratio(colour(vars, token), lit);
          if (r < 4.5) failures.push(`${theme} ${token} at ${reach}%: ${r.toFixed(2)}`);
        }
      }
    }
    expect(failures).toEqual([]);
  });
});

describe("the term line (§3.1, Q3)", () => {
  const terms = [
    { name: "Monsoon", start: "2026-08-01", end: "2026-12-31" },
    { name: "Spring term", start: "2027-01-10", end: "2027-05-20" },
  ];
  const now = new Date("2026-11-18T12:00:00Z");

  it("is the term holding today, and none between terms", () => {
    expect(termHolding(terms, now)?.name).toBe("Monsoon");
    expect(termHolding(terms, new Date("2027-01-05T12:00:00Z"))).toBeUndefined();
  });

  it("says its words as plain text, and names only its track: the days access ends, people counted once", () => {
    const ending = [
      { user_id: "u1", expires_at: "2026-11-24T10:00:00Z" },
      { user_id: "u2", expires_at: "2026-11-27T10:00:00Z" },
      { user_id: "u2", expires_at: "2026-11-27T11:00:00Z" },
      { user_id: "u3", expires_at: "2027-02-01T10:00:00Z" }, // after the term: not on its line
    ];
    render(<TermLine term={terms[0]} ending={ending} now={now} />);
    expect(screen.getByText("Monsoon term, day 110 of 153")).toBeInTheDocument();
    expect(screen.getByText("Ends 31 Dec")).toBeInTheDocument();
    const track = screen.getByRole("img");
    expect(track).toHaveAccessibleName("Access ends for two people on 24 Nov and 27 Nov.");
    expect(track.closest("[data-term-line]")).not.toHaveAttribute("role");
    expect(Array.from(track.querySelectorAll("[data-ending]")).map((m) => m.getAttribute("data-ending"))).toEqual(["2026-11-24", "2026-11-27"]);
    expect(track.querySelector("[data-today]")).not.toBeNull();
  });

  it("is drawn as the board draws it: no dot of its own, and on a phone one full-width track under its words (A-24 frames 2, 2p)", () => {
    render(<TermLine term={terms[0]} ending={[]} now={now} />);
    const line = document.querySelector("[data-term-line]")!;
    expect(line.querySelector("[data-dot]")).toBeNull();
    // A grid, never a wrapping row: "Ends" can't fall onto a line of its own under a short track.
    expect(line).toHaveClass("grid", "grid-cols-[minmax(0,1fr)_auto]", "tablet:grid-cols-[auto_minmax(0,1fr)_auto]", "max-w-[760px]");
    expect(line).not.toHaveClass("flex-wrap");
    const track = line.querySelector("[data-term-track]")!;
    // Phone: its own row, both columns wide. From tablet: the middle column, between the words.
    expect(track).toHaveClass("col-span-2", "row-start-2", "tablet:col-span-1", "tablet:col-start-2", "tablet:row-start-1");
    expect(track).not.toHaveClass("max-w-80");
  });

  // A-24 frame 2: "Hover or focus on a mark reads its day, as the usage band does." The track is one
  // focusable image; the words go to the line's readout, the term's day's seat, and back when it is left.
  it("reads a marked day on hover, and on focus with the arrows, in the line's readout", () => {
    const ending = [
      { user_id: "u1", expires_at: "2026-11-24T10:00:00Z" },
      { user_id: "u2", expires_at: "2026-11-24T11:00:00Z" },
      { user_id: "u3", expires_at: "2026-11-27T10:00:00Z" },
    ];
    render(<TermLine term={terms[0]} ending={ending} now={now} />);
    const readout = () => document.querySelector("[data-term-readout] [aria-live]")!.textContent;
    const track = screen.getByRole("img");
    expect(readout()).toBe("Monsoon term, day 110 of 153");
    fireEvent.pointerEnter(document.querySelector('[data-ending="2026-11-27"]')!);
    expect(readout()).toBe("Access ends for one person on 27 Nov.");
    fireEvent.pointerLeave(track);
    expect(readout()).toBe("Monsoon term, day 110 of 153");
    expect(track).toHaveAttribute("tabindex", "0");
    fireEvent.focus(track);
    expect(readout()).toBe("Access ends for two people on 24 Nov.");
    fireEvent.keyDown(track, { key: "ArrowRight" });
    expect(readout()).toBe("Access ends for one person on 27 Nov.");
    fireEvent.blur(track);
    expect(readout()).toBe("Monsoon term, day 110 of 153");
  });

  it("counts a one-day term as one day", () => {
    render(<TermLine term={{ name: "Open day", start: "2026-11-18", end: "2026-11-18" }} ending={[]} now={now} />);
    expect(screen.getByText("Open day term, day 1 of 1")).toBeInTheDocument();
  });

  it("names nothing when nothing ends: the track is only a picture", () => {
    render(<TermLine term={terms[0]} ending={[]} now={now} />);
    expect(screen.queryByRole("img")).toBeNull();
  });
});

describe("the line that knows on a person's page (§9, Q10)", () => {
  const names = { role: (_p: string, key: string) => ({ op: "Operator", tr: "Trained", mb: "Member" })[key] ?? key };
  const read = { read_at: "2026-11-18T09:00:00Z", current: true, truncated: false };
  const view = (projects: Array<Record<string, unknown>>, count: number | undefined, observation = read) =>
    ({ projects, observation, observed_role_count: count }) as unknown as UserAccessView;
  const now = new Date("2026-11-18T12:00:00Z");
  const laser = (extra: Record<string, unknown>) => ({ project_id: "pL", project_name: "Laser cutting", source_roles: [], derived_roles: [], ...extra });
  const wood = (extra: Record<string, unknown>) => ({ project_id: "pW", project_name: "Wood shop", source_roles: [], derived_roles: [], ...extra });

  it("says only that it is checking while there is no count: loading, or a read missing, failed or cut short", () => {
    const one = [laser({ standings: { op: "held" } })];
    expect(personLine(undefined, names).text).toBe(CHECKING);
    expect(personLine(view(one, undefined), names, now).text).toBe(CHECKING);
    // A zero is an absence claim, and waits for a read that succeeded and saw everything.
    expect(personLine(view([], 0, { read_at: "", current: true, truncated: false }), names, now).text).toBe(CHECKING);
    expect(personLine(view([], 0, { ...read, current: false }), names, now).text).toBe(CHECKING);
    expect(personLine(view([], 0, { ...read, truncated: true }), names, now).text).toBe(CHECKING);
  });

  it("counts as the People row counts (observedHolding), with its projects from Zitadel's standings", () => {
    const projects = [
      laser({ standings: { op: "held", tr: "waiting" }, source_roles: [{ role_key: "tr", reasons: [] }] }),
      wood({ standings: { mb: "held" } }),
    ];
    expect(personLine(view(projects, 2), names, now).text).toMatch(/^Holds 2 roles across 2 projects\./);
    expect(personLine(view([laser({ standings: { op: "held" } })], 1), names, now).text).toBe("Holds 1 role in Laser cutting.");
    expect(personLine(view([laser({ standings: { op: "sent" } })], 0), names, now).text).toMatch(/^Holds no roles\./);
  });

  it("adds the one thing worth knowing: the soonest end of a held role, then Not in Zitadel, then what is on its way", () => {
    const soon = laser({
      standings: { op: "held", tr: "held" },
      holdings: { op: { kind: "until", until: "2026-11-30T12:00:00Z" }, tr: { kind: "until", until: "2026-11-21T12:00:00Z" } },
    });
    expect(personLine(view([soon], 2), names, now).text).toBe("Holds 2 roles in Laser cutting. Trained in Laser cutting ends in 3 days.");
    // An end past the window, or on a role Zitadel does not show, is not worth the line.
    const later = laser({ standings: { op: "held", tr: "sent" }, holdings: { op: { kind: "until", until: "2027-01-30T12:00:00Z" }, tr: { kind: "until", until: "2026-11-20T12:00:00Z" } } });
    expect(personLine(view([later], 1), names, now).text).toBe("Holds 1 role in Laser cutting. Trained in Laser cutting is on its way to Zitadel.");
    expect(personLine(view([laser({ standings: { op: "held", tr: "not_held" } })], 1), names, now).text).toBe(
      "Holds 1 role in Laser cutting. Not in Zitadel: Trained in Laser cutting.",
    );
  });

  it("wears its ladder's dot, as A-24 frame 18 draws it: amber when it names an end or a role not in Zitadel", () => {
    const ends = laser({ standings: { op: "held" }, holdings: { op: { kind: "until", until: "2026-11-21T12:00:00Z" } } });
    expect(personLine(view([ends], 1), names, now).tone).toBe("warn");
    expect(personLine(view([laser({ standings: { op: "held", tr: "not_held" } })], 1), names, now).tone).toBe("warn");
    expect(personLine(view([laser({ standings: { op: "held", tr: "sent" } })], 1), names, now).tone).toBe("neutral");
    expect(personLine(view([laser({ standings: { op: "held" } })], 1), names, now).tone).toBe("neutral");
    expect(personLine(undefined, names).tone).toBe("none");
  });

  it("never says a role held twice ends: a direct grant's end is not the bundle's", () => {
    const twice = laser({
      standings: { tr: "held" },
      holdings: { tr: { kind: "until", until: "2026-11-21T12:00:00Z" } },
      source_roles: [{ role_key: "tr", reasons: [{ kind: "direct" }, { kind: "bundle", bundle_id: "b1", bundle_name: "Lab Tech" }] }],
    });
    expect(personLine(view([twice], 1), names, now).text).toBe("Holds 1 role in Laser cutting.");
  });
});

describe("one sentence under the storage meter (Q18)", () => {
  const GB = 1024 ** 3;
  it("is plenty of room under half, then what is left, and nothing at the limit", () => {
    expect(roomLine(0.2 * 100 * GB, 100 * GB)).toBe("Plenty of room.");
    expect(roomLine(PLENTY_BELOW * 100 * GB, 100 * GB)).toMatch(/^50(\.0)? GiB left\.$/);
    // Past 90% the meter itself says nearly full: the sentence says only what is left.
    expect(roomLine(0.95 * 100 * GB, 100 * GB)).toMatch(/^5(\.0)? GiB left\.$/);
    expect(roomLine(100 * GB, 100 * GB)).toBeNull();
    expect(roomLine(5 * GB, 0)).toBeNull();
  });
});
