// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TermLine } from "@/components/home/TermLine";
import { PLENTY_BELOW, roomLine } from "@/components/kinds/storage/captions";
import { personLine } from "@/components/people/personLine";
import { needsYou, roomOf } from "@/components/shell/SyndraMark";
import { dayPhase, greeting } from "@/lib/greeting";
import type { UserAccessView } from "@/lib/queries/useUsers";
import { termHolding } from "@/lib/term-dates";

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

describe("the hour's light (§3.1, Q2): the one exception, inside its bounds", () => {
  const light = utility("hour-light");
  const reaches = Array.from(light.matchAll(/--hour-reach:\s*(\d+)%/g)).map((m) => Number(m[1]));

  it("places the day's parts at their hours, and the greeting says each in its word", () => {
    const at = (hour: number) => new Date(2026, 10, 18, hour, 30);
    const expected: Array<[number, string, string]> = [
      [4, "quiet", "Evening"], [5, "morning", "Morning"], [11, "morning", "Morning"], [12, "afternoon", "Afternoon"],
      [16, "afternoon", "Afternoon"], [17, "evening", "Evening"], [21, "evening", "Evening"], [22, "quiet", "Evening"],
    ];
    for (const [hour, phase, word] of expected) {
      expect(dayPhase(at(hour)), String(hour)).toBe(phase);
      expect(greeting(at(hour)), String(hour)).toBe(word);
    }
  });

  it("has a place for every part of the day, is still, is the accent, and goes under forced colours", () => {
    for (const phase of ["morning", "afternoon", "evening", "quiet"]) expect(light).toContain(`&[data-hour="${phase}"]`);
    expect(reaches).toHaveLength(4);
    expect(light).not.toMatch(/animation|transition/);
    expect(light).toContain("var(--accent)");
    expect(light).not.toMatch(/--(healthy|warn|danger)\b|#[0-9a-f]{3,6}\b/i);
    expect(light).toMatch(/@media \(forced-colors: active\)\s*\{\s*display: none;/);
    // A phone's gutter is 16px: the pool reaches it and no further, so nothing scrolls sideways.
    expect(light).toMatch(/@media \(width < theme\(--breakpoint-tablet\)\)\s*\{\s*inset-inline: -16px;/);
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

  it("marks each day someone's access ends, says it in words, and counts people once", () => {
    const ending = [
      { user_id: "u1", expires_at: "2026-11-24T10:00:00Z" },
      { user_id: "u2", expires_at: "2026-11-27T10:00:00Z" },
      { user_id: "u2", expires_at: "2026-11-27T11:00:00Z" },
      { user_id: "u3", expires_at: "2027-02-01T10:00:00Z" }, // after the term: not on its line
    ];
    render(<TermLine term={terms[0]} ending={ending} now={now} />);
    const line = screen.getByRole("img");
    expect(line).toHaveAccessibleName("Monsoon term, day 110 of 153. Ends 31 Dec. Access ends for two people on 24 Nov and 27 Nov.");
    expect(Array.from(line.querySelectorAll("[data-ending]")).map((m) => m.getAttribute("data-ending"))).toEqual(["2026-11-24", "2026-11-27"]);
    expect(line).toHaveTextContent("Ends 31 Dec");
  });

  it("counts a one-day term as one day", () => {
    render(<TermLine term={{ name: "Open day", start: "2026-11-18", end: "2026-11-18" }} ending={[]} now={now} />);
    expect(screen.getByRole("img")).toHaveAccessibleName(/^Open day term, day 1 of 1\./);
  });

  it("says only the term's day when nothing ends", () => {
    render(<TermLine term={terms[0]} ending={[]} now={now} />);
    expect(screen.getByRole("img")).toHaveAccessibleName("Monsoon term, day 110 of 153. Ends 31 Dec.");
  });
});

describe("the mark's room (Q5)", () => {
  it("says Zitadel first, unknown as unknown, and dims only a quiet room with nothing waiting", () => {
    expect(roomOf({ needs: true, degraded: true, quiet: false, complete: true })).toBe("waiting");
    expect(roomOf({ needs: undefined, degraded: false, quiet: false, complete: true })).toBeUndefined();
    expect(roomOf({ needs: true, degraded: false, quiet: true, complete: true })).toBe("needs");
    expect(roomOf({ needs: false, degraded: false, quiet: true, complete: true })).toBe("quiet");
    expect(roomOf({ needs: false, degraded: false, quiet: false, complete: true })).toBe("clear");
  });

  it("never says nothing waits from a reading that does not cover all of Home's blocks", () => {
    expect(roomOf({ needs: false, degraded: false, quiet: false, complete: false })).toBeUndefined();
    expect(roomOf({ needs: false, degraded: false, quiet: true, complete: false })).toBeUndefined();
    expect(roomOf({ needs: true, degraded: false, quiet: false, complete: false })).toBe("needs");
    expect(roomOf({ needs: false, degraded: true, quiet: false, complete: false })).toBe("waiting");
  });

  it("counts every queue the reading carries, whichever view shows, retries and unconfirmed operations too", () => {
    const zero = {
      pending_requests: 0, expiring_grants: 0, pending_propagation: 0, drift: 0, unconfirmed_revocations: 0,
      holds_due: 0, revocations_escalated: false, zitadel_reachable: true,
    };
    expect(needsYou(zero)).toBe(false);
    for (const one of [{ drift: 1 }, { expiring_grants: 1 }, { holds_due: 1 }, { retrying: 1 }, { gave_up: 1 }, { unconfirmed_operations: { truenas: 1 } }]) {
      expect(needsYou({ ...zero, ...one }), JSON.stringify(one)).toBe(true);
    }
  });
});

describe("the line that knows on a person's page (§9, Q10)", () => {
  const names = { role: (_p: string, key: string) => ({ op: "Operator", tr: "Trained", mb: "Member" })[key] ?? key };
  const read = { read_at: "2026-11-18T09:00:00Z", current: true, truncated: false };
  const view = (projects: Array<Record<string, unknown>>, observation = read) =>
    ({ projects, observation }) as unknown as UserAccessView;
  const now = new Date("2026-11-18T12:00:00Z");
  const laser = (extra: Record<string, unknown>) => ({ project_id: "pL", project_name: "Laser cutting", ...extra });
  const wood = (extra: Record<string, unknown>) => ({ project_id: "pW", project_name: "Wood shop", ...extra });

  it("says less when Zitadel's answer is missing, failed or cut short: it never counts", () => {
    const one = [laser({ standings: { op: "held" } })];
    expect(personLine(undefined, names)).toBe("Checking what they hold.");
    expect(personLine(view(one, { read_at: "", current: true, truncated: false }), names, now)).toBe("Zitadel hasn't answered yet, so what they hold isn't counted.");
    expect(personLine(view(one, { ...read, current: false }), names, now)).toBe("Zitadel couldn't be read just now, so what they hold isn't counted.");
    expect(personLine(view(one, { ...read, truncated: true }), names, now)).toBe("Zitadel's answer was cut short, so what they hold isn't counted.");
  });

  it("counts only what Zitadel shows held, never the records", () => {
    const projects = [
      laser({ standings: { op: "held", tr: "waiting" }, source_roles: [{ role_key: "tr" }, { role_key: "op" }] }),
      wood({ standings: { mb: "held" } }),
    ];
    expect(personLine(view(projects), names, now)).toMatch(/^Holds 2 roles across 2 projects\./);
    expect(personLine(view([laser({ standings: { op: "held" } })]), names, now)).toBe("Holds 1 role in Laser cutting.");
    expect(personLine(view([laser({ standings: { op: "sent" } })]), names, now)).toMatch(/^Holds no roles\./);
  });

  it("adds the one thing worth knowing: the soonest end of a held role, then what isn't in Zitadel, then what is on its way", () => {
    const soon = laser({
      standings: { op: "held", tr: "held" },
      holdings: { op: { kind: "until", until: "2026-11-30T12:00:00Z" }, tr: { kind: "until", until: "2026-11-21T12:00:00Z" } },
    });
    expect(personLine(view([soon]), names, now)).toBe("Holds 2 roles in Laser cutting. Trained in Laser cutting ends in 3 days.");
    // An end past the window, or on a role Zitadel does not show, is not worth the line.
    const later = laser({ standings: { op: "held", tr: "sent" }, holdings: { op: { kind: "until", until: "2027-01-30T12:00:00Z" }, tr: { kind: "until", until: "2026-11-20T12:00:00Z" } } });
    expect(personLine(view([later]), names, now)).toBe("Holds 1 role in Laser cutting. Trained in Laser cutting is on its way to Zitadel.");
    expect(personLine(view([laser({ standings: { op: "held", tr: "not_held" } })]), names, now)).toBe(
      "Holds 1 role in Laser cutting. Trained in Laser cutting isn't in Zitadel yet.",
    );
  });
});

describe("one sentence under the storage meter (Q18)", () => {
  const GB = 1024 ** 3;
  it("is plenty of room under half, what is left up to nearly full, getting full past it, and nothing at the limit", () => {
    expect(roomLine(0.2 * 100 * GB, 100 * GB)).toBe("Plenty of room.");
    expect(roomLine(PLENTY_BELOW * 100 * GB, 100 * GB)).toMatch(/^50(\.0)? GiB left\.$/);
    expect(roomLine(0.95 * 100 * GB, 100 * GB)).toMatch(/^Getting full: 5(\.0)? GiB left\.$/);
    expect(roomLine(100 * GB, 100 * GB)).toBeNull();
    expect(roomLine(5 * GB, 0)).toBeNull();
  });
});
