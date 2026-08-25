// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AccountSheet } from "@/components/shell/AccountSheet";
import type { SessionUser } from "@/lib/session";

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({ theme: "dark", toggle: vi.fn() }),
}));

const session = {
  id: "u_1",
  name: "Kabir Rao",
  email: "kabir@example.edu",
  avatar: "KR",
  role: "admin",
} as SessionUser;

function open() {
  render(<AccountSheet session={session} />);
  fireEvent.click(screen.getByRole("button", { name: /Account/ }));
}

describe("the account sheet", () => {
  // A standing sign-out button on a phone sits one mis-tap from a
  // destination, and signing out clears every tab's place.
  it("keeps sign-out two taps deep rather than standing in the header", () => {
    render(<AccountSheet session={session} />);
    expect(screen.queryByRole("button", { name: "Sign out" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /Account/ }));
    expect(screen.getByRole("button", { name: "Sign out" })).toBeTruthy();
  });

  it("says what signing out costs, before it is tapped", () => {
    open();
    expect(screen.getByText(/clears where you were on every tab/i)).toBeTruthy();
  });

  // It is a POST, not a link: a session ends through a form the server
  // handles, and nothing about moving it into a sheet may change that.
  it("ends the session through a form the server owns", () => {
    open();
    const form = screen.getByRole("button", { name: "Sign out" }).closest("form");
    expect(form).toHaveAttribute("action", "/auth/logout");
    expect(form).toHaveAttribute("method", "post");
  });

  it("names who is signed in, with the address under it", () => {
    open();
    const sheet = screen.getByRole("dialog");
    // The trigger also carries the name at tablet width, so scope to the
    // sheet rather than the document.
    expect(within(sheet).getByText("Kabir Rao")).toBeTruthy();
    expect(within(sheet).getByText("kabir@example.edu")).toBeTruthy();
  });

  it("offers appearance where a narrow header has no room for a toggle", () => {
    open();
    expect(screen.getByRole("button", { name: /Appearance/ })).toBeTruthy();
  });

  // Every control in here is something a thumb has to land on.
  it("gives every control a real target", () => {
    open();
    for (const name of [/Appearance/, /Sign out/]) {
      const control = screen.getByRole("button", { name });
      expect(control.className, `${name} must clear the touch floor`).toContain("min-h-[44px]");
    }
  });
});
