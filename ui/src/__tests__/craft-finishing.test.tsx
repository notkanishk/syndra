// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it, vi } from "vitest";

import { ChangeTrail, trailSteps } from "@/components/people/ChangeTrail";
import { EmptyState, PLATES_SHOWN, Plate } from "@/components/states";
import { CheckTrail } from "@/components/ui/CheckTrail";
import { usersQueryKeys } from "@/lib/queries/useUsers";
import type { Standing } from "@/lib/standing";
import { LONG_WAIT_MS, useLongWait } from "@/lib/useLongWait";

/**
 * A-24 wave 2, finishing and trust (§4.1, §5, §6, §7). The trail is held to
 * the truth: a step settles only on what was observed, and a step nothing can
 * observe is not drawn.
 */

const css = readFileSync(resolve(__dirname, "../app/globals.css"), "utf8");
const said = (steps: ReturnType<typeof trailSteps>) => steps.map((s) => `${s.tone === "healthy" ? "●" : "○"} ${s.sentence}`);
/** A Zitadel read made after the act. */
const Z = { readAt: "2026-11-09T09:43:00Z" };

describe("the change's trail settles only on what was observed (§5)", () => {
  it("never settles the last step without a Zitadel read made after the act, whatever the standing says", () => {
    expect(said(trailSteps("give", true, "held"))[2]).toBe("○ Not read back from Zitadel yet.");
    expect(said(trailSteps("revoke", false, "not_held")).slice(1)).toEqual(["○ Not sent to Zitadel yet.", "○ Not removed from Zitadel yet."]);
  });

  it("a give: sent from the act's own answer, in Zitadel only from a read that says held", () => {
    expect(said(trailSteps("give", true, "held", Z))).toEqual(["● Recorded in Syndra.", "● Sent to Zitadel.", "● In Zitadel."]);
    expect(said(trailSteps("give", true, undefined))).toEqual([
      "● Recorded in Syndra.",
      "● Sent to Zitadel.",
      "○ Not read back from Zitadel yet.",
    ]);
    expect(said(trailSteps("give", false, "waiting"))).toEqual([
      "● Recorded in Syndra.",
      "○ Not sent to Zitadel yet.",
      "○ Not in Zitadel yet.",
    ]);
    // Sent later, seen by a read: the step fills though the act's own answer said it waited.
    expect(said(trailSteps("give", false, "sent"))[1]).toBe("● Sent to Zitadel.");
    // Only "held" is in Zitadel: sent, waiting, being removed, not held or unread never settle it.
    for (const standing of ["sent", "waiting", "removing", "not_held", undefined, null] as Standing[]) {
      expect(said(trailSteps("give", true, standing, Z))[2], String(standing)).toBe("○ Not read back from Zitadel yet.");
    }
  });

  it("a revoke: no longer in Zitadel only from a read that says not held; nothing else settles it", () => {
    expect(said(trailSteps("revoke", true, "not_held", Z))[2]).toBe("● No longer in Zitadel.");
    for (const standing of ["held", "removing", "sent", "waiting", undefined, null] as Standing[]) {
      expect(said(trailSteps("revoke", true, standing, Z))[2], String(standing)).toBe("○ Not read back from Zitadel yet.");
    }
    // A give's "sent" says nothing about a revoke reaching Zitadel.
    expect(said(trailSteps("revoke", false, "sent")).slice(1)).toEqual(["○ Not sent to Zitadel yet.", "○ Not removed from Zitadel yet."]);
  });

  it("a settled last step says when it was read back; an open step is a ring", () => {
    const [, , last] = trailSteps("give", true, "held", Z);
    expect(last.faint?.[0]).toMatch(/^Read back at \d\d:\d\d\.$/);
    expect(trailSteps("give", false, undefined).filter((s) => s.ring)).toHaveLength(2);
  });
});

describe("CheckTrail reads as a route when it is labelled", () => {
  it("names the list and reads each step as Step n, with no pass or fail word", () => {
    render(<CheckTrail label="On its way to Zitadel" checks={trailSteps("give", true, undefined)} />);
    const list = screen.getByRole("list", { name: "On its way to Zitadel" });
    expect(list).toHaveTextContent("Step 1: Recorded in Syndra.Step 2: Sent to Zitadel.Step 3: Not read back from Zitadel yet.");
    expect(list).not.toHaveTextContent(/passes|fails|nothing to check/);
  });

  it("stays the token's checks without one", () => {
    render(<CheckTrail checks={[{ tone: "healthy", sentence: "Signed." }]} />);
    expect(screen.getByRole("list", { name: "Checks" })).toHaveTextContent("Check 1, passes: Signed.");
  });
});

describe("ChangeTrail trusts a fetch after the act, and a Zitadel read newer than the one it began with", () => {
  const access = (standing: string, readAt: string) => ({
    projects: [{ project_id: "p1", standings: { operator: standing } }],
    observation: { read_at: readAt, current: true, truncated: false },
  });
  const mount = (client: QueryClient, at: number) =>
    render(
      <QueryClientProvider client={client}>
        <ChangeTrail userId="u1" projectId="p1" roleKey="operator" act="revoke" sent at={at} />
      </QueryClientProvider>,
    );
  const last = () => screen.getAllByRole("listitem")[2];

  it("a fresh fetch carrying an old Zitadel read settles nothing, a newer Zitadel read does", async () => {
    const client = new QueryClient();
    const at = Date.now();
    client.setQueryData(usersQueryKeys.access("u1"), access("held", "2026-11-09T09:00:00Z"), { updatedAt: at - 1 });
    mount(client, at);
    expect(last()).toHaveTextContent("Not read back from Zitadel yet.");

    // Fetched after the act, but its Zitadel read is the old one: a not_held from it says nothing about this revoke.
    act(() => void client.setQueryData(usersQueryKeys.access("u1"), access("not_held", "2026-11-09T09:00:00Z"), { updatedAt: at + 1 }));
    await waitFor(() => expect(client.getQueryState(usersQueryKeys.access("u1"))?.dataUpdatedAt).toBe(at + 1));
    expect(last()).toHaveTextContent("Not read back from Zitadel yet.");

    act(() => void client.setQueryData(usersQueryKeys.access("u1"), access("not_held", "2026-11-09T09:43:00Z"), { updatedAt: at + 2 }));
    await waitFor(() => expect(last()).toHaveTextContent(/^Step 3: No longer in Zitadel\.Read back at \d\d:\d\d\.$/));
  });

  it("with nothing read before, no Zitadel read can settle the last step", async () => {
    const client = new QueryClient();
    const at = Date.now();
    mount(client, at);
    act(() => void client.setQueryData(usersQueryKeys.access("u1"), access("not_held", "2026-11-09T09:43:00Z"), { updatedAt: at + 1 }));
    await waitFor(() => expect(screen.getAllByRole("listitem")[2]).toBeInTheDocument());
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(last()).toHaveTextContent("Not read back from Zitadel yet.");
  });
});

describe("a long wait says so once (§6)", () => {
  it("is false until the wait passes ten seconds, and false again the moment it settles", () => {
    vi.useFakeTimers();
    const renders: Array<[boolean, boolean]> = [];
    const { result, rerender } = renderHook(
      ({ pending }) => {
        const long = useLongWait(pending);
        renders.push([pending, long]);
        return long;
      },
      { initialProps: { pending: true } },
    );
    act(() => void vi.advanceTimersByTime(LONG_WAIT_MS - 1));
    expect(result.current).toBe(false);
    act(() => void vi.advanceTimersByTime(1));
    expect(result.current).toBe(true);
    rerender({ pending: false });
    expect(result.current).toBe(false);
    // Not even one render says it is still waiting once it has settled.
    expect(renders.filter(([pending, long]) => !pending && long)).toEqual([]);
    vi.useRealTimers();
  });
});

describe("the list closes, and plates wait for the owner (§4.1, §7, Q9)", () => {
  it("closing: the sentence takes the heading size and a lime hairline draws once over one flash", () => {
    render(<EmptyState resolved closes title="Nothing here needs you." />);
    expect(screen.getByText("Nothing here needs you.")).toHaveClass("type-section-title");
    expect(document.querySelector("[data-close-line]")).toHaveClass("close-line");
    const rule = css.slice(css.indexOf("@utility close-line"), css.indexOf("@utility plate"));
    expect(rule).toContain("animation: hairline-draw var(--duration-flash) var(--ease-flash) both;");
    expect(rule).toContain("var(--healthy)");
  });

  it("does not draw a plate until the owner has seen them at true size", () => {
    expect(PLATES_SHOWN).toBe(false);
    render(<EmptyState resolved plate="shut" title="Everyone inside is on the list." />);
    expect(document.querySelector("[data-plate]")).toBeNull();
  });

  it("draws the shut door and the open door from the tokens, hidden from assistive technology", () => {
    render(
      <>
        <Plate kind="shut" />
        <Plate kind="open" />
      </>,
    );
    for (const kind of ["shut", "open"]) {
      const plate = document.querySelector(`[data-plate="${kind}"]`)!;
      expect(plate).toHaveAttribute("aria-hidden");
      expect(plate.children).toHaveLength(3);
    }
    const rule = css.slice(css.indexOf("@utility plate"), css.indexOf("/* The shared bloom"));
    expect(rule).not.toMatch(/#[0-9a-f]{3,6}\b/i);
    expect(rule).toContain('&[data-plate="open"]');
  });
});
