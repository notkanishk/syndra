// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EmptyState } from "@/components/states";

// A-04 §7.5: only ArrowLink ends in an arrow. An empty state's action is a verb
// ("Ask for access"), whether it opens a dialog or goes to a page.
describe("EmptyState action", () => {
  it("names the verb with no arrow, as a button", () => {
    render(<EmptyState title="Nothing yet" action={{ label: "Ask for access", onClick: () => {} }} />);
    expect(screen.getByRole("button", { name: "Ask for access" }).textContent).toBe("Ask for access");
  });

  // A-17 G9: the button is the shared outline sm; the link is the ArrowLink, its arrow silent.
  it("is an outline sm Button when it acts", () => {
    render(<EmptyState title="Nothing yet" action={{ label: "Ask for access", onClick: () => {} }} />);
    expect(screen.getByRole("button", { name: "Ask for access" }).className).toContain("border-line-strong");
  });

  it("is an ArrowLink when it goes somewhere, with the arrow hidden from screen readers", () => {
    render(<EmptyState title="Nothing yet" action={{ label: "Open People", href: "/users" }} />);
    const link = screen.getByRole("link", { name: "Open People" });
    expect(link).toHaveAttribute("href", "/users");
    expect(link.querySelector('[aria-hidden="true"]')?.textContent).toBe("→");
  });
});
