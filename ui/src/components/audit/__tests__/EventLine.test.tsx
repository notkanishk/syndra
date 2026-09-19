// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { EventSentence } from "@/components/audit/EventLine";
import type { AuditEntry } from "@/lib/queries/useAudit";

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id === "u1" ? "Gurasheesh Paul Singh" : id}</span>,
  BundleName: ({ id }: { id: string }) => <span>{id === "b1" ? "Community · Basic" : id}</span>,
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
    expect(document.body.textContent).toBe("Gave Gurasheesh Paul Singh a bundle");
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
  // dropping it would be worse than the dash.
  it("keeps the dash for an action with no template", () => {
    render(<EventSentence entry={entry({ action: "bundle.updated", target_id: "u1" })} />);
    expect(document.body.textContent).toContain("—");
    expect(document.body.textContent).toContain("Gurasheesh Paul Singh");
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
});
