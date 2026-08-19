// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button, ButtonLink } from "@/components/ui/Button";

/**
 * The floor, asserted at the one place that can hold it.
 *
 * jsdom computes no layout, so this cannot measure a rendered height — but it
 * can hold the contract that produces one. Every button in the product goes
 * through `buttonClasses`, which is why raising the floor there was a single
 * edit rather than several hundred, and why losing it would be a single edit
 * too.
 *
 * The rule is 44px until the DESKTOP breakpoint, not until tablet: 720–1080 is
 * a floor operator holding a tablet, and a tablet is still a thumb.
 */
describe("every control clears the touch floor", () => {
  it("gives both button sizes a 44px minimum on touch", () => {
    render(
      <>
        <Button size="sm">Refresh</Button>
        <Button size="md">Approve</Button>
      </>,
    );
    for (const name of ["Refresh", "Approve"]) {
      const control = screen.getByRole("button", { name });
      expect(control.className, `${name} must clear the floor`).toContain("min-h-[44px]");
      // And releases it only above the desktop breakpoint, where the product
      // is a dense rail-and-table application again.
      expect(control.className).toContain("desktop:min-h-0");
    }
  });

  it("holds the floor for a link that looks like a button", () => {
    render(<ButtonLink href="/users">People</ButtonLink>);
    expect(screen.getByRole("link", { name: "People" }).className).toContain("min-h-[44px]");
  });

  it("keeps the floor on a destructive control, where a mis-tap costs most", () => {
    render(<Button variant="danger">Revoke</Button>);
    expect(screen.getByRole("button", { name: "Revoke" }).className).toContain("min-h-[44px]");
  });
});
