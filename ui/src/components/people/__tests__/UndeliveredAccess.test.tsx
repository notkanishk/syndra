// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RemovalDialog } from "@/components/people/RemovalDialog";

/**
 * Recorded is not delivered, and the screens must not say otherwise.
 *
 * Assigning a bundle in manual mode writes the assignment and queues the
 * grants; nothing reaches Zitadel until somebody confirms Pending changes. For
 * the whole of that window the person page rendered those roles exactly like
 * roles delivered a month ago, and the removal dialog offered to take away
 * access that had never been given — "THEY WILL LOSE ... no other source gives
 * it" about a grant sitting in the outbox.
 *
 * The operator who hit it read the page as confirmation the work was done.
 */

const removed = vi.hoisted(() => ({
  calls: 0,
  enqueued: 0,
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => ({
    mutateAsync: vi.fn(async () => {
      removed.calls += 1;
      return { cascade: { enqueued: removed.enqueued, mode: "manual" } };
    }),
    isPending: false,
  }),
}));
vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRemoveDirectGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

const bundleSource = (queued: boolean) => ({
  projectId: "p-audio",
  projectName: "Audio-Dash",
  roleKey: "community",
  sources: [
    { kind: "bundle", bundle_id: "b1", bundle_name: "Ops Admin", description: "", queued },
  ],
});

describe("removing a bundle whose grants were never sent", () => {
  it("does not claim they lose access they never received", () => {
    render(
      <RemovalDialog
        removal={bundleSource(true)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/Nothing to take back/i)).toBeInTheDocument();
    expect(screen.getByText(/recorded, never sent/i)).toBeInTheDocument();
    // The two sentences that were false, verbatim.
    expect(screen.queryByText(/They will lose/i)).toBeNull();
    expect(screen.queryByText(/no other source gives it/i)).toBeNull();
  });

  it("says the queue empties rather than filling with reversals", () => {
    render(
      <RemovalDialog
        removal={bundleSource(true)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/Pending changes will be empty afterwards/i)).toBeInTheDocument();
  });

  it("still says what a delivered bundle takes away", () => {
    render(
      <RemovalDialog
        removal={bundleSource(false)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/They will lose/i)).toBeInTheDocument();
    expect(screen.getByText(/no other source gives it/i)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing to take back/i)).toBeNull();
  });
});

describe("revoking a direct grant that was never sent", () => {
  const directSource = (queued: boolean) => ({
    projectId: "p-audio",
    projectName: "Audio-Dash",
    roleKey: "community",
    grantId: "g1",
    grantsResolved: true,
    sources: [{ kind: "direct", description: "", queued }],
  });

  it("says there is nothing at the door to close", () => {
    render(
      <RemovalDialog
        removal={directSource(true)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/Nothing to take back/i)).toBeInTheDocument();
    expect(screen.queryByText(/will lose this role/i)).toBeNull();
  });

  it("keeps the real consequence when the grant has been delivered", () => {
    render(
      <RemovalDialog
        removal={directSource(false)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/will lose this role/i)).toBeInTheDocument();
  });
});

describe("the action, once it has run", () => {
  it("names withdrawal rather than removal while nothing has been sent", () => {
    render(
      <RemovalDialog
        removal={bundleSource(true)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    const act = screen.getByRole("button", { name: /Withdraw the queued change/ });
    // Red is a promise about consequence, and this one takes nothing away.
    expect(act.className).not.toMatch(/danger/);
    expect(screen.queryByRole("button", { name: "Remove bundle" })).toBeNull();
  });

  it("cannot be pressed a second time", async () => {
    removed.calls = 0;
    removed.enqueued = 0;
    render(
      <RemovalDialog
        removal={bundleSource(true)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Withdraw the queued change/ }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument());

    // The dialog stays open to report what happened, and a destructive
    // confirm left armed inside it queues another cascade on the next click —
    // which is what a person does when the dialog does not close.
    expect(screen.queryByRole("button", { name: /Withdraw the queued change/ })).toBeNull();
    expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument();
    expect(removed.calls).toBe(1);
  });

  it("reports the queue it actually left behind", async () => {
    removed.calls = 0;
    removed.enqueued = 0;
    render(
      <RemovalDialog
        removal={bundleSource(true)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Withdraw the queued change/ }));
    await waitFor(() => expect(screen.getByText(/were withdrawn/i)).toBeInTheDocument());
    // The sentence that contradicted the dialog one centimetre above it.
    expect(screen.queryByText(/are revoked once you send/i)).toBeNull();
  });

  it("still says what a delivered removal queued", async () => {
    removed.calls = 0;
    removed.enqueued = 3;
    render(
      <RemovalDialog
        removal={bundleSource(false)}
        userId="shikha"
        userName="Shikha Yadav"
        onClose={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Remove bundle" }));
    await waitFor(() => expect(screen.getByText(/are revoked once you send/i)).toBeInTheDocument());
  });
});
