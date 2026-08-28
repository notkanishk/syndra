// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import UpstreamProjectsPage from "@/app/zitadel/projects/page";
import { ApiError } from "@/lib/api-client";

const state = vi.hoisted(() => ({ create: vi.fn() }));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamProjects: () => ({
    data: { items: [{ id: "pLaser", name: "Laser Lab" }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamProjectRoles: () => ({
    data: { items: [] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamCreateRole: () => ({ mutateAsync: state.create, isPending: false }),
  useUpstreamUpdateRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamDeleteRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

function openTheDialog() {
  render(<UpstreamProjectsPage />);
  fireEvent.click(screen.getByRole("button", { name: "New role in Zitadel" }));
  fireEvent.change(screen.getByLabelText("Role key"), { target: { value: "trained" } });
  fireEvent.click(screen.getByRole("checkbox", { name: /I understand this changes Zitadel/ }));
}

beforeEach(() => {
  state.create.mockReset();
});

/**
 * The four upstream consoles are the least undoable writes in the product:
 * no rehearsal, no cascade preview, no ledger row. The dialog therefore
 * becomes its own result rather than closing itself — the sentence it has to
 * deliver is that Syndra has no record of what just happened, and a dialog
 * that closes takes that sentence with it.
 */
describe("the upstream role dialog becomes its own result", () => {
  it("stays open on success and says Syndra has no record of it", async () => {
    state.create.mockResolvedValue(undefined);
    openTheDialog();

    fireEvent.click(screen.getByRole("button", { name: "Create in Zitadel" }));

    // Scoped to the dialog: the empty roles list behind it is a `status` too.
    const report = await within(screen.getByRole("dialog")).findByRole("status");
    expect(report.textContent).toContain("trained created in Zitadel");
    expect(report.textContent).toContain("recorded one line in Audit and nothing else");
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  // Gated on "something was applied", not on "these exact values were already
  // written" — so editing the display name again after a successful rename
  // means closing and reopening. Deliberate, in this direction: this is the
  // least undoable write in the product and the second tap is far more often
  // a double-tap than a correction.
  it("writes once and then stops, offering Done instead", async () => {
    state.create.mockResolvedValue(undefined);
    openTheDialog();

    fireEvent.click(screen.getByRole("button", { name: "Create in Zitadel" }));
    await within(screen.getByRole("dialog")).findByRole("status");

    expect(screen.getByRole("button", { name: "Create in Zitadel" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Done" })).toBeTruthy();
    expect(state.create).toHaveBeenCalledTimes(1);
  });

  it("reports a refusal in place, with the dialog and its input still there", async () => {
    state.create.mockRejectedValue(new ApiError(403, { error: "FORBIDDEN", message: "Not allowed" }));
    openTheDialog();

    fireEvent.click(screen.getByRole("button", { name: "Create in Zitadel" }));

    const report = await screen.findByRole("alert");
    expect(report.textContent).toContain("Refused");
    await waitFor(() => expect(screen.getByLabelText("Role key")).toHaveValue("trained"));
    // Still writable: a refusal is a state the operator can act on from here.
    expect(screen.getByRole("button", { name: "Create in Zitadel" })).not.toBeDisabled();
  });
});
