import { describe, expect, it } from "vitest";

import { appSignInMeta, projectSignInLine } from "@/lib/sign-in-line";

const now = new Date("2026-11-03T12:00:00Z");
const apps = [
  { id: "fq", name: "Fab queue", project_id: "fab" },
  { id: "lb", name: "Laser bookings", project_id: "laser" },
  { id: "lr", name: "Laser badge reader", project_id: "laser" },
];
const row = (project_id: string, app_id: string, last_at: string) => ({ project_id, app_id, kind: "sign_in" as const, last_at });

describe("a project's last-sign-in line (A-10 §6)", () => {
  it("names the app used last, relative under a day, a date after", () => {
    const signIns = {
      receiving: true,
      usage: [row("fab", "fq", "2026-11-03T11:50:00Z"), row("laser", "lr", "2026-10-02T09:00:00Z"), row("laser", "lb", "2026-10-31T09:00:00Z")],
    };
    expect(projectSignInLine("fab", apps, signIns, now)).toBe("Last sign-in 10 min ago, to Fab queue");
    expect(projectSignInLine("laser", apps, signIns, now)).toBe("Last sign-in 31 Oct, to Laser bookings");
  });

  it("names no app for a row from before apps were kept", () => {
    expect(projectSignInLine("fab", apps, { receiving: true, usage: [row("fab", "", "2026-11-03T09:00:00Z")] }, now)).toBe(
      "Last sign-in 3 hours ago",
    );
  });

  it("says none in 90 days, or since Syndra began reading, with its app or apps", () => {
    expect(projectSignInLine("fab", apps, { receiving: true, usage: [] }, now)).toBe("No sign-in to its app in 90 days");
    expect(projectSignInLine("laser", apps, { receiving: true, usage: [], receiving_since: "2026-09-03T08:00:00Z" }, now)).toBe(
      "No sign-in to its apps since 3 Sept",
    );
  });

  it("shows nothing for a project with no app, or while sign-ins aren't arriving", () => {
    expect(projectSignInLine("wood", apps, { receiving: true, usage: [] }, now)).toBeNull();
    expect(projectSignInLine("fab", apps, { receiving: false, usage: [row("fab", "fq", "2026-11-03T11:50:00Z")] }, now)).toBeNull();
    expect(projectSignInLine("fab", apps, undefined, now)).toBeNull();
  });
});

describe("an app's sign-in meta (A-10 §7)", () => {
  it("reads people in 7 days and the last sign-in", () => {
    expect(appSignInMeta({ receiving: true, people_7d: 9, last_at: "2026-11-03T11:50:00Z" }, now)).toBe(
      "9 people in 7 days · last sign-in 10 min ago",
    );
    expect(appSignInMeta({ receiving: true, people_7d: 1, last_at: "2026-11-03T11:50:00Z" }, now)).toBe(
      "1 person in 7 days · last sign-in 10 min ago",
    );
    expect(appSignInMeta({ receiving: true, people_7d: 0, last_at: "2026-10-02T09:00:00Z" }, now)).toBe(
      "Nobody in 7 days · last sign-in 2 Oct",
    );
  });

  it("says no sign-in yet, and nothing while none arrive", () => {
    expect(appSignInMeta({ receiving: true, people_7d: 0, last_at: null }, now)).toBe("No sign-in yet");
    expect(appSignInMeta({ receiving: false }, now)).toBeNull();
    expect(appSignInMeta(undefined, now)).toBeNull();
  });
});
