// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it, vi } from "vitest";

import * as PlanReviewModule from "@/components/ui/PlanReview";
import * as RehearsalModule from "@/components/ui/RehearsalDialog";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import type { BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";

// A-21 §Q1 on screen: the plan in its folds, the before-line, the Apply
// button's words, and after Apply the same rows re-grouped in the true tense
// with one outcome that ends on one step.

const CANCEL = "Cancels a change waiting for you to send: the grant of Member in Laser Lab queued 2 Nov 2026 (wrong person).";

const row = (name: string, over: Partial<BulkOutcome> = {}): BulkOutcome => ({
  user_id: `id-${name}`,
  name,
  email: "",
  effect: "apply",
  detail: "Joins Safety.",
  consequence: "Gains Member in Laser Lab.",
  delivery: "straight_away",
  ...over,
});

const JULES = "Gains Member in Laser Lab and Trained in Laser Lab. Giving Trained in Laser Lab waits for you to send.";

const rehearsed: BulkPlan = {
  op: "assign_bundle",
  plan_id: "plan_1",
  applied: false,
  outcomes: [
    row("Omar", { consequence: `Gains Member in Laser Lab. ${CANCEL}` }),
    row("Jules", { delivery: "waits_for_you", waits: ["Trained in Laser Lab"], consequence: JULES }),
    row("Kabir", {
      delivery: "none",
      detail: "Leaves Safety.",
      consequence: `Cancels giving Member in Laser Lab, which was waiting for you to send, so nothing is sent to Zitadel. ${CANCEL}`,
    }),
    row("Dev", { effect: "no_change", delivery: undefined, detail: "Already in Safety.", consequence: undefined }),
    row("Sam", { effect: "blocked", delivery: undefined, detail: "Account is departed.", consequence: undefined }),
  ],
  summary: { total: 5, apply: 3, no_change: 1, blocked: 1, failed: 0, succeeded: 0, queued: 0 },
};

// After Apply the settle keeps every row's words and sets only the state.
const result: BulkPlan = {
  ...rehearsed,
  plan_id: undefined,
  applied: true,
  outcomes: [
    { ...rehearsed.outcomes[0], effect: "failed", wait_reason: "refused:not_found" },
    { ...rehearsed.outcomes[1], effect: "queued", wait_reason: "role_waits", person: "person-jules" },
    { ...rehearsed.outcomes[2], effect: "applied" },
    rehearsed.outcomes[3],
    rehearsed.outcomes[4],
  ],
  summary: { total: 5, apply: 0, no_change: 1, blocked: 1, failed: 1, succeeded: 1, queued: 1 },
};

function open() {
  return render(
    <RehearsalDialog
      title="Give the Safety bundle"
      lede=""
      noun={["person", "people"]}
      onRehearse={vi.fn(async () => rehearsed)}
      onApply={vi.fn(async () => result)}
      onClose={() => {}}
    />,
  );
}

describe("a grouped plan, before Apply", () => {
  it("draws the folds with their counts, the rows with the planner's words, and a badge only under Left as they are", async () => {
    open();
    const go = await screen.findByRole("group", { name: "Straight away, 2 people" });
    const wait = screen.getByRole("group", { name: "Will wait for you to send, 1 person" });
    const left = screen.getByRole("group", { name: "Left as they are, 2 people" });
    expect(within(wait).getByText("Because Trained in Laser Lab always waits for you.")).toBeInTheDocument();
    expect(within(wait).getByText(new RegExp(JULES.replace(/\./g, "\\.")))).toBeInTheDocument();
    expect(within(left).getByText("No change")).toBeInTheDocument();
    expect(within(left).getByText("Refused")).toBeInTheDocument();
    expect(within(go).queryByText(/Will change|No change|Refused/)).toBeNull();
    expect(screen.queryByText("Will change")).toBeNull();
    // Kabir only cancels: his one sentence is the whole row; the note below names the cancel.
    expect(within(go).getByText(/Cancels giving Member in Laser Lab, which was waiting for you to send/).textContent).not.toMatch(/Cancels a change/);
    expect(screen.getByRole("note").textContent).toContain("Cancels 2 changes that are waiting for you to send:");
  });

  it("says the before-line and puts the cancels on the Apply button", async () => {
    open();
    expect(
      await screen.findByText(
        "Apply sends 1 change to Zitadel now. 1 waits for you to send. For 1, this only cancels a change that's waiting; nothing is sent.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel 2 and apply to 3 people" })).toBeEnabled();
  });

  it("folds a group from its heading", async () => {
    open();
    const go = await screen.findByRole("group", { name: "Straight away, 2 people" });
    fireEvent.click(within(go).getByRole("button", { expanded: true }));
    expect(within(go).queryByText("Omar")).toBeNull();
    expect(within(go).getByRole("button", { expanded: false })).toBeInTheDocument();
  });
});

describe("a grouped plan, after Apply", () => {
  async function apply() {
    open();
    fireEvent.click(await screen.findByRole("button", { name: "Cancel 2 and apply to 3 people" }));
    await screen.findByRole("button", { name: "Close" });
  }

  it("re-groups the same rows in the true tense, each keeping its plan words, with its seat", async () => {
    await apply();
    expect(screen.getByRole("group", { name: "Done, 1 person" })).toBeInTheDocument();
    const waiting = screen.getByRole("group", { name: "Waiting for you to send, 1 person" });
    const refused = screen.getByRole("group", { name: "Zitadel refused, 1 person" });
    expect(screen.getByRole("group", { name: "Left as they were, 2 people" })).toBeInTheDocument();
    // The words survive Apply.
    expect(within(waiting).getByText(new RegExp(JULES.replace(/\./g, "\\.")))).toBeInTheDocument();
    // The row's seat: the link to send it, narrowed to the person.
    const links = within(waiting).getAllByRole("link", { name: /Waiting for you to send/ });
    expect(links.map((a) => a.getAttribute("href"))).toEqual([
      "/governance/pending",
      "/governance/pending?person=person-jules",
    ]);
    expect(within(refused).getByText(/^Zitadel refused this: it no longer has this person, project or role\./)).toBeInTheDocument();
    expect(within(refused).getByText(/Gains Member in Laser Lab\./)).toBeInTheDocument();
  });

  it("composes one clause per fold and ends on one step", async () => {
    await apply();
    const block = screen.getByRole("alert");
    expect(block.textContent).toContain("Given to 2 of 3 people");
    expect(block.textContent).toContain("1 is in Zitadel now. 1 waits for you to send. Zitadel refused 1; the row says why.");
    expect(block.textContent).toContain("Follow the step on each row Zitadel refused, below.");
    expect(block.textContent).not.toMatch(/automatic rules|Recorded in Syndra/);
  });
});

describe("retired with A-21", () => {
  it("planNote and the separate automatic-rule count are gone", () => {
    expect("planNote" in PlanReviewModule).toBe(false);
    expect("ruleWaiting" in RehearsalModule).toBe(false);
    const root = join(__dirname, "..", "..", "..");
    for (const file of ["components/ui/PlanReview.tsx", "components/ui/RehearsalDialog.tsx", "lib/plan-groups.ts"]) {
      const source = readFileSync(join(root, file), "utf8");
      expect(source, file).not.toMatch(/planNote|ruleWaiting|from automatic rules|Syndra leaves those as they are/);
    }
  });
});
