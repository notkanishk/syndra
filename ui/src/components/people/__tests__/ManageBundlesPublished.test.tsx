// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Zitadel answering: what the dialogs say before the act is not under test here.
vi.mock("@/components/states/useWaitsBeforeLine", () => ({ useWaitsBeforeLine: () => undefined }));

import { ManageBundles } from "@/components/people/ManageBundles";

/**
 * What the assign panel promises has to be what the assign gives.
 *
 * An assignment pins the latest PUBLISHED version of a bundle and nothing else.
 * This panel asked for the working copy, so it listed every unpublished edit as
 * a role the person was about to receive — the operator saw the roles they had
 * just added promised here and called "unpublished" on the bundle screen, and
 * the apply then granted neither of the sets they had been shown.
 */

const state = vi.hoisted(() => ({
  roleQueries: vi.fn(),
  assignAsync: vi.fn().mockResolvedValue(undefined),
  whatIf: vi.fn(),
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({
    data: [
      {
        id: "b1",
        name: "Lab Tech",
        latest_version: 2,
        unpublished_changes: 2,
        holder_count: 0,
      },
    ],
  }),
  useBundleRolesByBundle: (...args: unknown[]) => state.roleQueries(...args),
  useRemoveBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

// The rules are followed by the backend (the what-if), never by this panel.
vi.mock("@/lib/queries/useInsights", () => ({
  useInsightsWhatIf: (...args: unknown[]) => state.whatIf(...args),
}));

vi.mock("@/lib/queries/useUsers", () => ({
  useAssignBundle: () => ({ mutateAsync: state.assignAsync, isPending: false }),
  useUserAccess: () => ({ data: { projects: [] } }),
}));

vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
}));

beforeEach(() => {
  state.roleQueries = vi.fn().mockReturnValue({
    byId: {
      b1: [{ bundle_id: "b1", zitadel_project_id: "p1", zitadel_role_key: "published-role" }],
    },
    allLoaded: true,
  });
  state.assignAsync = vi.fn().mockResolvedValue(undefined);
  state.whatIf = vi.fn().mockReturnValue({ data: undefined, error: null });
});

function open() {
  return render(
    <ManageBundles userId="u1" userName="Ada" assigned={[]} open onClose={vi.fn()} />,
  );
}

describe("ManageBundles", () => {
  it("asks for the published version, not the working copy", () => {
    open();

    expect(state.roleQueries).toHaveBeenCalledWith(["b1"], { published: true });
  });

  it("previews the roles the assignment will actually grant", () => {
    open();
    // Tick the bundle to select it for preview.
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));

    expect(screen.getByText("published-role")).toBeInTheDocument();
  });

  // The panel is right and the bundle screen shows a different set, so an
  // operator who has just added those roles would read their absence here as
  // this panel being stale rather than as the truth about what a pin means.
  it("says the unpublished edits are not part of the assignment", () => {
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));

    expect(screen.getByText(/2 unpublished changes/)).toBeInTheDocument();
    expect(screen.getByText(/an assignment gives v2/i)).toBeInTheDocument();
  });

  // The pre-apply hint used to promise "Queues for confirmation" every time
  // something was staged, which the apply outcome does not always keep: a
  // change that reaches nobody's effective access is reported as `applied`,
  // not queued. The hint must hold in both cases.
  it("hints at what the button does without promising a queue that may not happen", () => {
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));

    expect(screen.getByText("Recorded in Syndra first, not sent yet.")).toBeInTheDocument();
    expect(screen.queryByText(/queues for confirmation/i)).not.toBeInTheDocument();
  });

  it("reports Recorded, not queued, when the assignment reaches nobody yet", async () => {
    state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 0 } });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    // The act's own word, never the "Applied" fallback.
    expect(await screen.findByText("Recorded")).toBeInTheDocument();
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Waiting for you to send/ })).not.toBeInTheDocument();
    expect(screen.getByText(/nothing is waiting for you to send/i)).toBeInTheDocument();
  });

  // Final verification N1: a cascade Zitadel refused is said as a refusal, with
  // its cause and step; it never waits, and Send is not the step.
  it("says Zitadel refused this, with why and what to do, when it did", async () => {
    state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 1, wait_reason: "refused:not_allowed" } });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    expect(await screen.findByText("Refused by Zitadel")).toBeInTheDocument();
    expect(screen.getByText(/Zitadel refused this: Syndra's service account isn't allowed/)).toBeInTheDocument();
    expect(screen.getByText(/Ask whoever runs Zitadel/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Waiting for you to send/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/Nothing was changed/)).not.toBeInTheDocument();
  });

  it("says a cascade the server sent went, never that it waits", async () => {
    state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 2 } });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    expect(await screen.findByText(/Sent to Zitadel: 2 changes/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Waiting for you to send/ })).not.toBeInTheDocument();
  });

  it("reports the change waiting, and where to send it, when it enqueues a cascade", async () => {
    state.assignAsync = vi.fn().mockResolvedValue({ cascade: { enqueued: 2, waiting: 2, wait_reason: "role_waits" } });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));
    fireEvent.click(screen.getByRole("button", { name: "Apply 1 change" }));

    expect(await screen.findByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute(
      "href",
      "/governance/pending",
    );
    expect(screen.getByText(/2 changes wait under pending changes/i)).toBeInTheDocument();
  });

  it("lists every role the automatic rules add, from the backend, and what is already held", () => {
    state.whatIf = vi.fn().mockReturnValue({
      data: {
        gains: [
          {
            user_id: "u1",
            name: "Ada",
            roles: [
              { project_id: "p2", role_key: "chained-one" },
              { project_id: "p3", role_key: "chained-two" },
            ],
          },
        ],
        losses: [],
        unchanged: 0,
      },
      error: null,
    });
    open();
    fireEvent.click(screen.getByRole("checkbox", { name: /Lab Tech/ }));

    expect(state.whatIf).toHaveBeenLastCalledWith({ kind: "bundle_assigned", bundle_id: "b1", user_id: "u1" });
    // Both rule targets, not just the first matching rule's.
    expect(screen.getByText("chained-one")).toBeInTheDocument();
    expect(screen.getByText("chained-two")).toBeInTheDocument();
    // The bundle's own role gains nothing by the backend's answer: already held.
    expect(screen.getByText("already held, no change")).toBeInTheDocument();
  });
});
