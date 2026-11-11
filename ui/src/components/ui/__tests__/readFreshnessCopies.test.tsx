// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { cooldownLine, copyLine, unreachableLine, MEMBER, type Reader } from "@/components/results/copy";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { formatClock, formatRelative } from "@/lib/format";
import { COOLDOWN_REFUSED, cooldownNext } from "@/lib/outcome";

/**
 * What a card says when the add-on answered from a copy (feature freshness):
 * one sentence per reason, and the control that is the next step. Real clock
 * for the age (the strip reads it), fake timers only where a count is the point.
 */

const NAME = "TrueNAS";
const at = new Date(Date.now() - 3 * 60_000).toISOString();
const OPERATOR: Reader = { audience: "operator", person: "Rey" };
const copy = (stale: ReadState["stale"], extra: Partial<ReadState> = {}): ReadState => ({ readAt: at, current: false, stale, ...extra });

afterEach(() => vi.useRealTimers());

describe("a copy the add-on kept", () => {
  it("says how old it is and offers Read again", () => {
    const press = vi.fn();
    render(<ReadFreshness state={copy({ reason: "cached" })} subject={NAME} onRefresh={press} />);
    expect(screen.getByText(copyLine(NAME, formatRelative(at)))).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(press).toHaveBeenCalledTimes(1);
  });

  it("offers nothing where no page can read again", () => {
    render(<ReadFreshness state={copy({ reason: "cached" })} subject={NAME} />);
    expect(screen.queryByRole("button")).toBeNull();
  });
});

describe("a Read again inside the cooldown", () => {
  it("says it was read moments ago, counts down, keeps the button off with its reason, and gives it back at 0", () => {
    vi.useFakeTimers();
    const press = vi.fn();
    render(<ReadFreshness state={copy({ reason: "cooldown", retry_after: 3 })} subject={NAME} onRefresh={press} />);
    const line = screen.getByText(cooldownLine(NAME, 3));
    const button = screen.getByRole("button", { name: "Read again" });
    expect(button).toBeDisabled();
    expect(button.getAttribute("aria-describedby")).toBe(line.id);
    fireEvent.click(button);
    expect(press).not.toHaveBeenCalled();

    act(() => void vi.advanceTimersByTime(1000));
    expect(screen.getByText(cooldownLine(NAME, 2))).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(2000));
    expect(screen.queryByText(/moments ago/)).toBeNull();
    expect(screen.getByText(copyLine(NAME, formatRelative(at)))).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Read again" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(press).toHaveBeenCalledTimes(1);
  });

  it("with nothing to show, still the cooldown line and never an error, then not-read-yet with the button", () => {
    vi.useFakeTimers();
    render(<ReadFreshness state={{ stale: { reason: "cooldown", retry_after: 1 } }} subject={NAME} onRefresh={() => {}} />);
    expect(screen.getByText(cooldownLine(NAME, 1))).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
    act(() => void vi.advanceTimersByTime(1000));
    expect(screen.getByText(`${NAME} has not been read yet`)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Read again" })).toBeEnabled();
  });
});

describe("a system that cannot be reached", () => {
  it("says what it last said and when, ends on the step for a member, and offers no button", () => {
    render(<ReadFreshness state={copy({ reason: "unreachable" })} subject={NAME} reader={MEMBER} onRefresh={() => {}} />);
    expect(screen.getByText(unreachableLine(NAME, formatClock(at), MEMBER))).toBeInTheDocument();
    expect(screen.getByText(/ask makerspace staff\.$/)).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("sends an operator to the add-on's health instead", () => {
    render(<ReadFreshness state={copy({ reason: "unreachable" })} subject={NAME} reader={OPERATOR} />);
    expect(screen.getByText(/check the add-on's health on its system page\.$/)).toBeInTheDocument();
    expect(screen.queryByText(/makerspace staff/)).toBeNull();
  });
});

describe("a current read", () => {
  it("is not offered a Read again, whatever the page can do", () => {
    render(<ReadFreshness state={{ readAt: new Date().toISOString(), current: true }} subject={NAME} onRefresh={() => {}} />);
    expect(screen.queryByRole("button")).toBeNull();
  });
});

describe("a refusal that counts down", () => {
  it("says nothing was changed and counts to the step", () => {
    vi.useFakeTimers();
    const outcome = { kind: "refused" as const, message: COOLDOWN_REFUSED, retryAfter: 2, next: cooldownNext(2) };
    render(<ActionOutcome outcome={outcome} word="Not removed" />);
    expect(screen.getByText("That system was read moments ago, so nothing was changed.")).toBeInTheDocument();
    expect(screen.getByText(cooldownNext(2))).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(1000));
    expect(screen.getByText(cooldownNext(1))).toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(1000));
    expect(screen.getByText(cooldownNext(0))).toBeInTheDocument();
    expect(screen.queryByText(/try again in/)).toBeNull();
  });
});
