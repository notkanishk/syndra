// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EmptyState } from "@/components/states";
import { SettingRow } from "@/components/targets/SettingRow";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { ActionSeat, Card, CardColumns, CardGrid, CardHeader, CardRow, Cells, ROW } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { StandingLine } from "@/components/ui/StandingLine";
import { ACTIONS, actionsInProse, actionsOutsideSeats, cardGridsByHand, dotLinesOffIndent, headsOffRows, longNotes, NOTE_MAX, unsizedColumns } from "@/test-utils/layout";

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
        <CardColumns cols={["min-w-[120px] flex-1"]} seat="wide">
          <span>Account</span>
        </CardColumns>
        <CardRow first cols={["min-w-[120px] flex-1"]} seat="wide" action={<Button size="sm">Adopt</Button>}>
          <span>kofi</span>
        </CardRow>
        <CardRow cols={["min-w-[120px] flex-1"]} seat="wide">
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
        <CardColumns cols={["min-w-[120px] flex-1"]}>
          <span>Name</span>
        </CardColumns>
        <CardRow first cols={["min-w-[120px] flex-1"]}>
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

describe("cards side by side", () => {
  it("are top-aligned, never stretched to the tallest beside them, and as wide as their `min` allows", () => {
    const { container, rerender } = render(
      <CardGrid>
        <Card>
          <CardHeader title="Gaps" />
        </Card>
        <Card>
          <CardHeader title="Roles most people hold" />
        </Card>
      </CardGrid>,
    );
    const grid = container.querySelector("[data-card-grid]")!;
    expect(grid).toHaveClass("grid", "items-start", "gap-4", "grid-cols-[repeat(auto-fit,minmax(min(100%,420px),1fr))]");
    rerender(<CardGrid min="wide" />);
    expect(container.querySelector("[data-card-grid]")).toHaveClass("items-start", "grid-cols-[repeat(auto-fit,minmax(min(100%,600px),1fr))]");
  });

  it("are caught in a grid built by hand, and a band of stat tiles is left alone", () => {
    const { container, rerender } = render(
      <div className="grid gap-6 desktop:grid-cols-2">
        <Card>Gaps</Card>
        <div>
          <Card>Roles most people hold</Card>
        </div>
      </div>,
    );
    expect(cardGridsByHand(container)).toHaveLength(1);
    rerender(
      <div className="grid gap-4 [grid-template-columns:repeat(auto-fit,minmax(220px,1fr))] grid-cols-2">
        <div data-stat-tile className="card">12</div>
        <div data-stat-tile className="card">3</div>
      </div>,
    );
    expect(cardGridsByHand(container)).toEqual([]);
  });
});

describe("a seat's heading", () => {
  it("names what the seat holds, at the seat's width, read by assistive tech, when the table says so", () => {
    const { container } = render(
      <CardColumns cols={["min-w-[160px] flex-1"]} seat="wide" seatLabel="Needs you">
        <span>System</span>
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

describe("the column guard", () => {
  const cols = ["tablet:w-[180px] tablet:flex-none", "min-w-0 tablet:min-w-[120px] tablet:flex-1", "tablet:w-(--seat-number) tablet:text-right"];

  it("draws each heading and each cell in its column's box, and passes a table on one list and one seat", () => {
    const { container } = render(
      <Card>
        <CardColumns cols={cols} seat="wide">
          <span>Who</span>
          <span>What</span>
          <span>Found</span>
        </CardColumns>
        <CardRow first cols={cols} seat="wide" action={<Button size="sm">Adopt</Button>} detail="Maker not known.">
          <span>Bianca Belair</span>
          <span>Laser cutter</span>
          <span>3 Nov</span>
        </CardRow>
        <div className={`flex flex-wrap ${ROW}`}>
          <Cells cols={cols}>
            <span>Kofi Kingston</span>
            <span>Wood shop</span>
            <span>28 Oct</span>
          </Cells>
          <ActionSeat seat="wide" />
        </div>
      </Card>,
    );
    const head = container.querySelector("[data-card-columns]")!;
    const row = container.querySelector("[data-card-row]")!;
    for (const line of [head, row]) {
      expect(Array.from(line.querySelectorAll(":scope > [data-col]"), (cell) => cell.className)).toEqual(cols);
    }
    expect(screen.getByText("Found").parentElement).toHaveClass("tablet:text-right");
    expect(headsOffRows(container)).toEqual([]);
  });

  it("catches a heading on columns of its own, a heading with no seat cell, and a part out of its column", () => {
    // Drift on staging, 19 Nov: widths of the heading's own, and no cell over the seat. A heading can
    // no longer be typed without a column list (`cols` is required: the type below), so the drift
    // left is a list that is not its rows'.
    const { container, rerender } = render(
      <Card>
        <CardColumns cols={["w-[180px]", "w-[120px]"]}>
          <span>Who</span>
          <span>What</span>
        </CardColumns>
        <CardRow first cols={cols.slice(0, 2)} seat="wide" action={<Button size="sm">Adopt</Button>}>
          <span>Bianca Belair</span>
          <span>Laser cutter</span>
        </CardRow>
      </Card>,
    );
    expect(headsOffRows(container)).toEqual([
      `a row off its headings' columns: [${cols[0]} | ${cols[1]}] under [w-[180px] | w-[120px]]: "Bianca BelairLaser cutterAdopt"`,
      'a row\'s seat (wide) under a heading seat of none: "Bianca BelairLaser cutterAdopt"',
    ]);
    // Requests: an avatar drawn as a part of its own pushed every cell after it out from under its heading.
    rerender(
      <Card>
        <CardColumns cols={cols.slice(0, 2)}>
          <span>Who</span>
          <span>What</span>
        </CardColumns>
        <div className={`flex flex-wrap ${ROW}`}>
          <Cells cols={cols.slice(0, 2)}>
            <span>B</span>
            <span>Bianca Belair</span>
            <span>Laser cutter</span>
          </Cells>
        </div>
      </Card>,
    );
    expect(headsOffRows(container)).toHaveLength(1);
    cleanup(); // the offenders were drawn on purpose
  });

  it("adds the checkbox's cell in select mode without remounting the cells after it", () => {
    const row = (selecting: boolean) => (
      <CardRow first cols={selecting ? ["w-[26px]", ...cols] : cols}>
        {selecting && <input type="checkbox" aria-label="Select" />}
        <span>Bianca Belair</span>
        <span>Laser cutter</span>
        <span>3 Nov</span>
      </CardRow>
    );
    const { rerender } = render(row(false));
    const who = screen.getByText("Bianca Belair").parentElement;
    rerender(row(true));
    expect(screen.getByText("Bianca Belair").parentElement).toBe(who);
    expect(who).toHaveClass(cols[0]);
  });

  it("wraps the headings as the rows wrap, the seat's cell to a second line with theirs", () => {
    // jsdom draws no lines, so the guard reads each line's wrap at tablet and at desktop.
    const table = (head: string, row: string) => (
      <Card>
        <CardColumns cols={cols.slice(0, 1)} className={head}>
          <span>Who</span>
        </CardColumns>
        <div className={`${row} ${ROW}`}>
          <Cells cols={cols.slice(0, 1)}>
            <span>Bianca Belair</span>
          </Cells>
        </div>
      </Card>
    );
    const { container, rerender } = render(table("", "flex flex-wrap"));
    expect(container.querySelector("[data-card-columns]")).toHaveClass("flex-wrap");
    expect(headsOffRows(container)).toEqual([]);
    // Requests at 1024 and 1280: rows that wrap under headings that never did.
    rerender(table("tablet:flex-nowrap", "flex flex-wrap"));
    expect(headsOffRows(container)).toEqual([
      'a row that wraps yes at tablet, yes at desktop under headings that wrap no at tablet, no at desktop: "Bianca Belair"',
    ]);
    // A table whose rows wrap below desktop and shrink from it says the same of its headings.
    rerender(table("", "flex tablet:flex-wrap desktop:flex-nowrap"));
    expect(headsOffRows(container)).toHaveLength(1);
    rerender(table("desktop:flex-nowrap", "flex tablet:flex-wrap desktop:flex-nowrap"));
    expect(headsOffRows(container)).toEqual([]);
    cleanup();
  });

  it("holds every column to a width or a minimum: a share with neither starves, a column sized by its words wraps alone", () => {
    // Staging, 19 Nov: a person's role row gave its source (`min-w-0 flex-1`) 36px at 1100, under a
    // 94px chip; Connected systems' System heading had no size and its row's had 220.
    const table = (columns: string[]) => (
      <CardColumns cols={columns}>
        {columns.map((c) => (
          <span key={c}>{c || "words"}</span>
        ))}
      </CardColumns>
    );
    const { container, rerender } = render(table(["min-w-0 flex-1", "", "text-right"]));
    expect(unsizedColumns(container)).toEqual([
      'a column with no size (a width, or a min-width for one that flexes): [min-w-0 flex-1] "min-w-0 flex-1"',
      'a column with no size (a width, or a min-width for one that flexes): [] "words"',
      'a column with no size (a width, or a min-width for one that flexes): [text-right] "text-right"',
    ]);
    rerender(
      table([
        "w-full min-w-0 tablet:min-w-[140px] tablet:flex-1",
        "min-w-[min(160px,100%)] flex-[2_1_0%]",
        "tablet:w-(--seat-number) tablet:flex-none tablet:text-right",
        "hidden desktop:block desktop:w-[130px] desktop:flex-none",
        "hidden",
      ]),
    );
    expect(unsizedColumns(container)).toEqual([]);
    cleanup();
  });

  it("heads the rows' detail line under the columns, at the text's edge, never as a column", () => {
    // Drift on staging, 19 Nov: Why sat beside What over nothing at 1100, and stacked at 1280.
    const { container } = render(
      <Card>
        <CardColumns cols={cols} seat="wide" detail="Why">
          <span>Who</span>
          <span>What</span>
          <span>Found</span>
        </CardColumns>
        <CardRow first cols={cols} seat="wide" detail="Maker not known.">
          <span>Bianca Belair</span>
          <span>Laser cutter</span>
          <span>3 Nov</span>
        </CardRow>
      </Card>,
    );
    const head = container.querySelector("[data-card-columns]")!;
    const why = head.lastElementChild!;
    expect(why).toHaveTextContent("Why");
    expect(why).toHaveAttribute("data-row-detail");
    expect(why).toHaveClass("basis-full");
    expect(why).not.toHaveAttribute("data-col");
    expect(headsOffRows(container)).toEqual([]);
  });
});

// A heading is given its table's columns, or it does not compile: typed beside its rows, nineteen
// tables' headings drifted off them (layout audit, 19 Nov 2026). Never called; `tsc` reads it.
export function headingWithoutColumns() {
  // @ts-expect-error `cols` is required
  return <CardColumns><span>Who</span></CardColumns>;
}

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
