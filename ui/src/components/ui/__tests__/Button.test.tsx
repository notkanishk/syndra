// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button, ButtonLink, PILL } from "@/components/ui/Button";

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
    //
    // Compared on the CONTROL, not on each tree's root. `Button` always wraps
    // itself now, so that its root does not change shape when a reason appears
    // and remount the node somebody is typing next to; the root is that
    // wrapper and has never been the thing this asserts.
    expect(linked.querySelector("a")?.className).toBe(
      pressed.querySelector("button")?.className,
    );
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

  // D-1 §3: height came from padding plus border, so an outline button stood
  // 2px taller than an accent one beside it. Height is set, from the token.
  it("draws an accent and an outline button of one size at one height, from the token", () => {
    for (const size of ["sm", "md"] as const) {
      const { container } = render(
        <>
          <Button size={size} variant="accent">Save name</Button>
          <Button size={size} variant="outline">Cancel</Button>
        </>,
      );
      const [accent, outline] = Array.from(container.querySelectorAll("button"));
      const heights = (button: Element) => button.className.split(/\s+/).filter((c) => /(?:^|:)(?:min-)?h-|(?:^|:)py-/.test(c));
      expect(heights(accent)).toEqual(heights(outline));
      expect(heights(accent)).toContain(`desktop:min-h-(--control-${size})`);
      expect(PILL[size], "padding must not build the height").not.toMatch(/py-/);
    }
  });
});
