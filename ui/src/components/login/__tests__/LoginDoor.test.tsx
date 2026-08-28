// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LoginDoor } from "@/components/login/LoginDoor";
import { loginFailure } from "@/lib/login-error";

const IDENTITIES = [
  { id: "dev_admin", name: "Alice Rivera", role: "admin" as const },
  { id: "sam_student", name: "Sam Patel", role: "user" as const },
];

function stage() {
  return document.querySelector<HTMLElement>(".login-stage")!;
}

describe("LoginDoor", () => {
  it("offers exactly one action, and it is a real link", () => {
    render(<LoginDoor mode="oidc" identities={[]} failure={null} />);

    const action = screen.getByRole("link", { name: /sign in with your makerspace account/i });
    expect(action).toHaveAttribute("href", "/auth/zitadel");

    // No email, no password, no "or continue with", no sign-up, no reset —
    // Zitadel owns all of that.
    expect(screen.queryAllByRole("textbox")).toHaveLength(0);
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("never shows a message the page is not showing", () => {
    render(<LoginDoor mode="oidc" identities={[]} failure={null} />);
    expect(screen.queryByText(/taking you to the sign-in page/i)).toBeNull();
  });

  it("opens the door and hands off without blocking the navigation", () => {
    render(<LoginDoor mode="oidc" identities={[]} failure={null} />);
    const action = screen.getByRole("link", { name: /sign in with your makerspace account/i });

    // The click must reach the browser: the animation is cover for the
    // redirect's latency, not a gate in front of it.
    const clicked = fireEvent.click(action);
    expect(clicked).toBe(true);

    expect(stage()).toHaveAttribute("data-scene", "opening");
    expect(screen.getByText(/taking you to the sign-in page/i)).toBeInTheDocument();
  });

  it("leaves the page alone when the click is a new-tab click", () => {
    render(<LoginDoor mode="oidc" identities={[]} failure={null} />);
    fireEvent.click(screen.getByRole("link", { name: /sign in/i }), { metaKey: true });
    expect(stage()).toHaveAttribute("data-scene", "idle");
  });

  it("arrives already refused, and says why without echoing the code", () => {
    render(
      <LoginDoor mode="oidc" identities={[]} failure={loginFailure("access_denied")} />,
    );

    expect(stage()).toHaveAttribute("data-scene", "unreachable");
    expect(screen.getByRole("alert")).toHaveTextContent(/didn't let you through/i);
    expect(document.body.textContent).not.toContain("access_denied");
  });

  it("reopens the door on retry, and drops the error from the URL", () => {
    window.history.replaceState(null, "", "/login?error=misconfigured");
    render(<LoginDoor mode="oidc" identities={[]} failure={loginFailure("misconfigured")} />);

    fireEvent.click(screen.getByRole("link", { name: /back to sign in/i }));

    expect(stage()).toHaveAttribute("data-scene", "idle");
    expect(screen.queryByText(/didn't answer/i)).toBeNull();
    expect(window.location.search).toBe("");
  });

  it("reveals the development identities behind the same door", () => {
    render(<LoginDoor mode="demo" identities={IDENTITIES} failure={null} />);

    // Not reachable — not merely invisible — until the door opens.
    expect(screen.queryByRole("button", { name: /alice rivera/i })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /sign in as a test person/i }));

    const alice = screen.getByRole("button", { name: /alice rivera/i });
    expect(alice.closest("form")).toHaveAttribute("action", "/auth/login");
    expect(alice.closest("form")?.querySelector("input")).toHaveValue("dev_admin");
    expect(alice).toHaveFocus();
    expect(screen.getByRole("button", { name: /sam patel · member/i })).toBeInTheDocument();
  });
});
