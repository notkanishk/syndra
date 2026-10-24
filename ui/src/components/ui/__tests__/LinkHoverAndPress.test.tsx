// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatCard } from "@/components/ui/StatCard";

/** A-08 §2a and §5: one hover for every text link, and a press that layers. */

describe("ArrowLink", () => {
  it("keeps its violet and tints the underline in, with no brightness lift", () => {
    render(<ArrowLink href="/users">People</ArrowLink>);
    const link = screen.getByRole("link", { name: /People/ });
    expect(link).toHaveClass("text-accent-text", "link-hover");
    expect(link.className).not.toMatch(/brightness|hover:text-ink/);
  });
});

describe("a StatCard that is a link", () => {
  it("adds --tint-3 as a second background layer when pressed, never in place of the surface", () => {
    render(<StatCard label="Drift" value="3" detail="to look at" href="/review/drift" />);
    const card = screen.getByRole("link", { name: "Drift" });
    // A gradient is an image layer over background-color, so the card's own
    // surface still shows through; `active:bg-*` would have replaced it.
    expect(card.className).toContain("active:[background-image:linear-gradient(var(--tint-3),var(--tint-3))]");
    expect(card.className).not.toMatch(/active:bg-/);
    // The accent ring on hover stays.
    expect(card.className).toContain("hover:shadow-[0_0_0_1px_var(--accent-line)]");
  });

  it("is not a link, and presses nothing, without an href", () => {
    render(<StatCard label="Drift" value="3" detail="to look at" />);
    expect(screen.queryByRole("link")).toBeNull();
  });
});
