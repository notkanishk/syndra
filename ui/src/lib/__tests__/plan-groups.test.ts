import { describe, expect, it } from "vitest";

import { NOTHING_TO_DO, waitsForSend } from "@/lib/outcome";
import {
  GROUPS,
  NOTHING_TO_APPLY,
  REFUSED_ROWS_NEXT,
  applyLabel,
  beforeLine,
  foldOf,
  groupPlan,
  groupable,
  refusedSeat,
  resultOutcome,
  type FoldKey,
} from "@/lib/plan-groups";
import type { BulkEffect, BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";

// A-21 §Q1: a plan grouped by how each change travels. People, not writes;
// fixed folds; the same order in the true tense after Apply.

const CANCEL = "Cancels a change waiting for you to send: the grant of Member in Laser Lab queued 2 Nov 2026 (wrong person).";

const row = (name: string, over: Partial<BulkOutcome> = {}): BulkOutcome => ({
  user_id: `id-${name}`,
  name,
  email: `${name.toLowerCase()}@example.edu`,
  effect: "apply",
  detail: "Joins Safety.",
  consequence: "Gains Member in Laser Lab.",
  delivery: "straight_away",
  ...over,
});

function plan(outcomes: BulkOutcome[], over: Partial<BulkPlan> = {}): BulkPlan {
  const n = (e: BulkEffect) => outcomes.filter((o) => o.effect === e).length;
  return {
    op: "assign_bundle",
    plan_id: "plan_1",
    applied: false,
    outcomes,
    summary: {
      total: outcomes.length,
      apply: n("apply"),
      no_change: n("no_change"),
      blocked: n("blocked"),
      failed: n("failed"),
      succeeded: n("applied"),
      queued: n("queued"),
    },
    ...over,
  };
}

// The board's plan (a21-plan-grouped): a person with some of each, a
// cancel-only row, one that cancels and sends, one that sends nothing, No
// change and Refused.
const board = () =>
  plan([
    row("omar", { consequence: `Gains Member in Laser Lab. ${CANCEL}` }),
    row("Jules", {
      delivery: "waits_for_you",
      waits: ["Trained in Laser Lab"],
      goes: ["Member in Laser Lab"],
      consequence: "Gains Member in Laser Lab and Trained in Laser Lab. Giving Trained in Laser Lab waits for you to send.",
    }),
    row("Kabir", {
      delivery: "none",
      detail: "Leaves Safety.",
      consequence: `Cancels giving Member in Laser Lab, which was waiting for you to send, so nothing is sent to Zitadel. ${CANCEL}`,
    }),
    row("Lena", { delivery: "none", consequence: "Already holds every role it gives, so nothing is sent to Zitadel." }),
    row("Aanya", { delivery: "waits_for_you", waits: ["Supervisor in Laser Lab"] }),
    row("Dev", { effect: "no_change", delivery: undefined, detail: "Already in Safety.", consequence: undefined }),
    row("Sam", { effect: "blocked", delivery: undefined, detail: "Syndra couldn't read Zitadel's list of roles." }),
  ]);

const names = (g: { rows: BulkOutcome[] }) => g.rows.map((o) => o.name);

describe("groupPlan before Apply", () => {
  it("draws the three folds in their fixed order, one row per person, alphabetical inside each", () => {
    const groups = groupPlan(board())!;
    expect(groups.map((g) => g.heading)).toEqual(["Straight away", "Will wait for you to send", "Left as they are"]);
    expect(names(groups[0])).toEqual(["Kabir", "Lena", "omar"]); // case-insensitive
    expect(names(groups[1])).toEqual(["Aanya", "Jules"]);
    expect(names(groups[2])).toEqual(["Dev", "Sam"]);
    expect(groups.flatMap(names)).toHaveLength(7);
    expect(groups.map((g) => g.count)).toEqual(["3 people", "2 people", "2 people"]);
  });

  it("puts a person with any role that waits under the wait fold, with the cause line over the fold's roles", () => {
    const [, wait] = groupPlan(board())!;
    expect(wait.key).toBe("wait");
    expect(wait.dot).toBe("warn");
    expect(wait.link).toBeUndefined();
    expect(wait.note).toBe("Because Supervisor in Laser Lab and Trained in Laser Lab always wait for you.");
    const one = groupPlan(plan([row("Jules", { delivery: "waits_for_you", waits: ["Trained in Laser Lab"] })]))!;
    expect(one[0].note).toBe("Because Trained in Laser Lab always waits for you.");
    expect(one[0].count).toBe("1 person");
  });

  it("puts a cancel-only row and a row that sends nothing under Straight away, and only the third fold carries no dot", () => {
    const [go, , left] = groupPlan(board())!;
    expect(go.rows.map((o) => o.name)).toContain("Kabir");
    expect(go.dot).toBeUndefined();
    expect(left.dot).toBeUndefined();
  });

  it("does not draw a fold with nobody in it", () => {
    expect(groupPlan(plan([row("Ada")]))!.map((g) => g.key)).toEqual(["go"]);
    const nothing = groupPlan(plan([row("Dev", { effect: "no_change", delivery: undefined })]))!;
    expect(nothing.map((g) => g.heading)).toEqual(["Left as they are"]);
  });

  it("is flat when a row that goes ahead says nothing of how it travels: no op half-claims Straight away", () => {
    const mixed = plan([row("Ada"), row("Bo", { delivery: undefined })]);
    expect(groupable(mixed)).toBe(false);
    expect(groupPlan(mixed)).toBeNull();
    expect(groupPlan(plan([]))).toBeNull();
    expect(groupPlan(plan([row("Ada", { delivery: undefined })], { op: "edit_mapping" }))).toBeNull();
  });
});

describe("foldOf: every (effect, delivery) pair lands in exactly one fold", () => {
  const effects: BulkEffect[] = ["apply", "applied", "no_change", "blocked", "failed", "queued"];
  const deliveries = ["straight_away", "waits_for_you", "none", undefined] as const;
  it("before and after Apply, and the fold exists in that tense", () => {
    for (const applied of [false, true]) {
      for (const effect of effects) {
        for (const delivery of deliveries) {
          const key: FoldKey = foldOf(row("x", { effect, delivery }), { op: "assign_role", applied });
          const group = GROUPS.find((g) => g.key === key);
          expect(group, `${effect}/${delivery}`).toBeDefined();
          if (!applied && (effect === "apply" || effect === "no_change" || effect === "blocked")) {
            expect(group!.before, `${effect}/${delivery} before`).toBeDefined();
          }
        }
      }
    }
  });

  it("after Apply: a role that waits, a retry, a refusal and a rule's waiting role each have their fold", () => {
    const after = { op: "assign_role" as const, applied: true };
    expect(foldOf(row("x", { effect: "applied" }), after)).toBe("go");
    expect(foldOf(row("x", { effect: "applied", rule_waiting: 1 }), after)).toBe("wait");
    expect(foldOf(row("x", { effect: "queued", wait_reason: "role_waits" }), after)).toBe("wait");
    expect(foldOf(row("x", { effect: "queued", wait_reason: "halted:retrying", retry: { state: "retrying", try: 2, of: 6 } }), after)).toBe("unanswered");
    // Stopped trying: nothing sends it until Send.
    expect(foldOf(row("x", { effect: "queued", wait_reason: "halted:retrying", retry: { state: "waiting", try: 6, of: 6 } }), after)).toBe("wait");
    expect(foldOf(row("x", { effect: "failed", wait_reason: "refused:not_found" }), after)).toBe("refused");
    // A revocation that stopped goes on its own, unless the drain is off.
    const revoke = { op: "remove_role" as const, applied: true };
    expect(foldOf(row("x", { effect: "queued", wait_reason: "halted:zitadel_offline" }), revoke)).toBe("unanswered");
    expect(foldOf(row("x", { effect: "queued", wait_reason: "halted:zitadel_offline" }), revoke, false)).toBe("wait");
  });
});

describe("groupPlan after Apply", () => {
  const applied = () =>
    plan(
      [
        row("Ada", { effect: "applied" }),
        row("Bo", { effect: "applied" }),
        row("Jules", { effect: "queued", wait_reason: "role_waits", delivery: "waits_for_you", waits: ["Trained in Laser Lab"] }),
        row("Kai", { effect: "queued", wait_reason: "halted:retrying", retry: { state: "retrying", try: 2, of: 6, next_try: "2026-11-08T14:05:00Z" } }),
        row("Sam", { effect: "failed", wait_reason: "refused:not_found" }),
        row("Dev", { effect: "no_change", delivery: undefined }),
      ],
      { applied: true, plan_id: undefined },
    );

  it("crosses each heading to the true tense, in the same order", () => {
    const groups = groupPlan(applied())!;
    expect(groups.map((g) => g.heading)).toEqual([
      "Done",
      "Waiting for you to send",
      "Zitadel didn't answer",
      "Zitadel refused",
      "Left as they were",
    ]);
    expect(groups.map((g) => g.dot)).toEqual(["healthy", undefined, "warn", "danger", undefined]);
    expect(groups[1].link).toBe(true);
    expect(groups[2].note).toMatch(/^Zitadel didn't answer\. Syndra sends it again at .* \(try 2 of 6\)\.$/);
  });

  it("blooms Done only when every written row reached Zitadel", () => {
    expect(groupPlan(applied())![0].bloom).toBe(false);
    const all = plan([row("Ada", { effect: "applied" }), row("Dev", { effect: "no_change", delivery: undefined })], { applied: true });
    expect(groupPlan(all)![0].bloom).toBe(true);
  });
});

describe("beforeLine", () => {
  it("says every shape of finishes-now-or-waits, by changes", () => {
    expect(beforeLine(plan([row("Ada"), row("Bo")]))).toBe("Every change goes to Zitadel when you apply.");
    expect(beforeLine(plan([row("Ada", { delivery: "waits_for_you" })]))).toBe(
      "Every change waits for you to send. Nothing reaches Zitadel until you press Send on Pending changes.",
    );
    expect(beforeLine(plan([row("Ada"), row("Bo", { delivery: "waits_for_you" }), row("Cy", { delivery: "waits_for_you" })]))).toBe(
      "Apply sends 1 change to Zitadel now. 2 wait for you to send.",
    );
    expect(beforeLine(plan([row("Ada"), row("Bo"), row("Cy", { delivery: "waits_for_you" })]))).toBe(
      "Apply sends 2 changes to Zitadel now. 1 waits for you to send.",
    );
  });

  // Browser pass, 8 Nov: Community – Basic to 3 people while Navidrome
  // community waits. Every person sits under the wait fold, yet 6 changes go now.
  it("counts each person's roles, so a bundle with one waiting role is not all waiting", () => {
    const basic = (name: string) =>
      row(name, { delivery: "waits_for_you", goes: ["Audio-Dash", "Homeassistant"], waits: ["Navidrome community"] });
    const p = plan([basic("Avani"), basic("Bo"), basic("Cy")]);
    expect(groupPlan(p)!.map((g) => g.key)).toEqual(["wait"]);
    expect(beforeLine(p)).toBe("Apply sends 6 changes to Zitadel now. 3 wait for you to send.");
    expect(beforeLine(p)).not.toMatch(/Every change waits|Nothing reaches Zitadel/);
    const allWait = plan([row("Ada", { delivery: "waits_for_you", waits: ["A", "B"] })]);
    expect(beforeLine(allWait)).toBe("Every change waits for you to send. Nothing reaches Zitadel until you press Send on Pending changes.");
    const allNow = plan([row("Ada", { goes: ["A", "B"] })]);
    expect(beforeLine(allNow)).toBe("Every change goes to Zitadel when you apply.");
  });

  it("adds the cancel-only sentence, singular and plural", () => {
    expect(beforeLine(board())).toBe(
      "Apply sends 2 changes to Zitadel now. 2 wait for you to send. For 1, this only cancels a change that's waiting; nothing is sent.",
    );
    const two = plan([row("Ada"), row("K1", { delivery: "none", consequence: CANCEL }), row("K2", { delivery: "none", consequence: CANCEL })]);
    expect(beforeLine(two)).toBe(
      "Every change goes to Zitadel when you apply. For 2, this only cancels changes that are waiting; nothing is sent.",
    );
  });

  it("says there is nothing to apply when no row acts, and nothing for a flat plan", () => {
    expect(beforeLine(plan([row("Dev", { effect: "no_change", delivery: undefined })]))).toBe(NOTHING_TO_APPLY);
    expect(beforeLine(plan([row("Ada", { delivery: undefined })], { op: "edit_mapping" }))).toBeUndefined();
    expect(beforeLine({ ...board(), applied: true })).toBeUndefined();
  });

  it("never says what the retired note said", () => {
    expect(beforeLine(board())).not.toMatch(/No change for|refused\.|leaves those as they are|Every selected/);
  });
});

describe("applyLabel", () => {
  it("carries the number, and the cancels in the cancel step's shape", () => {
    expect(applyLabel(plan([row("Ada"), row("Bo")]), ["person", "people"])).toBe("Apply to 2 people");
    expect(applyLabel(board(), ["person", "people"])).toBe("Cancel 2 and apply to 5 people");
    expect(applyLabel(plan([row("K", { delivery: "none", consequence: CANCEL })]), ["person", "people"])).toBe("Cancel 1 and apply to 1 person");
    expect(applyLabel({ ...board(), summary: undefined } as unknown as BulkPlan, ["person", "people"])).toBe("Apply the change");
  });

  it("nobody to apply to is plain Apply; the held button's reason says why", () => {
    expect(applyLabel(plan([row("Dev", { effect: "no_change", delivery: undefined })]), ["person", "people"])).toBe("Apply");
  });
});

describe("resultOutcome", () => {
  const done = (outcomes: BulkOutcome[], op: BulkPlan["op"] = "assign_bundle") => resultOutcome(plan(outcomes, { applied: true, op }))!;

  it("all reached: Given to N people, All N are in Zitadel now, nothing for you to do", () => {
    const r = done([row("Ada", { effect: "applied" }), row("Bo", { effect: "applied" })]);
    expect(r.word).toBe("Given to 2 people");
    expect(r.outcome).toMatchObject({ kind: "applied", message: "All 2 are in Zitadel now.", next: NOTHING_TO_DO });
  });

  it("some wait: one clause per fold, ending on the Send link", () => {
    const r = done([row("Ada", { effect: "applied" }), row("Bo", { effect: "queued", wait_reason: "role_waits" })]);
    expect(r.outcome.message).toBe("1 is in Zitadel now. 1 waits for you to send.");
    expect(waitsForSend(r.outcome)).toBe(true);
  });

  it("all wait", () => {
    const r = done([row("Ada", { effect: "queued", wait_reason: "role_waits" }), row("Bo", { effect: "queued", wait_reason: "role_waits" })]);
    expect(r.outcome.message).toBe("All 2 wait for you to send. Nothing has reached Zitadel yet.");
  });

  it("some not answered: Syndra sends them again, and when, as the one step", () => {
    const retry = { state: "retrying" as const, try: 2, of: 6, next_try: "2999-01-01T14:05:00Z" };
    const r = done([
      row("Ada", { effect: "applied" }),
      row("Bo", { effect: "queued", wait_reason: "halted:retrying", retry }),
      row("Cy", { effect: "queued", wait_reason: "halted:retrying", retry }),
    ]);
    expect(r.outcome.message).toBe("1 is in Zitadel now. 2 aren't in Zitadel yet; Syndra sends them again on its own.");
    expect(r.outcome).toMatchObject({ kind: "queued", sendsItself: true });
    expect(r.outcome.next).toMatch(/^Nothing for you to do\. Syndra sends them again at .* and says on Home if it stops sending them\.$/);
  });

  it("some refused: of N in the word, the refused clause last, and the step on each row", () => {
    const r = done([row("Ada", { effect: "applied" }), row("Sam", { effect: "failed", wait_reason: "refused:not_found" })], "remove_bundle");
    expect(r.word).toBe("Revoked from 1 of 2 people");
    expect(r.outcome).toMatchObject({ kind: "failed", recorded: true, message: "1 is in Zitadel now. Zitadel refused 1; the row says why.", next: REFUSED_ROWS_NEXT });
  });

  it("composes all five folds left to right in fold order", () => {
    const retry = { state: "retrying" as const, try: 2, of: 6 };
    const r = done([
      row("A", { effect: "applied" }),
      row("B", { effect: "applied" }),
      row("C", { effect: "queued", wait_reason: "role_waits" }),
      row("D", { effect: "queued", wait_reason: "halted:retrying", retry }),
      row("E", { effect: "failed", wait_reason: "refused:invalid" }),
      row("F", { effect: "failed", wait_reason: "refused:invalid" }),
      row("G", { effect: "no_change", delivery: undefined }),
    ]);
    expect(r.outcome.message).toBe(
      "2 are in Zitadel now. 1 waits for you to send. 1 isn't in Zitadel yet; Syndra sends it again on its own. Zitadel refused 2; each row says why.",
    );
    expect(r.word).toBe("Given to 4 of 6 people");
  });

  it("never counts automatic rules apart, and never says the act that went has not", () => {
    // The row's roles say its own role went and the rule's two wait.
    const listed = done([row("Ada", { effect: "applied", rule_waiting: 2, delivery: "waits_for_you", goes: ["Member"], waits: ["A", "B"] })]);
    expect(listed.outcome.message).toBe("1 is in Zitadel now. 2 wait for you to send.");
    // Without its roles, only what waits is counted.
    const r = done([row("Ada", { effect: "applied", rule_waiting: 2, delivery: "waits_for_you" })]);
    expect(r.outcome.message).toBe("All 2 wait for you to send. Nothing has reached Zitadel yet.");
    expect(r.outcome.message).not.toMatch(/automatic rules/);
  });

  // Browser pass, 8 Nov: the same three people after Apply. The result said
  // "All 3 wait for you to send. Nothing has reached Zitadel yet." while 6 had.
  it("counts changes per fold: a person waiting on one role had the rest sent", () => {
    const basic = (name: string, over: Partial<BulkOutcome> = {}) =>
      row(name, {
        effect: "queued",
        wait_reason: "role_waits",
        delivery: "waits_for_you",
        goes: ["Audio-Dash", "Homeassistant"],
        waits: ["Navidrome community"],
        ...over,
      });
    const r = done([basic("Avani"), basic("Bo"), basic("Cy")]);
    expect(r.word).toBe("Given to 3 people");
    expect(r.outcome).toMatchObject({ kind: "queued", message: "6 are in Zitadel now. 3 wait for you to send." });
    expect(waitsForSend(r.outcome)).toBe(true);
    // The roles sent with it left to a retry are not claimed as in Zitadel.
    const retry = { state: "retrying" as const, try: 2, of: 6 };
    const halted = done([basic("Avani"), basic("Bo", { retry })]);
    expect(halted.outcome.message).toBe("2 are in Zitadel now. 2 wait for you to send. 2 aren't in Zitadel yet; Syndra sends them again on its own.");
    // A halt with no retry: its unsent roles are not claimed as in Zitadel, and the step stays Send.
    const stopped = done([basic("Avani"), basic("Bo", { unsent: 2 })]);
    expect(stopped.outcome.message).toBe("2 are in Zitadel now. 2 wait for you to send. 2 didn't go to Zitadel.");
    expect(waitsForSend(stopped.outcome)).toBe(true);
    // Every role waits: the all-wait shape still holds when it is true.
    const all = done([basic("Avani", { goes: [] }), basic("Bo", { goes: [] })]);
    expect(all.outcome.message).toBe("All 2 wait for you to send. Nothing has reached Zitadel yet.");
  });

  it("is null for a flat plan or a plan not yet applied", () => {
    expect(resultOutcome(board())).toBeNull();
    expect(resultOutcome(plan([row("Ada", { delivery: undefined, effect: "applied" })], { applied: true, op: "edit_mapping" }))).toBeNull();
  });
});

describe("refusedSeat", () => {
  it("says the refusal and its one true step, by direction", () => {
    expect(refusedSeat(row("x", { wait_reason: "refused:not_allowed" }), "assign_role")).toBe(
      "Zitadel refused this: Syndra's service account isn't allowed to make this change. Ask whoever runs Zitadel to let Syndra's service account change grants. Open their page to see what they hold now, then give it again once the reason is fixed.",
    );
    expect(refusedSeat(row("x", { wait_reason: "refused:not_found" }), "remove_role")).toMatch(/It shows in Drift after Syndra's next check/);
  });
});
