// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ServiceAccountDialog, UnmarkDialog } from "@/components/system/ServiceAccountDialogs";
import { ApiError } from "@/lib/api-client";
import type { UserListEntry } from "@/lib/queries/useUsers";

const save = vi.hoisted(() => vi.fn());
const unmark = vi.hoisted(() => vi.fn());
const people = vi.hoisted(() => ({ data: [] as UserListEntry[] }));

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useSaveServiceAccount: () => ({ mutateAsync: save, isPending: false }),
  useUnmarkServiceAccount: () => ({ mutateAsync: unmark, isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUsers: (q: string) => ({ data: q ? people.data : [], isLoading: false }),
}));

function entry(id: string, name: string, extra: Partial<UserListEntry["user"]> = {}): UserListEntry {
  return {
    user: { id, name, email: `${id}@example.edu`, title: "Technician", team: "", status: "active", avatar: "", ...extra },
  } as UserListEntry;
}

const kiosk = { id: "kiosk", name: "Ingestion Station" };

beforeEach(() => {
  save.mockReset().mockResolvedValue({});
  unmark.mockReset().mockResolvedValue({});
  people.data = [entry("u9", "Sai Khurana"), entry("svc", "netbird", { service_account: true }), entry("kiosk", "Ingestion Station")];
});
afterEach(cleanup);

async function chooseOwner() {
  fireEvent.change(screen.getByLabelText("Search people by name or email"), { target: { value: "sa" } });
  fireEvent.click(await screen.findByText("Sai Khurana"));
}

describe("Mark as a service account", () => {
  it("asks what it is first, and won't send until that is answered", () => {
    render(<ServiceAccountDialog mode="mark" account={kiosk} onClose={() => {}} />);

    expect(screen.getByRole("heading", { name: "Mark Ingestion Station as a service account" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mark as a service account" })).toBeDisabled();
    expect(screen.getByText("Choose what it is first.")).toBeInTheDocument();
    // The four kinds, each with what it means.
    for (const label of ["Integration", "Device", "Agent", "Shared login"]) {
      expect(screen.getByRole("radio", { name: new RegExp(label) })).toBeInTheDocument();
    }
    expect(
      screen.getByText("This finishes now. It leaves People, the people counts and the welcome bundle; nothing about its access changes."),
    ).toBeInTheDocument();
  });

  it("sends kind, purpose and owner, then ends on Marked naming who answers for it", async () => {
    render(<ServiceAccountDialog mode="mark" account={kiosk} onClose={() => {}} />);

    fireEvent.click(screen.getByRole("radio", { name: /Device/ }));
    fireEvent.change(screen.getByLabelText("Purpose"), { target: { value: "  Intake kiosk  " } });
    await chooseOwner();
    fireEvent.click(screen.getByRole("button", { name: "Mark as a service account" }));

    await waitFor(() => expect(save).toHaveBeenCalledWith({ kind: "device", purpose: "Intake kiosk", owner_id: "u9" }));
    const result = await screen.findByRole("status");
    expect(result).toHaveTextContent("Marked");
    expect(result).toHaveTextContent("Ingestion Station is a service account now, and Sai Khurana answers for it.");
    expect(result).toHaveTextContent("If this was wrong, Mark as a person on its page puts it back.");
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("says nothing of an owner when none was chosen", async () => {
    render(<ServiceAccountDialog mode="mark" account={kiosk} onClose={() => {}} />);
    fireEvent.click(screen.getByRole("radio", { name: /Shared login/ }));
    fireEvent.click(screen.getByRole("button", { name: "Mark as a service account" }));

    await waitFor(() => expect(save).toHaveBeenCalledWith({ kind: "shared_login", purpose: "", owner_id: "" }));
    expect(await screen.findByRole("status")).toHaveTextContent(/^MarkedIngestion Station is a service account now\.It's in/);
  });

  it("offers only people as owner: not another service account, not itself", async () => {
    render(<ServiceAccountDialog mode="mark" account={kiosk} onClose={() => {}} />);
    fireEvent.change(screen.getByLabelText("Search people by name or email"), { target: { value: "n" } });

    expect(await screen.findByText("Sai Khurana")).toBeInTheDocument();
    expect(screen.queryByText("netbird")).not.toBeInTheDocument();
    // The account's own name is only in the title; the picker must not list it.
    expect(screen.queryByText("Ingestion Station")).not.toBeInTheDocument();
  });

  it("clears a chosen owner with the cross on its chip", async () => {
    render(<ServiceAccountDialog mode="mark" account={kiosk} onClose={() => {}} />);
    await chooseOwner();

    expect(screen.getByText("Technician")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Clear owner" }));
    expect(screen.getByLabelText("Search people by name or email")).toBeInTheDocument();
  });

  it("keeps the form and says nothing was changed when the save is refused", async () => {
    save.mockRejectedValue(new ApiError(400, { error: "VALIDATION_FAILED", message: "Purpose is too long" }));
    render(<ServiceAccountDialog mode="mark" account={kiosk} onClose={() => {}} />);
    fireEvent.click(screen.getByRole("radio", { name: /Agent/ }));
    fireEvent.click(screen.getByRole("button", { name: "Mark as a service account" }));

    expect(await screen.findByText(/Purpose is too long\. Nothing was changed\./)).toBeInTheDocument();
    expect(screen.queryByText("Marked")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mark as a service account" })).toBeEnabled();
  });
});

describe("Change purpose and Change owner", () => {
  const current = { id: "svc", name: "netbird", kind: "agent" as const, purpose: "Syncs the door list", owner_id: "u9" };

  it("changes the purpose alone, keeping the kind and the owner, and ends on Saved", async () => {
    render(<ServiceAccountDialog mode="purpose" account={current} onClose={() => {}} />);

    expect(screen.getByRole("heading", { name: "Change purpose" })).toBeInTheDocument();
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Purpose"), { target: { value: "Syncs doors and lockers" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(save).toHaveBeenCalledWith({ kind: "agent", purpose: "Syncs doors and lockers", owner_id: "u9" }),
    );
    expect(await screen.findByRole("status")).toHaveTextContent("Saved");
  });

  it("changes the owner alone, and can leave it with nobody", async () => {
    render(<ServiceAccountDialog mode="owner" account={current} onClose={() => {}} />);

    expect(screen.getByRole("heading", { name: "Change owner" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Purpose")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Clear owner" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(save).toHaveBeenCalledWith({ kind: "agent", purpose: "Syncs the door list", owner_id: "" }));
  });
});

describe("Mark as a person", () => {
  it("confirms, deletes the mark and ends on its own result", async () => {
    render(<UnmarkDialog account={kiosk} onClose={() => {}} />);

    expect(screen.getByRole("heading", { name: "Mark Ingestion Station as a person" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Mark as a person" }));

    await waitFor(() => expect(unmark).toHaveBeenCalled());
    expect(await screen.findByRole("status")).toHaveTextContent("Ingestion Station is a person again.");
  });

  it("reports Zitadel's own refusal for a machine account, and nothing else changes", async () => {
    unmark.mockRejectedValue(new ApiError(409, { error: "MACHINE_ACCOUNT", message: "Zitadel says it's a machine account" }));
    render(<UnmarkDialog account={kiosk} onClose={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "Mark as a person" }));

    expect(await screen.findByText(/Zitadel says it's a machine account\. Nothing was changed\./)).toBeInTheDocument();
  });
});
