// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Term } from "@/components/ui/Term";
import { GLOSSARY } from "@/lib/glossary";

/**
 * The point of a defined term is that everybody can reach the definition.
 *
 * Hover was the obvious implementation and would have been the wrong one:
 * it hides the meaning from anybody navigating by keyboard, and from
 * everybody on the workshop tablet. These tests exist because "it works when
 * I mouse over it" is not evidence that it works.
 */
describe("Term", () => {
  it("is reachable and operable from the keyboard", () => {
    render(<Term name="cascade">cascade</Term>);
    const word = screen.getByRole("button", { name: "cascade" });

    word.focus();
    expect(word).toHaveFocus();

    expect(word).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(word);
    expect(word).toHaveAttribute("aria-expanded", "true");
  });

  it("describes the word to a screen reader whether or not the popover is open", () => {
    render(<Term name="drift">drift</Term>);
    const word = screen.getByRole("button", { name: "drift" });
    const describedBy = word.getAttribute("aria-describedby");
    expect(describedBy).toBeTruthy();

    // Shut: the definition must still be IN the document and resolvable, not
    // removed. `display: none` here would silently break the description in
    // several screen readers, leaving the marked-up word less informative
    // than the plain word it replaced.
    const note = document.getElementById(describedBy!);
    expect(note).not.toBeNull();
    expect(note).toHaveTextContent(GLOSSARY.drift.definition);
  });

  it("opens on hover for a mouse, without needing a click", () => {
    render(<Term name="outbox">outbox</Term>);
    expect(document.querySelector(".settle-in")).toBeNull();

    fireEvent.mouseEnter(screen.getByRole("button", { name: "outbox" }).parentElement!);
    expect(document.querySelector(".settle-in")).not.toBeNull();
  });

  it("closes on Escape, so a sticky definition is never a trap", () => {
    render(<Term name="hold">hold</Term>);
    const word = screen.getByRole("button", { name: "hold" });

    fireEvent.click(word);
    expect(word).toHaveAttribute("aria-expanded", "true");

    fireEvent.keyDown(document, { key: "Escape" });
    expect(word).toHaveAttribute("aria-expanded", "false");
  });

  it("falls back to the glossary's own title when no text is given", () => {
    render(<Term name="truenas" />);
    expect(screen.getByRole("button", { name: "TrueNAS" })).toBeInTheDocument();
  });

  it("defines every term it offers, in one voice", () => {
    for (const [name, entry] of Object.entries(GLOSSARY)) {
      expect(entry.title, `${name} has no title`).toBeTruthy();
      // A definition that is one clause long is a synonym, not a definition,
      // and a synonym is what this mechanism exists to stop shipping.
      expect(entry.definition.length, `${name}'s definition is too thin`).toBeGreaterThan(60);
      expect(entry.definition.trim().endsWith("."), `${name} must end in a full stop`).toBe(true);
    }
  });
});
