// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import UpstreamUsersPage from "@/app/zitadel/users/page";

const state = vi.hoisted(() => ({
  update: vi.fn(),
  assign: vi.fn(),
  waits: undefined as Array<{ project_id: string; role_key: string }> | undefined,
}));

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamUsers: () => ({
    data: { items: [{ id: "u1", displayName: "Priya", userName: "priya", email: "priya@example.org", state: "USER_STATE_ACTIVE" }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamUserGrants: () => ({
    data: { items: [{ id: "g1", projectId: "pLaser", roleKeys: ["trained"] }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamProjects: () => ({ data: { items: [{ id: "pLaser", name: "Laser Lab" }] } }),
  useUpstreamProjectRoles: () => ({ data: { items: [] } }),
  useUpstreamAssignGrant: () => ({ mutateAsync: state.assign, isPending: false }),
  useUpstreamUpdateGrant: () => ({ mutateAsync: state.update, isPending: false }),
  useUpstreamRemoveGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => ({ isLoading: false, data: { projects: [] } }),
}));

vi.mock("@/lib/queries/useRolesThatWait", () => ({
  useRolesThatWait: () => ({ data: state.waits ? { roles: state.waits } : undefined }),
}));

vi.mock("@/components/names", () => ({ ProjectName: () => null }));

beforeEach(() => {
  state.update = vi.fn();
  state.assign = vi.fn();
  state.waits = [];
});

function submitEdit() {
  fireEvent.click(screen.getByRole("button", { name: "Change roles" }));
  const dialog = screen.getByRole("dialog");
  fireEvent.click(within(dialog).getByRole("checkbox"));
  fireEvent.click(within(dialog).getByRole("button", { name: "Replace roles in Zitadel" }));
}

function openGive(keys: string) {
  fireEvent.click(screen.getByRole("button", { name: "Give roles in Zitadel" }));
  const dialog = screen.getByRole("dialog");
  fireEvent.change(within(dialog).getByLabelText("Project"), { target: { value: "pLaser" } });
  fireEvent.change(within(dialog).getByLabelText("Role keys"), { target: { value: keys } });
  return dialog;
}

// A success used to close the dialog onto the row; the sheet now becomes its result (A-17 G4).
describe("replacing roles in Zitadel", () => {
  it("ends on its result in the sheet, with the act's word and one Close", async () => {
    state.update.mockResolvedValue(undefined);
    render(<UpstreamUsersPage />);
    submitEdit();

    const dialog = screen.getByRole("dialog");
    await waitFor(() => expect(within(dialog).getByText("Replaced")).toBeTruthy());
    expect(dialog.textContent).toMatch(/Roles replaced in Zitadel/);
    expect(within(dialog).queryByRole("button", { name: "Replace roles in Zitadel" })).toBeNull();
    expect(within(dialog).queryByRole("button", { name: "Cancel" })).toBeNull();
    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps a failure in the dialog, beside the retry", async () => {
    state.update.mockRejectedValue(new Error("nope"));
    render(<UpstreamUsersPage />);
    submitEdit();

    await waitFor(() => expect(state.update).toHaveBeenCalled());
    const dialog = screen.getByRole("dialog");
    await waitFor(() => expect(within(dialog).getByRole("alert")).toBeTruthy());
    expect(document.body.textContent).not.toMatch(/Roles replaced in Zitadel/);
  });

  // C4: what a replace takes away, in the caption.
  it("says a role left out stays when a bundle or a rule still gives it", () => {
    render(<UpstreamUsersPage />);
    fireEvent.click(screen.getByRole("button", { name: "Change roles" }));
    expect(document.body.textContent).toMatch(/revokes any you leave out, unless a bundle or a rule still gives it/);
  });

  // W8: a role that waits is not "replaced in Zitadel".
  it("reports a role that waits as waiting, not done", async () => {
    state.update.mockResolvedValue({
      status: "pending",
      wait_reason: "role_waits",
      roles: [
        { role: "pLaser/trained", change: "revoked", status: "applied" },
        { role: "pLaser/keyholder", change: "given", status: "waiting" },
      ],
    });
    render(<UpstreamUsersPage />);
    submitEdit();

    const dialog = screen.getByRole("dialog");
    await waitFor(() => expect(dialog.textContent).toMatch(/Waiting for you to send, because the role always waits for you: keyholder\./));
    expect(dialog.textContent).toMatch(/Revoked in Zitadel: trained\./);
    expect(dialog.textContent).not.toMatch(/Roles replaced in Zitadel/);
    expect(within(dialog).queryByRole("button", { name: "Replace roles in Zitadel" })).toBeNull();
  });
});

// C3: the give consent. A record IS written, and the caption says when it is sent.
describe("giving roles in Zitadel", () => {
  it("never says there is no record, and says it is sent straight away", () => {
    render(<UpstreamUsersPage />);
    const dialog = openGive("trained");
    expect(within(dialog).getByText("I understand this changes Zitadel now, with no preview and no reason.")).toBeTruthy();
    expect(dialog.textContent).not.toMatch(/no record in Syndra/);
    expect(dialog.textContent).toMatch(/It is sent to Zitadel straight away\./);
    expect(dialog.textContent).not.toMatch(/when Pending changes is sent/);
  });

  it("names the roles that wait in the caption", () => {
    state.waits = [{ project_id: "pLaser", role_key: "keyholder" }];
    render(<UpstreamUsersPage />);
    const dialog = openGive("trained, keyholder");
    expect(dialog.textContent).toMatch(
      /It is sent to Zitadel straight away, except keyholder: that role always waits for you, so its change waits in Pending changes until you send it\./,
    );
  });

  it("before the roles that wait are read, names the case rather than guessing", () => {
    state.waits = undefined;
    render(<UpstreamUsersPage />);
    const dialog = openGive("trained");
    expect(dialog.textContent).toMatch(/straight away, unless a role waits for you/);
  });

  // W8: given in Zitadel, waiting, or not taken: each role in its own words.
  it("shows each role's real status after a give", async () => {
    state.assign.mockResolvedValue({
      status: "pending",
      roles: [
        { role: "pLaser/trained", change: "given", status: "applied" },
        { role: "pLaser/maintainer", change: "given", status: "pending" },
      ],
    });
    render(<UpstreamUsersPage />);
    const dialog = openGive("trained, maintainer");
    fireEvent.click(within(dialog).getByRole("checkbox"));
    fireEvent.click(within(dialog).getByRole("button", { name: "Give roles in Zitadel" }));

    await waitFor(() => expect(dialog.textContent).toMatch(/Given in Zitadel: trained\./));
    expect(dialog.textContent).toMatch(/Not in Zitadel yet, because Zitadel didn't take it\. It is in Pending changes: maintainer\./);
    expect(dialog.textContent).not.toMatch(/Roles given in Zitadel/);
  });
});
