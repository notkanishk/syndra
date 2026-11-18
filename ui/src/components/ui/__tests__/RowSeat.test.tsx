// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EmptyState } from "@/components/states";
import { SettingRow } from "@/components/targets/SettingRow";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { ActionSeat, Card, CardColumns, CardHeader, CardRow, ROW } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { StandingLine } from "@/components/ui/StandingLine";
import { ACTIONS, actionsInProse, actionsOutsideSeats, dotLinesOffIndent, longNotes, NOTE_MAX } from "@/test-utils/layout";

/**
 * D-1 §4: every action sits in its row's right seat on the first line, never in
 * a sentence and never on a row of its own; the seat's width is the table's.
 * jsdom has no layout, so these hold the structure the layout is drawn from.
 * The two layout guards also run after every test in the suite (test-setup.ts);
 * here they are held to what they must catch and what they must let pass.
 */

/** Every action in a row that is not in the row's seat (a form or a dialog is its own row). */
function actionsOutsideSeat(row: Element): Element[] {
  return Array.from(row.querySelectorAll(ACTIONS)).filter(
    (action) => !action.closest('[data-action-seat], form, [role="dialog"]'),
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
    expect(actionsOutsideSeat(row)).toEqual([]);
    expect(actionsInProse(row)).toEqual([]);
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

  it("top-aligns a row of more than one line from tablet, and centres it on a phone", () => {
    const { container, rerender } = render(
      <CardRow first top action={<Button size="sm">Decide</Button>}>
        <span>A long finding</span>
      </CardRow>,
    );
    const row = () => container.firstElementChild!;
    // One alignment per breakpoint: centred on a phone, where the seat wraps under the row.
    expect(row()).toHaveClass("row-top", "items-center", "tablet:items-start");
    rerender(
      <CardRow first action={<Button size="sm">Decide</Button>}>
        <span>A short finding</span>
      </CardRow>,
    );
    expect(row()).toHaveClass("items-center");
    expect(row()).not.toHaveClass("row-top");
    expect(row()).not.toHaveClass("tablet:items-start");
  });

  it("keeps the seat in a row that discloses the rest of itself", () => {
    render(
      <CardRow first disclosure={<span>body</span>} action={<ArrowLink href="/drift">Decide on Drift</ArrowLink>}>
        <span>Unexplained roles</span>
      </CardRow>,
    );
    expect(screen.getByRole("link", { name: /Decide on Drift/ }).closest("[data-action-seat]")).not.toBeNull();
  });

  it("refuses an action or a detail line on a row that is itself a button", () => {
    // Never rendered: tsc is the check (`bun run typecheck`, and `next build`).
    const nested = () => (
      // @ts-expect-error a button row cannot hold a button
      <CardRow onToggle={() => {}} action={<Button size="sm">Decide</Button>}>
        <span>Laser cutter</span>
      </CardRow>
    );
    const paragraph = () => (
      // @ts-expect-error a <p> is not phrasing content, so not inside a button
      <CardRow onToggle={() => {}} detail="Why">
        <span>Laser cutter</span>
      </CardRow>
    );
    expect([nested, paragraph]).toHaveLength(2);
  });

  it("is built on ROW, the box and column grid every list row takes, and the headings read the same grid", () => {
    const { container } = render(
      <Card>
        <CardColumns>
          <span>Name</span>
        </CardColumns>
        <CardRow first>
          <span>Becky Lynch</span>
        </CardRow>
      </Card>,
    );
    const [columns, row] = Array.from(container.firstElementChild!.children);
    for (const c of ROW.split(" ")) expect(row).toHaveClass(c);
    expect(ROW).toContain("tablet:min-h-0");
    expect(ROW).toContain("row-cols");
    expect(columns).toHaveClass("row-cols");
    // Neither states a column gap of its own beside the grid's.
    for (const el of [columns, row]) expect(el.className).not.toMatch(/(?:^|\s)(?:[a-z]+:)?gap-(?!y-)/);
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
    expect(caption).toHaveClass("row-detail");
    // After the title, the note and any action: its own line, under them.
    expect(caption.parentElement!.lastElementChild).toBe(caption);
  });
});

describe("a setting row", () => {
  it("is a CardRow: its way on in the action seat, its label in the label seat", () => {
    const { container } = render(
      <SettingRow label="Home folders" action={<Button size="sm">Choose folder</Button>}>
        /mnt/tank/home
      </SettingRow>,
    );
    const row = container.firstElementChild!;
    for (const c of ROW.split(" ")) expect(row).toHaveClass(c);
    expect(screen.getByText("Home folders")).toHaveClass("w-(--seat-label)");
    expect(row.lastElementChild).toHaveAttribute("data-action-seat");
    expect(row.lastElementChild).toHaveClass("row-seat");
    expect(actionsOutsideSeat(row)).toEqual([]);
  });

  it("keeps its seat when it has no way on", () => {
    const { container } = render(<SettingRow label="Name">TrueNAS</SettingRow>);
    expect(container.querySelector("[data-action-seat]")).toBeEmptyDOMElement();
  });
});

describe("the prose guard", () => {
  it("passes the shared parts that carry a step: a message ends on its action, never inside a sentence", () => {
    const { container } = render(
      <>
        <Notice tone="warn" title="TrueNAS is not answering" next="Check the NAS." action={{ label: "Read again", onClick: () => {} }}>
          Syndra keeps what it read last.
        </Notice>
        <EmptyState title="Nothing waiting on you" guidance="Syndra checks again on its own." action={{ label: "Check now", onClick: () => {} }} />
        <Card>
          <CardHeader title="Accounts" caption={<>Every account Syndra made. <ArrowLink href="/drift">Drift</ArrowLink></>} />
          <CardRow first action={<Button size="sm">Show</Button>} detail="Made by Syndra.">
            <span>roman</span>
          </CardRow>
        </Card>
      </>,
    );
    expect(container.querySelectorAll("button.rounded-pill")).toHaveLength(3);
    expect(actionsInProse(container)).toEqual([]);
  });

  it("catches an action in a sentence, in a paragraph or in an inline run of text", () => {
    const { container } = render(
      <div>
        <p>
          Syndra checks on its own. <Button size="sm">Check now</Button>
        </p>
        <p>
          <span>Two ways there:</span>
          <ArrowLink href="/governance/pending">Pending changes</ArrowLink>
        </p>
        <span>
          In Laser Lab. <ArrowLink href="/apps/1">Open it</ArrowLink>
        </span>
        <div>
          No mapping can give one yet.{" "}
          <span>
            <ArrowLink href="/system">Choose where</ArrowLink>
          </span>
        </div>
      </div>,
    );
    expect(actionsInProse(container)).toEqual([
      'button "Check now" in "Syndra checks on its own. Check now"',
      'a "Pending changes→" in "Two ways there:Pending changes→"',
      'a "Open it→" in "In Laser Lab. Open it→"',
      'a "Choose where→" in "Choose where→"',
    ]);
    cleanup(); // the offenders were drawn on purpose; the suite-wide guard reads only what a test leaves
  });

  it("lets a link sit in a lede or caption, never a button", () => {
    const { container } = render(
      <p data-prose>
        Read live. <ArrowLink href="/users">See Syndra&rsquo;s view</ArrowLink> <Button size="sm">Read again</Button>
      </p>,
    );
    expect(actionsInProse(container)).toEqual(['button "Read again" in "Read live. See Syndra’s view→ Read again"']);
    cleanup();
  });

  it("lets a row's parts sit side by side", () => {
    const { container } = render(
      <div>
        <span>Roman Reigns</span>
        <span>Ready to use</span>
        <Button size="sm">Extend</Button>
      </div>,
    );
    expect(actionsInProse(container)).toEqual([]);
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

  it("is caught at any number of its own, and passes at the token", () => {
    const line = (pad: string) => (
      <span>
        <span className="inline-flex items-center gap-2">
          <span data-dot />
          Answering
        </span>
        <span className={pad}>Last answered a minute ago</span>
      </span>
    );
    const { container, rerender } = render(line("pl-[18px] text-faint"));
    expect(dotLinesOffIndent(container)).toEqual(['"Last answered a minute ago" under a status dot at pl-[18px]']);
    rerender(line("pl-4 text-faint"));
    expect(dotLinesOffIndent(container)).toEqual(['"Last answered a minute ago" under a status dot at pl-4']);
    rerender(line("pl-(--dot-indent) text-faint"));
    expect(dotLinesOffIndent(container)).toEqual([]);
  });
});

describe("ActionSeat", () => {
  it("is the one seat a list row built on ROW takes", () => {
    const { container } = render(
      <ActionSeat seat="narrow">
        <Button size="sm">Withdraw</Button>
      </ActionSeat>,
    );
    expect(container.firstElementChild).toHaveAttribute("data-action-seat");
    expect(container.firstElementChild).toHaveAttribute("data-seat", "narrow");
  });
});

describe("a seat's heading", () => {
  it("names what the seat holds, at the seat's width, read by assistive tech, when the table says so", () => {
    const { container } = render(
      <CardColumns seat="wide" seatLabel="Needs you">
        <span className="flex-1">System</span>
      </CardColumns>,
    );
    const cell = container.querySelector(".row-seat")!;
    expect(cell).toHaveAttribute("data-seat", "wide");
    expect(cell).toHaveTextContent("Needs you");
    expect(cell).not.toHaveAttribute("aria-hidden");
  });
});

describe("the seat guard", () => {
  it("catches an action in a CardRow that is not in its seat, and passes one that is", () => {
    const { container, rerender } = render(
      <CardRow first>
        <span>Check for differences</span>
        <Button size="sm">Check now</Button>
      </CardRow>,
    );
    expect(actionsOutsideSeats(container)).toEqual([
      'button "Check now" in a row, outside its seat: "Check for differencesCheck now"',
    ]);
    rerender(
      <CardRow first action={<Button size="sm">Check now</Button>}>
        <span>Check for differences</span>
      </CardRow>,
    );
    expect(actionsOutsideSeats(container)).toEqual([]);
  });

  it("lets a form be its own row, and leaves a value's copy control and a fold's toggle to their own", () => {
    const { container } = render(
      <CardRow first>
        <form>
          <Button size="sm">Save name</Button>
        </form>
        <button type="button" className="rounded-pill" data-inline>
          Copy id
        </button>
        <button type="button" className="rounded-pill" aria-expanded={false}>
          Show evidence
        </button>
      </CardRow>,
    );
    expect(container.querySelectorAll(ACTIONS)).toHaveLength(2); // the fold toggle is an action, judged in prose, not by the seat
    expect(actionsOutsideSeats(container)).toEqual([]);
  });
});

describe("the note guard", () => {
  it("fails a CardHeader note over 48 characters, pointing at caption, and passes a short fact", () => {
    const sentence = "Compares TrueNAS with what Syndra expects and records any fixes.";
    const { container, rerender } = render(<CardHeader title="Check for differences" note={sentence} />);
    expect(sentence.length).toBeGreaterThan(NOTE_MAX);
    expect(longNotes(container)).toEqual([`a CardHeader note of ${sentence.length} characters (use caption): "${sentence}"`]);
    rerender(<CardHeader title="Check for differences" note="All read" caption={sentence} />);
    expect(longNotes(container)).toEqual([]);
  });
});
