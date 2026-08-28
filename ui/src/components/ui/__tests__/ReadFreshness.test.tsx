// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  blocksIrreversibleAction,
  classifyRead,
  ReadFreshness,
} from "@/components/ui/ReadFreshness";

/**
 * §31 A — one answer to "how old is this", used everywhere state is read from a
 * target.
 *
 * The rules under test are the ones that were previously decided three separate
 * times on three screens two clicks apart.
 */

// Real wall-clock, because the rendered strip reads the real clock: a fixed
// NOW would test the pure classifier twice and the component not at all.
const NOW = Date.now();
const ago = (minutes: number) => new Date(NOW - minutes * 60_000).toISOString();

describe("classifying a read", () => {
  it("calls a read from the last minute live", () => {
    expect(classifyRead({ readAt: ago(0.5), current: true }, NOW)).toBe("live");
  });

  it("calls a read from a few minutes ago ageing, not stale", () => {
    expect(classifyRead({ readAt: ago(4), current: true }, NOW)).toBe("ageing");
  });

  it("calls a read past the window stale", () => {
    expect(classifyRead({ readAt: ago(11), current: true }, NOW)).toBe("stale");
  });

  // A four-second-old copy of state we could not reach is still a copy. Ageing
  // it into `live` is the one mistake this vocabulary exists to prevent.
  it("calls a mirror provisional however fresh the copy is", () => {
    expect(classifyRead({ readAt: ago(0.1), current: false }, NOW)).toBe("provisional");
  });

  it("treats a read that never happened as stale rather than live", () => {
    expect(classifyRead({}, NOW)).toBe("stale");
  });
});

/**
 * The split §31 A is explicit about: the rule is what the ACTION does, not how
 * old the read is. Adoption binds an identity irreversibly off a list that may
 * have moved; applying a plan joins a queue an operator can still inspect.
 */
describe("what a stale read blocks", () => {
  it("blocks an irreversible action once the read is stale", () => {
    expect(blocksIrreversibleAction({ readAt: ago(11), current: true }, NOW)).toBe(true);
  });

  it("blocks it on a mirror read at any age", () => {
    expect(blocksIrreversibleAction({ readAt: ago(1), current: false }, NOW)).toBe(true);
  });

  it("allows it while the read is merely ageing", () => {
    expect(blocksIrreversibleAction({ readAt: ago(4), current: true }, NOW)).toBe(false);
  });
});

describe("the strip", () => {
  it("always carries an age, never a word alone", () => {
    render(<ReadFreshness state={{ readAt: ago(4), current: true }} subject="The account list" />);
    // "recently" is not something an operator can act on: the number is the
    // whole content of the label.
    expect(screen.getByText(/4 min ago/)).toBeInTheDocument();
  });

  it("says a provisional read is the last state seen, with its age", () => {
    render(<ReadFreshness state={{ readAt: ago(14), current: false }} />);
    expect(screen.getByText(/last state seen/i)).toBeInTheDocument();
    expect(screen.getByText(/14 min ago/)).toBeInTheDocument();
  });

  // Truncation is orthogonal — a complete statement about what was seen and a
  // silence about the rest — so it rides alongside the age rather than
  // replacing it.
  it("reports truncation beside the age rather than instead of it", () => {
    render(<ReadFreshness state={{ readAt: ago(2), current: true, truncated: true }} />);
    expect(screen.getByText(/2 min ago/)).toBeInTheDocument();
    expect(screen.getByText(/not the whole list/i)).toBeInTheDocument();
  });

  it("offers a re-read only once the read is too old to act on", () => {
    const { rerender } = render(
      <ReadFreshness state={{ readAt: ago(2), current: true }} onRefresh={() => {}} />,
    );
    expect(screen.queryByRole("button", { name: /read again/i })).toBeNull();

    rerender(<ReadFreshness state={{ readAt: ago(20), current: true }} onRefresh={() => {}} />);
    expect(screen.getByRole("button", { name: /read again/i })).toBeInTheDocument();
  });
});
