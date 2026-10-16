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

  it("names the verb with no arrow, as a link", () => {
    render(<EmptyState title="Nothing yet" action={{ label: "Open People", href: "/users" }} />);
    expect(screen.getByRole("link", { name: "Open People" }).textContent).toBe("Open People");
  });
});
