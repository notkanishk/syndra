// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RemovalDialog } from "@/components/people/RemovalDialog";

/**
 * The removal flow has to be reachable from the person page.
 *
 * `RemovalDialog` resolves the person as `removal.userId ?? userId`, and
 * `PersonAccess` passed NEITHER — not on the `removal` object it builds, not as
 * a prop. So `person` was undefined, the confirm button was disabled, and the
 * bundle dialog rendered its fallback title: "Remove the Admin Ops bundle from
 * this person?". The comment beside that control calls it "the only way into
 * the removal flow for a role"; it could not be pressed.
 *
 * Nothing caught it because the dialog's own tests always supply a person, and
 * the other caller — the project role page — has always passed `userId`. The
 * defect lived entirely in one call site's wiring, so the assertions here are
 * about what the dialog does with a person missing versus present.
 */

vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useRemoveGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

const bundleRemoval = {
  projectId: "p-admin",
  projectName: "Admin Ops Project",
  roleKey: "admin-staff",
  sources: [{ kind: "bundle", bundle_id: "b1", bundle_name: "Admin Ops", description: "" }],
};

describe("RemovalDialog with a bundle source", () => {
  it("is dead, and says so, when the caller passes no person", () => {
    render(
      <RemovalDialog
        removal={bundleRemoval}
        userId={undefined}
        userName={undefined}
        onClose={vi.fn()}
      />,
    );

    // The fingerprint of the defect, exactly as it appeared on screen.
    expect(screen.getByText(/from this person/)).toBeInTheDocument();

    const remove = screen.getByRole("button", { name: "Remove bundle" });
    expect(remove).toBeDisabled();
    // And it no longer greys out in silence.
    expect(screen.getByText(/could not tell which person this is/)).toBeInTheDocument();
  });

  it("names the person and offers a live button when the caller passes one", () => {
    render(
      <RemovalDialog
        removal={bundleRemoval}
        userId="shikha"
        userName="Shikha Rao"
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/from Shikha Rao/)).toBeInTheDocument();
    expect(screen.queryByText(/from this person/)).toBeNull();
    expect(screen.getByRole("button", { name: "Remove bundle" })).toBeEnabled();
    expect(screen.queryByText(/could not tell which person this is/)).toBeNull();
  });
});
