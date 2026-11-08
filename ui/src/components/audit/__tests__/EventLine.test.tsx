// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { EventSentence } from "@/components/audit/EventLine";
import type { AuditEntry } from "@/lib/queries/useAudit";

const NAMES: Record<string, string> = { u1: "Gaurav Prem Singh", op1: "Kanishk Singh", op2: "Asha Rao" };
vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{NAMES[id] ?? id}</span>,
  BundleName: ({ id }: { id: string }) => <span>{id === "b1" ? "Community · Basic" : id}</span>,
  ProjectName: ({ id }: { id: string }) => <span>{id === "p1" ? "Network storage" : id}</span>,
}));

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({ value: NAMES[id] ? { display_name: NAMES[id] } : undefined, resolved: true }),
    resolveRole: (_p: string, k: string) => ({ value: k === "member" ? { display_name: "Member" } : undefined, resolved: true }),
    resolveProject: () => ({ value: undefined, resolved: true }),
    resolveBundle: () => ({ value: undefined, resolved: true }),
  }),
}));

function entry(over: Partial<AuditEntry>): AuditEntry {
  return {
    id: "a1",
    actor_id: "actor",
    target_id: "u1",
    action: "bundle.assigned",
    resource_id: "",
    created_at: "2026-09-19T14:43:00Z",
    ...over,
  } as AuditEntry;
}

/**
 * The same event used to render two ways. `target_id` is a person on most
 * actions and a BUNDLE on a bundle edit, and only the Audit page knew that —
 * the home page's Lately panel handed every id to the person resolver, so one
 * screen said "Community · Basic" and the other said "Unknown account
 * 9e93893d-…" about the same row.
 */
describe("EventSentence", () => {
  it("names a bundle through the bundle resolver, not the person one", () => {
    render(<EventSentence entry={entry({ action: "bundle.role_removed", target_id: "b1" })} />);
    expect(screen.getByText(/Community · Basic/)).toBeTruthy();
    expect(document.body.textContent).toContain("Removed a role from the");
    expect(document.body.textContent).toContain("bundle");
  });

  it("names a person through the person resolver", () => {
    render(<EventSentence entry={entry({ action: "bundle.assigned", target_id: "u1" })} />);
    expect(document.body.textContent).toBe("Gave Gaurav Prem Singh a bundle");
  });

  // The thing acted on belongs INSIDE the sentence. "Removed a role from a
  // bundle — Community · Basic" makes the reader do the joining and reads as
  // two facts about one event.
  it("puts the named thing inside the sentence, not after a dash", () => {
    render(<EventSentence entry={entry({ action: "bundle.role_removed", target_id: "b1" })} />);
    expect(document.body.textContent).not.toContain("—");
  });

  it("falls back to the bare verb when the row names nothing", () => {
    render(<EventSentence entry={entry({ action: "bundle.created", target_id: "-" })} />);
    expect(document.body.textContent).toBe("Created a bundle");
  });

  // An action nobody has written a template for still has to name its target;
  // dropping it would be worse than the colon.
  it("joins the target with a colon, never a dash, for an action with no template", () => {
    render(<EventSentence entry={entry({ action: "bundle.updated", target_id: "u1" })} />);
    expect(document.body.textContent).not.toContain("—");
    expect(document.body.textContent).toContain(": Gaurav Prem Singh");
  });

  it("names a rule or a retired bundle in words, never by its handle", () => {
    render(<EventSentence entry={entry({ action: "mapping_rule.created", target_id: "0123abcd-0000-0000-0000-000000000000" })} />);
    expect(document.body.textContent).toContain("an automatic rule");
    expect(document.body.textContent).not.toMatch(/R_[0-9a-f]{4}/);
  });

  it("marks a destructive action, and only that", () => {
    const { container: gone } = render(
      <EventSentence entry={entry({ action: "bundle.role_removed", target_id: "b1" })} />,
    );
    expect(gone.querySelector(".text-danger-text")).toBeTruthy();
    const { container: made } = render(
      <EventSentence entry={entry({ action: "bundle.assigned", target_id: "u1" })} />,
    );
    expect(made.querySelector(".text-danger-text")).toBeNull();
  });

  // A-04 §14.8: a cancellation is in Change history, by whoever's action caused
  // it, worded the operator's way. Syndra's own say so, and that nobody was asked.
  it("an operator's cancellation says what was cancelled and for whom", () => {
    render(<EventSentence entry={entry({ action: "propagation.cancelled_unsent", actor_id: "op1", target_id: "u1" })} />);
    expect(document.body.textContent).toBe(
      "Cancelled a change that was waiting for you to send, for Gaurav Prem Singh",
    );
    expect(document.body.textContent).not.toMatch(/waiting to be sent|makerspace staff to send/);
  });

  it("Syndra's own cancellation says Syndra, and that nobody was asked", () => {
    render(<EventSentence entry={entry({ action: "propagation.cancelled_unsent", actor_id: "system:scheduler", target_id: "u1" })} />);
    expect(document.body.textContent).toContain(
      "Syndra cancelled a change that was waiting for you to send, for Gaurav Prem Singh",
    );
    expect(document.body.textContent).toContain("Nobody was asked.");
  });

  // A-04 §14.8: with `details` the entry says what the change was, who queued it, when,
  // why, and which act cancelled it.
  // `said` is what the backend reads from the reason (db.Said).
  const cancelled = (over: Record<string, unknown> = {}, cause: Record<string, unknown> = { action: "grant", via: "direct" }, actor = "op1") =>
    entry({
      action: "propagation.cancelled_unsent",
      actor_id: actor,
      target_id: "u1",
      created_at: new Date().toISOString(),
      said: ("reason" in over ? over.reason : "Left the lab") as string | undefined,
      details: {
        cancelled: {
          kind: "remove", project_id: "p1", role_key: "member", user_id: "u1",
          initiated_by: "op2", queued_at: new Date(Date.now() - 2 * 3600_000).toISOString(),
          reason: "Left the lab", ...over,
        },
        cause,
      },
    });

  it("with details, says the change, who queued it and when, the reason, and the act that cancelled it", () => {
    render(<EventSentence entry={cancelled()} />);
    const t = document.body.textContent!;
    expect(t).toContain("Cancelled a change that was waiting for you to send");
    expect(t).not.toContain("Kanishk Singh cancelled");
    expect(t).toContain("Removing Network storage · Member from Gaurav Prem Singh");
    expect(t).toContain("Queued by Asha Rao · 2 hours ago");
    expect(t).toContain("“Left the lab”");
    expect(t).toContain("cancelled when Kanishk gave it back");
    expect(t).not.toMatch(/waiting to be sent/);
  });

  it("words a give, and each act that can cancel", () => {
    const { unmount } = render(<EventSentence entry={cancelled({ kind: "give" })} />);
    expect(document.body.textContent).toContain("Giving Network storage · Member to Gaurav Prem Singh");
    unmount();
    const a = render(<EventSentence entry={cancelled({}, { action: "remove", via: "direct" })} />);
    expect(document.body.textContent).toContain("cancelled when Kanishk removed it");
    a.unmount();
    render(<EventSentence entry={cancelled({}, { action: "approve", via: "direct" })} />);
    expect(document.body.textContent).toContain("cancelled when Kanishk approved the request");
  });

  it("without a reason, says so by first name, never a pronoun", () => {
    render(<EventSentence entry={cancelled({ reason: undefined })} />);
    expect(document.body.textContent).toContain("Asha gave no reason.");
    expect(document.body.textContent).not.toMatch(/\b(he|she|they|his|her|their)\b/i);
  });

  it("Syndra's own, with details, says Nobody was asked and names no act", () => {
    render(<EventSentence entry={cancelled({ initiated_by: undefined, reason: undefined }, { action: "remove", via: "rule" }, "system")} />);
    const t = document.body.textContent!;
    expect(t).toContain("Syndra cancelled a change that was waiting for you to send");
    expect(t).toContain("Removing Network storage · Member from Gaurav Prem Singh");
    expect(t).toContain("Nobody was asked.");
    expect(t).not.toContain("cancelled when");
  });
});

/**
 * The operator's reason is said under the event, quoted, once (task 1.8a): on
 * the Audit page, a person's Activity and Home, which all draw EventSentence.
 * Only the backend's `said` is quoted, never `details.reason`, which can hold
 * Syndra's own labels ("Direct access removal"); nothing when none was given.
 */
describe("EventSentence · the reason", () => {
  it.each([
    ["bundle.assigned", "Gave Gaurav Prem Singh a bundle"],
    ["bundle.unassigned", "Removed Gaurav Prem Singh from a bundle"],
    ["direct_grant.upserted", "Gave Gaurav Prem Singh a role"],
    ["direct_grant.removed", "Revoked Gaurav Prem Singh's direct access by hand"],
    ["direct_grant.revoked", "Revoked Gaurav Prem Singh's direct access"],
  ])("quotes it under %s", (action, sentence) => {
    render(<EventSentence entry={entry({ action, details: { reason: "term start" }, said: "term start" })} />);
    expect(document.body.textContent).toBe(`${sentence}“term start”`);
    expect(screen.getAllByText("“term start”")).toHaveLength(1);
  });

  it("says nothing when none was given", () => {
    render(<EventSentence entry={entry({ action: "bundle.assigned" })} />);
    expect(document.body.textContent).toBe("Gave Gaurav Prem Singh a bundle");
  });

  // A removal recorded on 8 Nov with Syndra's label: the backend says nothing was said.
  it("never quotes a label of Syndra's kept as the reason", () => {
    render(<EventSentence entry={entry({ action: "direct_grant.removed", details: { reason: "Direct access removal" } })} />);
    expect(document.body.textContent).toBe("Revoked Gaurav Prem Singh's direct access by hand");
  });

  it("never quotes another event's details as a reason", () => {
    render(<EventSentence entry={entry({ action: "action_key.partly_replaced", target_id: "-", details: { reason: "Zitadel replaced one key" }, said: "Zitadel replaced one key" })} />);
    expect(document.body.textContent).not.toContain("“");
  });

  it("a cancellation quotes only what was said", () => {
    render(
      <EventSentence
        entry={entry({
          action: "propagation.cancelled_unsent",
          actor_id: "op1",
          details: {
            cancelled: { kind: "give", project_id: "p1", role_key: "member", user_id: "u1", initiated_by: "op2", queued_at: new Date().toISOString(), reason: "Extended from Home" },
            cause: { action: "remove", via: "direct" },
          },
        })}
      />,
    );
    expect(document.body.textContent).not.toContain("“");
    expect(document.body.textContent).toContain("Asha gave no reason.");
  });
});
