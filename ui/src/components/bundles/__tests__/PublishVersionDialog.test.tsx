// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PublishVersionDialog } from "@/components/bundles/PublishVersionDialog";
import type { BundleDraft } from "@/lib/queries/useBundleVersions";

const state = vi.hoisted(() => ({
  rehearse: vi.fn(),
  apply: vi.fn(),
}));

vi.mock("@/lib/queries/useBundleVersions", () => ({
  useRehearsePublish: () => ({ mutateAsync: state.rehearse }),
  useApplyPublish: () => ({ mutateAsync: state.apply }),
  draftChangeCount: () => 1,
}));

vi.mock("@/components/names", () => ({
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
}));


const emptyPlan = {
  op: "publish_bundle_version",
  applied: false,
  outcomes: [],
  summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
};

function draft(overrides: Partial<BundleDraft> = {}): BundleDraft {
  return {
    bundle_id: "b1",
    latest_version: 2,
    next_version: 3,
    added: [{ bundle_id: "b1", zitadel_project_id: "p1", zitadel_role_key: "trained" }],
    removed: [],
    holder_count: 14,
    ...overrides,
  } as BundleDraft;
}

beforeEach(() => {
  state.rehearse = vi.fn().mockResolvedValue({ plan: emptyPlan, draft: draft() });
  state.apply = vi.fn().mockResolvedValue({ plan: { ...emptyPlan, applied: true } });
});

describe("PublishVersionDialog", () => {
  // The question the whole feature exists to ask. Neither answer is a default:
  // pre-selecting one would be the product making the decision.
  it("will not rehearse until the migrate question is answered", () => {
    render(
      <PublishVersionDialog bundleId="b1" name="Lab Tech" draft={draft()} onClose={vi.fn()} />,
    );

    const choices = screen.getAllByRole("radio");
    expect(choices).toHaveLength(2);
    expect(choices.every((c) => c.getAttribute("aria-checked") === "false")).toBe(true);
    expect(state.rehearse).not.toHaveBeenCalled();
  });

  it("rehearses with migrate=true when everyone is moving", async () => {
    render(
      <PublishVersionDialog bundleId="b1" name="Lab Tech" draft={draft()} onClose={vi.fn()} />,
    );
    fireEvent.click(screen.getByText("Move everyone to v3"));
    fireEvent.click(screen.getByRole("button", { name: "Preview the change" }));

    await waitFor(() => expect(state.rehearse).toHaveBeenCalled());
    expect(state.rehearse.mock.calls[0][0]).toMatchObject({ migrate: true });
  });

  it("rehearses with migrate=false when they stay put", async () => {
    render(
      <PublishVersionDialog bundleId="b1" name="Lab Tech" draft={draft()} onClose={vi.fn()} />,
    );
    fireEvent.click(screen.getByText("Leave them on the version they are on"));
    fireEvent.click(screen.getByRole("button", { name: "Preview the change" }));

    await waitFor(() => expect(state.rehearse).toHaveBeenCalled());
    expect(state.rehearse.mock.calls[0][0]).toMatchObject({ migrate: false });
  });

  // With nobody holding it there is no question, so none is asked — and
  // publishing cannot be blocked on answering it.
  it("asks nothing when nobody holds the bundle", async () => {
    render(
      <PublishVersionDialog
        bundleId="b1"
        name="Lab Tech"
        draft={draft({ holder_count: 0 })}
        onClose={vi.fn()}
      />,
    );
    expect(screen.queryAllByRole("radio")).toHaveLength(0);
    fireEvent.click(screen.getByRole("button", { name: "Preview the change" }));
    await waitFor(() => expect(state.rehearse).toHaveBeenCalled());
    expect(state.rehearse.mock.calls[0][0]).toMatchObject({ migrate: false });
  });
});
