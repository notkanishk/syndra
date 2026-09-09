// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PageHeader } from "@/components/ui/PageHeader";

/**
 * `/users/<id>` paired a `flex-1` (basis 0%) title block against `actions`
 * on the same un-wrapped flex line: the title reported zero size to the
 * wrap decision, so `actions` claimed the line and the title was squeezed
 * into whatever sliver was left — "Sam Patel" wrapped into "Sam" / "Patel"
 * with the rest of the row empty. `flex-auto` (basis auto) makes the title
 * count its real size, so a tight line wraps `actions` below instead.
 */
describe("PageHeader title block sizing", () => {
  it("gives the title its content-based size instead of starving it next to actions", () => {
    render(<PageHeader title="Sam Patel" actions={<button>Grant direct access</button>} />);

    const heading = screen.getByRole("heading", { level: 1, name: "Sam Patel" });
    const titleBlock = heading.parentElement;

    expect(titleBlock?.className).toMatch(/\bflex-auto\b/);
    expect(titleBlock?.className).not.toMatch(/\bflex-1\b/);
  });
});
