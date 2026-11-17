// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EmptyState } from "@/components/states";
import { SettingRow } from "@/components/targets/SettingRow";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader, CardRow, ROW } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { StandingLine } from "@/components/ui/StandingLine";

/**
 * D-1 §4: every action sits in its row's right seat on the first line, never in
 * a sentence and never on a row of its own; the seat's width is the table's.
 * jsdom has no layout, so these hold the structure the layout is drawn from.
 */

/** A control a reader presses: a button, or a link drawn as a pill or an ArrowLink. */
const CONTROLS = "button, a.rounded-pill, a[data-arrow-link]";

/** Every control inside a `<p>`: a button in a sentence moves with its words. */
function controlsInProse(root: Element): Element[] {
  return Array.from(root.querySelectorAll(CONTROLS)).filter((control) => control.closest("p"));
}

/** Every control in a row that is not in the row's seat (a form or a dialog is its own row). */
function controlsOutsideSeat(row: Element): Element[] {
  return Array.from(row.querySelectorAll(CONTROLS)).filter(
    (control) => !control.closest('[data-action-seat], form, [role="dialog"]'),
  );
}

describe("CardRow's action seat", () => {
  it("draws the action as the row's last part, in the seat, before the detail line", () => {
    const { container } = render(
      <CardRow first action={<Button size="sm">Check now</Button>} detail="Last checked a minute ago.">
        <span>TrueNAS</span>
      </CardRow>,
    );
    const row = container.firstElementChild!;
    const parts = Array.from(row.children);
    const seat = parts.at(-2)!;
    expect(seat).toHaveAttribute("data-action-seat");
    expect(seat).toHaveClass("row-seat");
    expect(seat).not.toHaveAttribute("data-seat"); // the default width
    expect(seat).toContainElement(screen.getByRole("button", { name: "Check now" }));
    const detail = parts.at(-1)!;
    expect(detail).toHaveAttribute("data-row-detail");
    expect(detail).toHaveClass("row-detail");
    expect(detail).toHaveTextContent("Last checked a minute ago.");
    expect(controlsOutsideSeat(row)).toEqual([]);
    expect(controlsInProse(row)).toEqual([]);
  });

  it("takes the table's width, and keeps an empty seat in its place", () => {
    const { container } = render(
      <Card>
        <CardColumns seat="wide">
          <span className="flex-1">Account</span>
        </CardColumns>
        <CardRow first seat="wide" action={<Button size="sm">Adopt</Button>}>
          <span>kofi</span>
        </CardRow>
        <CardRow seat="wide">
          <span>bianca</span>
        </CardRow>
      </Card>,
    );
    const seats = container.querySelectorAll(".row-seat");
    expect(seats).toHaveLength(3);
    for (const seat of Array.from(seats)) expect(seat).toHaveAttribute("data-seat", "wide");
    // The heading's cell is empty and silent; the empty row's seat is still drawn.
    expect(seats[0]).toHaveAttribute("aria-hidden");
    expect(seats[2]).toBeEmptyDOMElement();
  });

  it("draws no seat for a row that asks for none, so every existing row renders as it did", () => {
    const { container } = render(
      <CardRow first>
        <span>Seth Rollins</span>
      </CardRow>,
    );
    expect(container.querySelector("[data-action-seat]")).toBeNull();
    expect(container.querySelector("[data-row-detail]")).toBeNull();
  });

  it("top-aligns a row of more than one line, and never holds two alignments", () => {
    const { container, rerender } = render(
      <CardRow first top action={<Button size="sm">Decide</Button>}>
        <span>A long finding</span>
      </CardRow>,
    );
    const row = () => container.firstElementChild!;
    expect(row()).toHaveClass("row-top");
    expect(row()).not.toHaveClass("items-center");
    rerender(
      <CardRow first action={<Button size="sm">Decide</Button>}>
        <span>A short finding</span>
      </CardRow>,
    );
    expect(row()).toHaveClass("items-center");
    expect(row()).not.toHaveClass("row-top");
  });

  it("keeps the seat in a row that discloses the rest of itself", () => {
    render(
      <CardRow first disclosure={<span>body</span>} action={<ArrowLink href="/drift">Decide on Drift</ArrowLink>}>
        <span>Unexplained roles</span>
      </CardRow>,
    );
    expect(screen.getByRole("link", { name: /Decide on Drift/ }).closest("[data-action-seat]")).not.toBeNull();
  });

  it("is built on ROW, the box every list row takes", () => {
    const { container } = render(
      <CardRow first>
        <span>Becky Lynch</span>
      </CardRow>,
    );
    for (const c of ROW.split(" ")) expect(container.firstElementChild).toHaveClass(c);
    expect(ROW).toContain("tablet:min-h-0");
  });
});

describe("a card's explanation", () => {
  it("is a caption under the title, at the measure, never the right-hand note", () => {
    render(
      <CardHeader
        title="Waiting on a decision"
        note="All read"
        caption="Syndra found these on its last check and changes nothing until you decide."
      />,
    );
    const caption = screen.getByText(/changes nothing until you decide/);
    expect(caption.tagName).toBe("P");
    expect(caption).toHaveClass("measure", "basis-full");
    // After the title, the note and any action: its own line, under them.
    expect(caption.parentElement!.lastElementChild).toBe(caption);
  });
});

describe("a setting row", () => {
  it("puts its way on in the action seat, and its label in the label seat", () => {
    const { container } = render(
      <SettingRow label="Home folders" action={<Button size="sm">Choose folder</Button>}>
        /mnt/tank/home
      </SettingRow>,
    );
    const row = container.firstElementChild!;
    expect(screen.getByText("Home folders")).toHaveClass("w-(--seat-label)");
    expect(row.lastElementChild).toHaveAttribute("data-action-seat");
    expect(row.lastElementChild).toHaveClass("row-seat");
    expect(controlsOutsideSeat(row)).toEqual([]);
  });
});

describe("no control in a sentence", () => {
  // A Notice and an EmptyState end on their action: a message is read top to
  // bottom and ends on its step. That step is still never inside a sentence.
  it("holds for the shared parts that carry a step", () => {
    const { container } = render(
      <>
        <Notice tone="warn" title="TrueNAS is not answering" next="Check the NAS." action={{ label: "Read again", onClick: () => {} }}>
          Syndra keeps what it read last.
        </Notice>
        <EmptyState title="Nothing waiting on you" guidance="Syndra checks again on its own." action={{ label: "Check now", onClick: () => {} }} />
        <Card>
          <CardHeader title="Accounts" caption="Every account Syndra made on this system." />
          <CardRow first action={<Button size="sm">Show</Button>} detail="Made by Syndra.">
            <span>roman</span>
          </CardRow>
        </Card>
      </>,
    );
    expect(screen.getAllByRole("button")).toHaveLength(3);
    expect(controlsInProse(container)).toEqual([]);
  });
});

describe("a line under a status dot", () => {
  it("starts at the word: StandingLine's step is in StatusSeat's own column", () => {
    const { container } = render(
      <StandingLine standing={{ usable: false, waiting_on: "none", note: "Switched off on TrueNAS." }} system="TrueNAS" audience="member" />,
    );
    const step = screen.getByText("Ask makerspace staff if you did not expect this.");
    const grid = container.querySelector("[data-standing] > .grid")!;
    expect(grid.className).toContain("grid-cols-[var(--dot-indent)_1fr]");
    expect(step.closest(".col-start-2")?.parentElement).toBe(grid);
  });
});
