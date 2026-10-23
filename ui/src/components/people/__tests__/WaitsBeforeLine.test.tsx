// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RemovalDialog } from "@/components/people/RemovalDialog";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";

/**
 * A-07 §9: while Zitadel isn't answering, a person's page, Requests and Expiring stay on and
 * their dialogs say, before the act, that the change will wait. Their buttons stay live.
 */

const LINE = "Zitadel isn't answering, so this will wait for you to send it once it does.";

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
        userName="Shikha Yadav"
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
        userName="Shikha Yadav"
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
