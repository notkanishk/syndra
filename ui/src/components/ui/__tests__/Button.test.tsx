// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button, ButtonLink } from "@/components/ui/Button";

describe("<ButtonLink/>", () => {
  // The bug this component exists to prevent: <Link><Button/></Link> renders
  // <a><button/></a> — invalid HTML, and two overlapping interactive elements
  // where the operator sees one control.
  it("is exactly one interactive element", () => {
    render(<ButtonLink href="/requests">Request an extension</ButtonLink>);

    const link = screen.getByRole("link", { name: "Request an extension" });
    expect(link).toHaveAttribute("href", "/requests");
    expect(screen.queryByRole("button")).toBeNull();
    expect(link.querySelector("button")).toBeNull();
  });

  it("carries the same surface as the button it replaces", () => {
    const { container: linked } = render(<ButtonLink href="/x" variant="accent">Go</ButtonLink>);
    const { container: pressed } = render(<Button variant="accent">Go</Button>);

    // Same variant, same visual control — only the element differs.
    expect(linked.firstElementChild?.className).toBe(pressed.firstElementChild?.className);
  });
});

describe("<Button/>", () => {
  it("renders a disabled control's reason as visible copy, not a title", () => {
    render(
      <Button disabled reason="Needs an endpoint that doesn't exist yet.">
        Remove access
      </Button>,
    );

    const button = screen.getByRole("button", { name: "Remove access" });
    expect(button).toBeDisabled();
    // Hover doesn't exist on touch and doesn't survive a screenshot.
    expect(button).not.toHaveAttribute("title");
    expect(screen.getByText("Needs an endpoint that doesn't exist yet.")).toBeInTheDocument();
  });
});
