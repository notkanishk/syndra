// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ChangeTrail, TRAIL_REREAD_MS, TRAIL_WATCH_MS, changeTrailFor } from "@/components/people/ChangeTrail";

/**
 * A-24 §5: the trail reads the person again, through the page's own query,
 * while its last step is open, and only then.
 */

const person = vi.hoisted(() => ({
  refetch: vi.fn(),
  standing: "sent" as string,
  readAt: "2026-11-09T09:00:00Z",
  current: true,
  updatedAt: 0,
}));

vi.mock("@/lib/queries/useUsers", () => ({
  useUserAccess: () => ({
    refetch: person.refetch,
    dataUpdatedAt: person.updatedAt,
    data: {
      projects: [{ project_id: "p1", standings: { operator: person.standing } }],
      observation: { read_at: person.readAt, current: person.current, truncated: false },
    },
  }),
}));

let at = 0;
const trail = () => <ChangeTrail userId="u1" projectId="p1" roleKey="operator" act="give" sent at={at} />;
const last = () => screen.getAllByRole("listitem")[2];

beforeEach(() => {
  vi.useFakeTimers();
  at = Date.now();
  person.refetch.mockReset();
  person.standing = "sent";
  person.readAt = "2026-11-09T09:00:00Z";
  person.current = true;
  person.updatedAt = at + 1;
});
afterEach(() => vi.useRealTimers());

describe("the trail reads the person again while its last step is open", () => {
  it("every fifteen seconds, and stops once the last step settles", () => {
    const view = render(trail());
    act(() => void vi.advanceTimersByTime(TRAIL_REREAD_MS));
    expect(person.refetch).toHaveBeenCalledTimes(1);

    // A newer Zitadel read now shows it held: the step settles, and the reading stops.
    person.standing = "held";
    person.readAt = "2026-11-09T09:43:00Z";
    view.rerender(trail());
    expect(last()).toHaveTextContent("In Zitadel.");
    act(() => void vi.advanceTimersByTime(TRAIL_REREAD_MS * 4));
    expect(person.refetch).toHaveBeenCalledTimes(1);
  });

  it("for no longer than the watch, however long the step stays open (one clock: the light's)", () => {
    render(trail());
    act(() => void vi.advanceTimersByTime(TRAIL_WATCH_MS - 1));
    expect(person.refetch).toHaveBeenCalledTimes(Math.floor(TRAIL_WATCH_MS / TRAIL_REREAD_MS) - 1);
    act(() => void vi.advanceTimersByTime(1));
    const calls = person.refetch.mock.calls.length;
    expect(calls).toBeLessThanOrEqual(Math.floor(TRAIL_WATCH_MS / TRAIL_REREAD_MS));
    act(() => void vi.advanceTimersByTime(TRAIL_REREAD_MS * 10));
    expect(person.refetch).toHaveBeenCalledTimes(calls);
  });

  it("not at all once the sheet closes", () => {
    const view = render(trail());
    view.unmount();
    act(() => void vi.advanceTimersByTime(TRAIL_REREAD_MS * 10));
    expect(person.refetch).not.toHaveBeenCalled();
  });
});

describe("a read that did not complete is no read of Zitadel", () => {
  it("leaves the last step open, however new its time", () => {
    const view = render(trail());
    person.standing = "held";
    person.readAt = "2026-11-09T09:43:00Z";
    person.current = false;
    view.rerender(trail());
    expect(last()).toHaveTextContent("Not read back from Zitadel yet.");
    person.current = true;
    view.rerender(trail());
    expect(last()).toHaveTextContent("In Zitadel.");
  });
});

describe("the one rule for whether an answer has a trail", () => {
  it("has one for a row of its own that went or waits, and none for no row, a failure or a refusal", () => {
    expect(changeTrailFor("applied", undefined)).toMatchObject({ sent: true });
    expect(changeTrailFor("applied", "role_waits")).toMatchObject({ sent: false });
    expect(changeTrailFor("pending", "halted:retrying")).toMatchObject({ sent: false });
    expect(changeTrailFor(undefined, undefined)).toBeUndefined();
    expect(changeTrailFor("", undefined)).toBeUndefined();
    expect(changeTrailFor("failed", undefined)).toBeUndefined();
    expect(changeTrailFor("pending", "refused:not_found")).toBeUndefined();
  });
});
