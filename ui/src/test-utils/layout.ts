/**
 * D-1's layout guards, read from the rendered tree. `test-setup.ts` runs them
 * after every test, over whatever that test rendered, so every page and part
 * the suite draws is checked, not a fixture built to pass.
 *
 * jsdom has no layout, so each rule is read from structure and class names.
 */

/**
 * An action a reader presses: a pill button, a link drawn as a pill, or an ArrowLink. Not a control
 * that belongs to the words beside it (`data-inline`: a value's copy, a line's retry).
 */
export const ACTIONS = ":is(button.rounded-pill, a.rounded-pill, a[data-arrow-link]):not([data-inline])";

/**
 * D-1 §4: an action is never inside a sentence. A sentence wraps, and an action
 * inside one moves with its words: to the end of a line one day, onto a line
 * of its own the next.
 *
 * In prose means a `<p>` ancestor, or text of its own (a non-blank text node)
 * in an inline element around the action or in the first block that holds
 * them: text running before or after it in one inline flow. A row keeps its
 * words in their own elements, so a seat beside a name passes, and
 * `In {where}. <ArrowLink>` and `{sentence}{" "}<Button>` fail.
 *
 * A link (never a button) inside a lede, a caption or a meta line is prose by
 * design (§4): the shared parts mark those `data-prose`.
 */
export function actionsInProse(root: Element): string[] {
  const found: string[] = [];
  for (const action of Array.from(root.querySelectorAll(ACTIONS))) {
    if (action.tagName === "A" && action.closest("[data-prose]")) continue;
    if (inProse(action, root)) found.push(`${describe(action)} in "${words(action.closest("p") ?? action.parentElement)}"`);
  }
  return found;
}

/** Elements that flow inline: text in one of them, or in the block that holds them, is one line with the action. */
const INLINE = new Set(["SPAN", "A", "STRONG", "EM", "B", "I", "SMALL", "LABEL", "CODE", "TIME", "ABBR"]);

function inProse(action: Element, root: Element): boolean {
  if (action.parentElement?.closest("p")) return true;
  // Up through the inline elements around the action, to the first block that holds them, and no further: a
  // block's text beside another block is another line.
  for (let node = action.parentElement; node && root.contains(node); node = node.parentElement) {
    if (Array.from(node.childNodes).some((child) => child.nodeType === 3 && child.textContent!.trim() !== "")) return true;
    if (!INLINE.has(node.tagName)) return false;
  }
  return false;
}

/**
 * D-1 §4 rule 5: a line under a status dot starts at the word, `--dot-indent`.
 * The dot's own line is the dot's parent; a later sibling of that line that
 * states a left padding states it as the token, never a number of its own.
 */
export function dotLinesOffIndent(root: Element): string[] {
  const found: string[] = [];
  for (const dot of Array.from(root.querySelectorAll("[data-dot]"))) {
    const line = dot.parentElement;
    for (let next = line?.nextElementSibling; next; next = next.nextElementSibling) {
      const padding = Array.from(next.classList).filter((c) => /^pl-/.test(c) && c !== "pl-(--dot-indent)");
      if (padding.length > 0) found.push(`"${words(next)}" under a status dot at ${padding.join(" ")}`);
    }
  }
  return found;
}

/**
 * D-1 §4 rules 1 and 6: in a row (CardRow's line, `data-card-row`), every action is
 * in the action seat, or in a form, which is its own row. An action anywhere
 * else in the row has a place that depends on the words beside it.
 */
export function actionsOutsideSeats(root: Element): string[] {
  const found: string[] = [];
  for (const row of Array.from(root.querySelectorAll("[data-card-row]"))) {
    for (const action of Array.from(row.querySelectorAll(ACTIONS))) {
      // A dialog's rows are A-22's, not D-1's. A fold's own toggle (`aria-expanded`, G3) belongs to the
      // row it opens, wherever the row puts it; in a sentence it is still caught (actionsInProse).
      if (action.matches("[aria-expanded]") || action.closest('[data-action-seat], form, [role="dialog"]')) continue;
      found.push(`${describe(action)} in a row, outside its seat: "${words(row)}"`);
    }
  }
  return found;
}

/**
 * D-1 §4 rule 7: a CardHeader's `note` is a short fact ("All read", a time);
 * a sentence is its `caption`. A long note sits right when it fits and drops
 * left when it does not, so where it lands depends on its words.
 */
export const NOTE_MAX = 48;

export function longNotes(root: Element): string[] {
  return Array.from(root.querySelectorAll("[data-card-note]"))
    .map((note) => words(note))
    .filter((text) => text.length > NOTE_MAX)
    .map((text) => `a CardHeader note of ${text.length} characters (use caption): "${text}"`);
}

/**
 * Cards side by side go through `CardGrid`, which top-aligns them: a grid built
 * by hand stretched a one-line card to the height of the list beside it (Home's
 * "Gaps", 370px; staging, 18 Nov). A grid that splits into columns and holds two
 * or more cards (directly, or each in one wrapper, as Home's folding blocks are)
 * is CardGrid's. A band of stat tiles (`data-stat-tile`) is equal tiles, not cards.
 */
export function cardGridsByHand(root: Element): string[] {
  const found: string[] = [];
  for (const grid of Array.from(root.querySelectorAll(".grid:not([data-card-grid])"))) {
    if (!/(?:^|\s)(?:[a-z]+:)?grid-cols-(?:2|\[repeat\(auto-fit)/.test(grid.getAttribute("class") ?? "")) continue;
    const cards = Array.from(grid.children).filter((cell) => {
      const card = cell.matches(".card") ? cell : cell.firstElementChild?.matches(".card") ? cell.firstElementChild : null;
      return card !== null && !card.matches("[data-stat-tile]");
    });
    if (cards.length >= 2) found.push(`${cards.length} cards side by side in a grid built by hand (use CardGrid): "${words(grid).slice(0, 60)}"`);
  }
  return found;
}

/**
 * D-1 §4 rule 2, and the column template: a table's headings and every row under
 * them are drawn on one column list (`Columns`, each part in its `data-col` box)
 * and one seat width, so a heading sits over its cells. A heading typed beside
 * its rows drifted on Drift (staging, 19 Nov): a column named over nothing, a
 * date out from under its label, no heading cell over the seat.
 *
 * For each CardColumns, every row after it in its card (a CardRow, or a row
 * drawing its parts through `Cells`) has the same boxes, class for class, and
 * the same seat, or none where the headings have none.
 */
export function headsOffRows(root: Element): string[] {
  const found: string[] = [];
  for (const head of Array.from(root.querySelectorAll("[data-card-columns]"))) {
    const card = head.closest(".card") ?? head.parentElement!;
    const rows = new Set([
      ...Array.from(card.querySelectorAll("[data-card-row]")),
      ...Array.from(card.querySelectorAll("[data-col]"), (cell) => cell.parentElement!),
    ]);
    const want = shape(head);
    for (const row of rows) {
      if (row === head || row.closest(".card") !== head.closest(".card")) continue;
      if (!(head.compareDocumentPosition(row) & Node.DOCUMENT_POSITION_FOLLOWING)) continue;
      const got = shape(row);
      if (got.cols !== want.cols) found.push(`a row off its headings' columns: [${got.cols}] under [${want.cols}]: "${words(row).slice(0, 60)}"`);
      if (got.seat !== want.seat) found.push(`a row's seat (${got.seat}) under a heading seat of ${want.seat}: "${words(row).slice(0, 60)}"`);
      if (got.wrap !== want.wrap) found.push(`a row that wraps ${got.wrap} under headings that wrap ${want.wrap}: "${words(row).slice(0, 60)}"`);
    }
  }
  return found;
}

/**
 * A header's or a row's column boxes, its seat's width ("none" without one), and
 * whether its line wraps at tablet and at desktop (the header is hidden below
 * tablet). A header that never wraps over rows that do ran past its edge where
 * the rows dropped their seat to a second line (Requests at 1024 and 1280).
 */
function shape(line: Element): { cols: string; seat: string; wrap: string } {
  const parts = Array.from(line.children);
  const seat = parts.find((part) => part.matches(".row-seat"));
  const classes = (line.getAttribute("class") ?? "").split(/\s+/);
  const wrapAt = (...prefixes: string[]) => {
    for (const prefix of prefixes) {
      if (classes.includes(`${prefix}flex-nowrap`)) return "no";
      if (classes.includes(`${prefix}flex-wrap`)) return "yes";
    }
    return "no";
  };
  return {
    cols: parts.filter((part) => part.matches("[data-col]")).map((cell) => cell.getAttribute("class") ?? "").join(" | "),
    seat: seat ? (seat.getAttribute("data-seat") ?? "default") : "none",
    wrap: `${wrapAt("tablet:", "")} at tablet, ${wrapAt("desktop:", "tablet:", "")} at desktop`,
  };
}

/** The guards' findings in `root`. */
export function layoutFindings(root: Element): string[] {
  return [
    ...actionsInProse(root),
    ...dotLinesOffIndent(root),
    ...actionsOutsideSeats(root),
    ...longNotes(root),
    ...cardGridsByHand(root),
    ...headsOffRows(root),
  ];
}

function describe(action: Element): string {
  return `${action.tagName.toLowerCase()} "${words(action)}"`;
}

function words(node: Element | null): string {
  return (node?.getAttribute("aria-label") ?? node?.textContent ?? "").trim().replace(/\s+/g, " ").slice(0, 120);
}
