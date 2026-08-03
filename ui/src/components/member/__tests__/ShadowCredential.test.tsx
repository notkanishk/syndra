// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ShadowCredential } from "@/components/member/ShadowCredential";

const state = vi.hoisted(() => ({
  status: { has_credential: false } as Record<string, unknown>,
  statusError: null as Error | null,
  set: vi.fn(),
  clear: vi.fn(),
}));

vi.mock("@/lib/queries/useShadowCredential", () => ({
  useShadowCredentialStatus: () => ({
    data: state.status,
    isLoading: false,
    error: state.statusError,
  }),
  useShadowCredentialAudit: () => ({ data: [], isLoading: false, error: null }),
  useSetShadowCredential: () => ({ mutateAsync: state.set, isPending: false }),
  useClearShadowCredential: () => ({ mutateAsync: state.clear, isPending: false }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() } }));

beforeEach(() => {
  state.status = { has_credential: false };
  state.statusError = null;
  state.set = vi.fn().mockResolvedValue({ message: "ok" });
  state.clear = vi.fn().mockResolvedValue({ message: "ok" });
});

describe("ShadowCredential", () => {
  // The single most likely misreading of this card is "my login password changed". Both the
  // card and the dialog have to deny it in words, not by context.
  it("says this is not the university login", () => {
    render(<ShadowCredential userId="u1" />);
    expect(document.body.textContent).toMatch(/not.*university login/i);
  });

  // This card used to render nothing when its status read failed, and that silence hid a real
  // fault for every member: the console proxy did not permit the vault's routes, so the card
  // vanished and looked like a design decision rather than a 403.
  it("says so when its status cannot be read, instead of disappearing", () => {
    state.statusError = new Error("Forbidden");
    render(<ShadowCredential userId="u1" />);
    expect(document.body.textContent).toMatch(/couldn.t load/i);
    // And it must not imply the member has lost anything.
    expect(document.body.textContent).toMatch(/access is unaffected/i);
  });

  // Nothing reads the credential until the hardware bridge is built. A member who sets one and
  // tries a door has to have been told, or they conclude the product is broken.
  it("says nothing is reading it yet", () => {
    render(<ShadowCredential userId="u1" />);
    expect(document.body.textContent).toMatch(/no hardware is connected/i);
  });

  it("offers removal only once a password exists", () => {
    const { rerender } = render(<ShadowCredential userId="u1" />);
    expect(screen.queryByRole("button", { name: "Remove" })).toBeNull();

    state.status = { has_credential: true };
    rerender(<ShadowCredential userId="u2" />);
    expect(screen.getByRole("button", { name: "Remove" })).toBeTruthy();
  });

  // Nothing can read the password back to compare against, so a typo would first surface at a
  // machine that will not open.
  it("will not submit until both fields match", async () => {
    render(<ShadowCredential userId="u1" />);
    fireEvent.click(screen.getByRole("button", { name: "Set a password" }));

    fireEvent.change(screen.getByLabelText("New password"), {
      target: { value: "Correct-horse1!" },
    });
    fireEvent.change(screen.getByLabelText("Type it again"), { target: { value: "Correct-h" } });

    const submit = screen.getByRole("button", { name: "Set it" });
    fireEvent.click(submit);
    expect(state.set).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Type it again"), {
      target: { value: "Correct-horse1!" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Set it" }));
    await waitFor(() => expect(state.set).toHaveBeenCalledWith("Correct-horse1!"));
  });

  // The server composes the failing requirements into one sentence. Showing it verbatim is what
  // keeps this file from growing a second opinion about password strength.
  it("shows the server's own rejection rather than its own rules", async () => {
    state.set = vi
      .fn()
      .mockRejectedValue(new Error("password complexity: must contain at least one symbol"));
    render(<ShadowCredential userId="u1" />);
    fireEvent.click(screen.getByRole("button", { name: "Set a password" }));
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: "Password1234" } });
    fireEvent.change(screen.getByLabelText("Type it again"), { target: { value: "Password1234" } });
    fireEvent.click(screen.getByRole("button", { name: "Set it" }));

    await waitFor(() =>
      expect(screen.getByText(/must contain at least one symbol/)).toBeTruthy(),
    );
    // Still open — a rejected password must not look like a saved one.
    expect(screen.getByRole("button", { name: "Set it" })).toBeTruthy();
  });
});
