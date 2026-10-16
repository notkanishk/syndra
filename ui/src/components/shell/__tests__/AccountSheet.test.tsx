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
    expect(screen.getByText(/signs you out in every open tab/i)).toBeTruthy();
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

  it("offers the drift sound as a browser-only setting", () => {
    open();
    const sheet = screen.getByRole("dialog");
    expect(within(sheet).getByText("Sound when drift is found")).toBeTruthy();
    expect(within(sheet).getByText("In this browser only")).toBeTruthy();
  });

  // Every control in here is something a thumb has to land on.
  it("gives every control a real target", () => {
    open();
    for (const name of [/Appearance/, /Sign out/]) {
      const control = screen.getByRole("button", { name });
      expect(control.className, `${name} must clear the touch floor`).toContain("min-h-[44px]");
    }
  });

  // An operator is a person too: their own storage password and folders sit
  // behind the account, since the rail is about running the makerspace.
  it("takes an operator to their own access and network storage, access first", () => {
    open();
    const links = screen.getAllByRole("link");
    expect(links.map((l) => l.getAttribute("href"))).toEqual(["/me", "/storage"]);
    expect(links[0].textContent).toContain("What you can use, and asking for more");
    expect(links[1].textContent).toContain("Your password and folders");
    for (const link of links) expect(link.className).toContain("min-h-[44px]");
  });

  // A member has the same page in their rail; one row per job.
  it("doesn't offer a member their storage twice", () => {
    render(<AccountSheet session={{ ...session, role: "user" } as SessionUser} />);
    fireEvent.click(screen.getByRole("button", { name: /Account/ }));
    expect(screen.queryByRole("link", { name: /Network storage|My access/ })).toBeNull();
  });
});
