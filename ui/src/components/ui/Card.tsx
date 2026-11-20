import React from "react";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Count } from "@/components/ui/Badge";
import { Unfold } from "@/lib/motion";

/**
 * The container surface: 20px radius, one hairline, no padding of its own so
 * a card can hold either a padded body or a run of full-bleed rows.
 *
 * `Card` + `CardHeader` + `CardRow` is the shape every list on the product
 * uses — a header carrying the title, a count badge and an optional right-side
 * note, then one row per item separated by a divider.
 */

export function Card({
  className = "",
  stale,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & {
  /**
   * Drawn from the last read of a system that isn't answering (A-13 (d)). The
   * card recedes to the page and its dots become rings; its text stays at full
   * strength, under the page's Notice. Pass it (true or false) on any card that
   * can go stale, so it also rises back by tint. Never dim a stale card.
   */
  stale?: boolean;
}) {
  return (
    <div
      // The `card` frame carries the light under your hand (A-25 moment 2); a stale card is refused it.
      className={`card overflow-hidden ${className}`}
      data-stale={stale || undefined}
      data-recedes={stale === undefined ? undefined : ""}
      {...props}
    >
      {children}
    </div>
  );
}

export function CardHeader({
  title,
  count,
  tone = "accent",
  note,
  caption,
  action,
  className = "",
}: {
  title: React.ReactNode;
  count?: number;
  tone?: "accent" | "warn" | "danger";
  /**
   * A short fact at the right: "All read", a time. A sentence is a `caption`:
   * a long note sits right when it fits and drops left when it does not, so
   * where it lands depends on its words (D-1 §4, rule 7).
   */
  note?: React.ReactNode;
  /** The card's explanation, under its title, at the measure. */
  caption?: React.ReactNode;
  /** Right-aligned link or control. */
  action?: React.ReactNode;
  className?: string;
}) {
  const badgeTone =
    tone === "warn"
      ? "bg-warn text-warn-ink"
      : tone === "danger"
        ? "bg-danger text-danger-ink"
        : "bg-accent-dense text-accent-ink";

  return (
    <div className={`flex flex-wrap items-center gap-[11px] px-(--card-x) py-(--card-head-y) tablet:gap-y-2 ${className}`}>
      <span className="type-card-title">{title}</span>
      {/* Hollow at zero, filled otherwise — the same thing `CountChip` says on
          a region heading. A solid badge is an alarm, and a solid `0` beside
          "Not going to happen" is an alarm about nothing happening. The two
          renderings of one count disagreed only in this case, which is the case
          where the difference is the whole message. */}
      {count !== undefined && (
        <span
          data-testid="card-count"
          className={`rounded-pill px-2.5 py-0.5 text-fine font-bold ${
            count === 0 ? "border border-line-strong text-label" : badgeTone
          }`}
        >
          <Count n={count} />
        </span>
      )}
      <span className="flex-1" />
      {note && (
        <span data-card-note className="text-small text-faint">
          {note}
        </span>
      )}
      {action}
      {/* The detail line's shape (`row-detail`): a line of its own under the title, words at the measure. A
          `basis-full` capped by `measure` fitted beside a short title on a wide card and sat at the right. */}
      {caption && (
        <p data-prose data-caption className="row-detail">
          {caption}
        </p>
      )}
    </div>
  );
}

/**
 * One item in a card. The first row omits its divider.
 *
 * On a phone a row that carries more than two facts discloses the rest rather
 * than truncating them: `disclosure` is the body, `expanded` says whether it is
 * open, and `onToggle` makes the WHOLE row the control. Never a chevron alone —
 * a 16px target inside a 60px row is a row that looks tappable and mostly
 * is not.
 *
 * `onToggle` is optional, and its absence is the second shape rather than a
 * mistake: a row that already holds a button cannot also BE one — that nests a
 * button inside a button — so the row's own control opens the panel and the row
 * stays inert. Both shapes put the panel in the same place, which is the point.
 * A panel that opens under the last row of a list instead of under the row you
 * clicked is a panel about the wrong account.
 *
 * All three props are optional and additive, so every existing caller is
 * unchanged and renders exactly as it did.
 *
 * The row's anatomy (D-1 §4): `children` are its parts, `action` sits in the
 * seat at the right of its first line, `detail` is the sentence under it. A
 * row whose `onToggle` makes it a button takes neither, and the types say so:
 * a button cannot hold a button, and a `<p>` is not phrasing content.
 */
type RowProps = React.HTMLAttributes<HTMLDivElement> & {
  first?: boolean;
  disclosure?: React.ReactNode;
  expanded?: boolean;
  /**
   * The seat's width, which is the table's: the most verbs any of its rows can
   * show. Pass it on every row of the table, with or without an `action`, and
   * an empty seat keeps its place.
   */
  seat?: SeatWidth;
  /** More than one line: top-aligned from tablet, each part's first line on the seat's. */
  top?: boolean;
  /** The table's columns, the list its CardColumns reads: each child is drawn in its column's box. */
  cols?: Columns;
};

export type CardRowProps = RowProps &
  (
    | {
        onToggle?: undefined;
        /** A button or an ArrowLink, in the action seat: never inside a sentence. */
        action?: React.ReactNode;
        /** The sentence that explains the row, under its first line, at the measure. */
        detail?: React.ReactNode;
      }
    | { onToggle: () => void; action?: never; detail?: never }
  );

export function CardRow({
  className = "",
  first = false,
  disclosure,
  expanded = false,
  onToggle,
  action,
  seat,
  detail,
  top = false,
  cols,
  children,
  ...props
}: CardRowProps) {
  // One definition of the line, used by all three shapes. It used to be typed
  // twice, and the copies had already drifted: only one of them lifted its
  // 60px floor above the tablet breakpoint. A wrapped part sits a gap under the
  // line on a phone (its 44px control would otherwise touch the text above it)
  // and 4px under it from tablet. A `top` row is centred on a phone, where its
  // seat wraps, and top-aligned from tablet: one alignment per breakpoint.
  const line = `flex flex-wrap gap-y-(--row-gap) tablet:gap-y-1 ${ROW} items-center ${top ? "row-top tablet:items-start" : ""} ${className}`;
  const parts = (
    <>
      {cols ? <Cells cols={cols}>{children}</Cells> : children}
      {(action !== undefined || seat !== undefined) && <ActionSeat seat={seat}>{action}</ActionSeat>}
      {detail && (
        <p data-row-detail className="row-detail">
          {detail}
        </p>
      )}
    </>
  );

  if (!disclosure) {
    return (
      <div data-card-row className={`${line} ${first ? "" : "row-divider"}`} {...props}>
        {parts}
      </div>
    );
  }

  // Pushed, never overlapped. The rows below move down and the operator keeps
  // their place in the list; an overlay would cover the row they were comparing
  // this one against. `Unfold`: the height settles and the contents tint in,
  // because a card never scales.
  const panel = (
    <Unfold open={expanded}>
      <div className="px-5 pb-4 pt-1">{disclosure}</div>
    </Unfold>
  );

  // `first` on the inner line so the divider is not drawn twice: it belongs to
  // the wrapper now, which is what the next row sits under.
  if (!onToggle) {
    return (
      <div className={first ? "" : "row-divider"}>
        <div data-card-row className={line} {...props}>
          {parts}
        </div>
        {panel}
      </div>
    );
  }

  return (
    <div className={first ? "" : "row-divider"}>
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={expanded}
        className="w-full text-left hover-tint"
      >
        <div data-card-row className={line}>
          {parts}
        </div>
      </button>
      {panel}
    </div>
  );
}

/**
 * The quiet way out of a card — "See all", "Full audit log".
 *
 * Not a `ButtonLink`: a pill in a card header competes with the card's own
 * title, and this is a way onward rather than the card's action. It is still a
 * control, so it carries the touch floor.
 *
 * It existed twice, on two cards of the same dashboard, and the copies had
 * already disagreed about exactly that: one was a 20px-tall line of text.
 */
export function CardHeaderLink({ href, children }: { href: string; children: React.ReactNode }) {
  // The ArrowLink itself (A-17 G2): it draws the arrow, so callers never type "→".
  return <ArrowLink href={href}>{children}</ArrowLink>;
}

/**
 * Column headings for a table-shaped card.
 *
 * Hidden below the tablet breakpoint, centrally rather than by every caller.
 * A column header is a promise that the cells beneath it line up, and on a
 * phone they do not — the row's fields stack, and each one carries its own
 * label inside the disclosure instead.
 *
 * `cols` is the table's column list, the same one every row is given: each
 * heading is drawn in its column's box, so it sits exactly over its cells. It is
 * required: a heading typed in widths of its own beside its rows drifted off
 * them on nineteen tables (layout audit, 19 Nov 2026).
 *
 * `seat` draws the action column's heading cell at the rows' seat width, so the
 * headings over the other columns line up with their cells. It is empty (a
 * column of verbs needs no name) unless `seatLabel` names what the seat holds
 * ("Decision"), set at the seat's right edge, over its contents.
 *
 * It is `flex-wrap`, as every table row is: where a row's seat drops to a
 * second line, the seat's heading cell does too, so the columns above stay over
 * theirs. No table overrides it any more; the guard (`headsOffRows`) still
 * compares the headings' wrap with their rows' at tablet and at desktop, so a
 * line that stopped wrapping on one side only fails.
 *
 * `detail` heads the rows' detail line: under the column headings, full width,
 * from the text's edge, where every row's detail line is. A heading for what
 * the detail line says, set beside another column's heading, sat over nothing
 * at one width and stacked at the next (Drift's "Why", staging 19 Nov).
 */
export function CardColumns({
  className = "",
  seat,
  seatLabel,
  cols,
  detail,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & {
  seat?: SeatWidth;
  seatLabel?: React.ReactNode;
  cols: Columns;
  detail?: React.ReactNode;
}) {
  return (
    <div
      data-card-columns
      className={`row-cols hidden flex-wrap gap-y-1 px-(--card-x) py-2 text-fine font-semibold uppercase tracking-[0.09em] text-label tablet:flex ${className}`}
      {...props}
    >
      <Cells cols={cols}>{children}</Cells>
      {seat && (
        <span aria-hidden={seatLabel ? undefined : true} className="row-seat" data-seat={seatWidth(seat)}>
          {seatLabel}
        </span>
      )}
      {detail && (
        <span data-row-detail className="basis-full">
          {detail}
        </span>
      )}
    </div>
  );
}

/**
 * A table's columns, left to right before the action seat: each column's
 * classes (its width, its alignment, from which breakpoint it shows). One list
 * per table, given to its CardColumns and to every row, so a heading and the
 * cells under it are drawn in the same box and cannot drift apart. A heading
 * typed beside its rows, in widths of its own, drifted on Drift (staging,
 * 19 Nov): a heading over an empty column, a date out from under its label.
 */
export type Columns = readonly string[];

/*
 * How a column is sized, so a row never clips and its headings never leave it (layout audit,
 * 19 Nov 2026). A row's line is `flex-wrap`, and flex breaks a line on each part's basis held to its
 * min-width, not on what it says. So every column states its size: a fixed seat (`w-*`, `flex-none`)
 * where D-1 has one (a number, a status, a label), else a share held to a minimum
 * (`min-w-[N] flex-1`, or `flex-[2_1_0%]` for a bigger share), its text wrapping or, for a name,
 * truncated. Then the headings and every row break at the same widths, whatever they hold; the
 * line breaks exactly when the minimums, the fixed seats and the action seat no longer fit, so a
 * row whose words fit keeps its seat on its first line; and where the row cannot hold its columns
 * the seat drops under it, right-aligned, and then the last column: never clipped. A share with no
 * minimum (`min-w-0 flex-1`) reports 0 to the wrap and starves (a person's role row gave its source
 * 36px at 1100); a size from the content wraps the rows where the headings do not. The guard
 * (`unsizedColumns`) holds every heading's column to a width or a minimum.
 */

/**
 * A number or a date, right-aligned under a right-aligned heading (G16), from tablet. Below tablet it
 * has no width of its own: in a row that stacks it is a line of its own, and in a CardRow that wraps
 * it is as wide as its words.
 */
export const NUMBER_COL = "tablet:w-(--seat-number) tablet:flex-none tablet:text-right";
/** A status a table can say, as wide as its longest (D-1 §5 #22), from tablet; as wide as its words below. */
export const STATUS_COL = "tablet:w-(--seat-status) tablet:flex-none";

/**
 * The parts of a header or a row, each in its column's box (`data-col`). A row
 * on `ROW` that is not a CardRow (a link, a label) draws its parts through it
 * too. A part that is sometimes absent keeps its place with an empty cell,
 * never by being left out: the guard counts the boxes against the list.
 */
export function Cells({ cols, children }: { cols: Columns; children: React.ReactNode }) {
  // Keyed by the part's own key (its place in what was written), so a checkbox column arriving
  // in select mode adds a cell rather than remounting every one after it.
  return React.Children.toArray(children).map((child, i) => (
    <span key={React.isValidElement(child) ? child.key : i} data-col className={cols[i]}>
      {child}
    </span>
  ));
}

/**
 * The seat's width: one of three, from the table (D-1 §4). `default` is one or
 * two verbs, `narrow` one short verb or one link, `wide` three verbs.
 */
export type SeatWidth = "narrow" | "default" | "wide";

function seatWidth(seat: SeatWidth | undefined): "narrow" | "wide" | undefined {
  return seat === "default" ? undefined : seat;
}

/**
 * The action seat: the row's last part, right-aligned, on its first line.
 * CardRow draws it; a list row built on `ROW` (a link, a row that stacks on a
 * phone) puts its action in one, so no page types `row-seat` itself.
 */
export function ActionSeat({ seat, children }: { seat?: SeatWidth; children?: React.ReactNode }) {
  return (
    <div data-action-seat className="row-seat" data-seat={seatWidth(seat)}>
      {children}
    </div>
  );
}

/**
 * A list row's box: the touch floor on a phone (lifted from tablet up), the
 * card's inset and the row's padding (D-1 §3), and the table's column gap
 * (`row-cols`, the one CardColumns reads, so a heading sits over its cell).
 * CardRow is built on it; a list page whose row is its own element (a link, a
 * row that stacks on a phone) takes it too, and states only its vertical gap
 * (`gap-y-*`), so no row keeps a 60px floor where a pointer is, and none
 * carries its own idea of the numbers.
 */
export const ROW = "row-cols min-h-[60px] px-(--card-x) py-(--row-y) tablet:min-h-0";

/**
 * Cards side by side: as many across as can each be `min` wide (`pair` 420, two
 * on a 1440 page; `wide` 600, two only from the 1800 frame), one under the other
 * where they cannot, 16 apart.
 *
 * Top-aligned, always (`items-start`): a card is as tall as what it says, never
 * stretched to the tallest beside it, so a one-line card stays one line high and
 * the space under it is the page's, not an empty card's (staging, 18 Nov: Home's
 * "Gaps" stood 370px tall beside a long list). Rows first, in the order given: a
 * CSS column flow would fill the short card's gap, but it moves a card from one
 * column to the other whenever a count changes (structure moving with data),
 * reads down before across, and cuts a card's fold animation at a column break.
 */
export function CardGrid({
  min = "pair",
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & { min?: "pair" | "wide" }) {
  return (
    <div data-card-grid className={`grid items-start gap-4 ${CARD_GRID_COLUMNS[min]} ${className}`} {...props}>
      {children}
    </div>
  );
}

const CARD_GRID_COLUMNS = {
  pair: "grid-cols-[repeat(auto-fit,minmax(min(100%,420px),1fr))]",
  wide: "grid-cols-[repeat(auto-fit,minmax(min(100%,600px),1fr))]",
} as const;

/**
 * The label/value pair a disclosed row is built from.
 *
 * A field that was a column on a wide screen becomes one of these, and the
 * label is what the column heading used to say — otherwise a disclosed value
 * is a string with no idea what it is.
 */
export function RowField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 py-1.5">
      <span className="type-label">{label}</span>
      <span className="text-small text-ink">{children}</span>
    </div>
  );
}
