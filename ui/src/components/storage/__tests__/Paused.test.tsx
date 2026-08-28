// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MyStorage } from "@/components/storage/MyStorage";
import type { MyTargetView } from "@/lib/queries/useMyStorage";

/**
 * A target somebody paused, read from the member's side (designs C1–C3).
 *
 * An operator sets a target draining or read-only and the member was told
 * nothing. Under either state their ACCESS is unchanged — the server works and
 * their files are where they left them — and what stops is Syndra changing
 * their account, of which the one thing they can start here is a password. So a
 * member set one, watched it not work, and had no way to learn why.
 */

const state = { view: null as MyTargetView | null };

vi.mock("@/lib/queries/useMyStorage", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useMyStorage")>(
    "@/lib/queries/useMyStorage",
  );
  return {
    ...actual,
    useMyStorage: () => ({
      data: state.view ? [state.view] : [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    }),
    useSetStorageCredential: () => ({ mutate: vi.fn(), isPending: false, error: null, data: null }),
  };
});

function view(over: Partial<MyTargetView> = {}): MyTargetView {
  return {
    target: "truenas",
    entitled: true,
    reachable: true,
    account: { username: "a.moller" },
    credential: { set: true },
    ...over,
  } as MyTargetView;
}

beforeEach(() => {
  state.view = null;
});

const text = () => document.body.textContent ?? "";

describe("a paused target, from the member's side", () => {
  // The clause that stops somebody going to mount the share to check.
  it("leads with the fact their access has not changed", () => {
    state.view = view({ lifecycle: "draining" });
    render(<MyStorage />);

    expect(text()).toMatch(/working normally and your access to it has not changed/);
  });

  // Operator words for a thing a member experiences as a pause.
  it("never says maintenance, draining or read-only", () => {
    for (const lifecycle of ["draining", "read_only"] as const) {
      state.view = view({ lifecycle });
      const { unmount } = render(<MyStorage />);
      expect(text().toLowerCase()).not.toMatch(/maintenance|draining|read-only/);
      unmount();
    }
  });

  // A drain ends by itself. Read-only ends when a person says so, and
  // "shortly" attached to an open-ended pause is the small lie that makes the
  // rest of the page untrustworthy.
  it("gives an estimate to the drain and a person to the read-only", () => {
    state.view = view({ lifecycle: "draining" });
    const { unmount } = render(<MyStorage />);
    expect(text()).toMatch(/for a few minutes/);
    expect(text()).not.toMatch(/ask makerspace staff/i);
    unmount();

    state.view = view({ lifecycle: "read_only" });
    render(<MyStorage />);
    expect(text()).toMatch(/ask makerspace staff/i);
    expect(text()).not.toMatch(/a few minutes/);
  });

  // The moment the field or its button dims, the page has said their access is
  // affected, which is false.
  it("keeps the password field live, and puts the promise in the label", () => {
    state.view = view({ lifecycle: "read_only" });
    render(<MyStorage />);

    // The label carries the whole promise, so a member who reads nothing else
    // cannot mistake it for taking effect now.
    const save = screen.getByRole("button", { name: /Save it for when changes start again/ });
    const field = screen.getByLabelText(/password/i);
    expect(field.hasAttribute("disabled")).toBe(false);

    // Empty is the only reason it is inert — the pause is not.
    fireEvent.change(field, { target: { value: "a-real-password" } });
    expect(save.hasAttribute("disabled")).toBe(false);
  });

  it("says nothing at all when the target is active", () => {
    state.view = view({ lifecycle: "active" });
    render(<MyStorage />);

    expect(text()).not.toMatch(/Changes to this account are paused/);
    expect(screen.getByRole("button", { name: "Set password" })).toBeTruthy();
  });

  // The cruellest combination: the thing they are waiting for is exactly the
  // thing that cannot happen.
  it("says the pause is chosen, not a fault, when the account does not exist yet", () => {
    state.view = view({ account: undefined, lifecycle: "read_only" });
    render(<MyStorage />);

    expect(text()).toMatch(/deliberately stopped/);
    expect(text()).toMatch(/a pause we chose, not a fault/);
    expect(text()).toMatch(/first in line/);
  });
});
