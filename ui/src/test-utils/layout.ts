/**
 * D-1's layout guards, read from the rendered tree. `test-setup.ts` runs them
 * after every test, over whatever that test rendered, so every page and part
 * the suite draws is checked, not a fixture built to pass.
 *
 * jsdom has no layout, so each rule is read from structure and class names.
 */

/** An action a reader presses: a pill button, a link drawn as a pill, or an ArrowLink. */
export const ACTIONS = "button.rounded-pill, a.rounded-pill, a[data-arrow-link]";

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
 * Found before D-1's seat pass and moved into seats by it (§5; tasks.md 7u-2).
 * Each is matched on its failure line; pass 2 empties this list.
 */
export const PASS_2_SEATS: Array<{ match: RegExp; why: string }> = [
  { match: /^button "Open Maintenance/, why: "§5 #1, TargetOverview's reach row" },
  { match: /^a "Open People→" in "Nothing for you to do\./, why: "§5 #8, the usage card's end row" },
  { match: /^a "Choose where home folders go→"/, why: "HomeMapping, the mapping dialog's home row (found 18 Nov)" },
  { match: /^a "Syndra’s explanation of the same access→"/, why: "zitadel/users, the person's intro row (found 18 Nov)" },
  { match: /^a "[^"]*→" in "Inside Syndra: /, why: "zitadel, the doors' rows (found 18 Nov)" },
];

/** The guards' findings in `root`, less the pass-2 list. */
export function layoutFindings(root: Element): string[] {
  return [...actionsInProse(root), ...dotLinesOffIndent(root)].filter(
    (finding) => !PASS_2_SEATS.some(({ match }) => match.test(finding)),
  );
}

function describe(action: Element): string {
  return `${action.tagName.toLowerCase()} "${words(action)}"`;
}

function words(node: Element | null): string {
  return (node?.getAttribute("aria-label") ?? node?.textContent ?? "").trim().replace(/\s+/g, " ").slice(0, 120);
}
