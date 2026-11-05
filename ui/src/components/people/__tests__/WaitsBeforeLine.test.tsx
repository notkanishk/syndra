// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RemovalDialog } from "@/components/people/RemovalDialog";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";

import { waitsBeforeLine } from "@/components/states/useWaitsBeforeLine";

/**
 * A-07 §9: while Zitadel isn't answering, a person's page, Requests and Expiring stay on and
 * their dialogs say, before the act, what happens to the change. Their buttons stay live.
 *
 * Exact per op (owner, 5 Nov 2026): it used to say every change "will wait for you to send
 * it", which was false for a removal (revocations go on their own) and is now false for a
 * give too (a give that finds Zitadel down is retried on its own). Only a role marked to
 * wait still waits for Send.
 */

const LINE = waitsBeforeLine("remove");

describe("the line itself", () => {
  it("never tells anyone to send what goes on its own", () => {
    for (const op of ["give", "remove", "change"] as const) {
      const line = waitsBeforeLine(op);
      expect(line).toMatch(/sends it on its own once Zitadel answers/);
      expect(line).toMatch(/nothing for you to send/);
      expect(line).not.toMatch(/will wait for you to send it once/);
    }
    expect(waitsBeforeLine("give")).toMatch(/records the give/);
    expect(waitsBeforeLine("remove")).toMatch(/records the removal/);
  });
});

vi.mock("@/components/states/DegradedBanner", () => ({ useDegraded: () => true }));
vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRemoveDirectGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

describe("a revoke dialog while Zitadel is not answering", () => {
  it("says the change will wait, and leaves Revoke pressable", () => {
    render(
      <RemovalDialog
        removal={{
          projectId: "p1",
          projectName: "Audio-Dash",
          roleKey: "community",
          sources: [{ kind: "direct", description: "" }],
          grantId: "g1",
          grantsResolved: true,
        }}
        userId="u1"
        userName="Sneha Yeole"
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(LINE)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Revoke/ })).toBeEnabled();
  });

  it("says it for a bundle removal too, ahead of what else the bundle carries", () => {
    render(
      <RemovalDialog
        removal={{
          projectId: "p1",
          projectName: "Audio-Dash",
          roleKey: "community",
          sources: [{ kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "" }],
        }}
        userId="u1"
        userName="Sneha Yeole"
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(LINE)).toBeInTheDocument();
  });
});

describe("a rehearsal dialog whose caller passes the waiting line", () => {
  const props = {
    title: "Approve requests",
    lede: "",
    noun: ["request", "requests"] as [string, string],
    onRehearse: vi.fn(),
    onApply: vi.fn(),
    onClose: vi.fn(),
  };

  it("says it before the preview, in the footer, and never when the caller passes none", () => {
    const { unmount } = render(<RehearsalDialog {...props} compose={<p>Compose</p>} waits={LINE} />);
    expect(screen.getByText(new RegExp(`^${LINE}`))).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Preview the change" })).toBeInTheDocument();
    unmount();

    render(<RehearsalDialog {...props} compose={<p>Compose</p>} />);
    expect(screen.queryByText(new RegExp(LINE))).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
  });
});
