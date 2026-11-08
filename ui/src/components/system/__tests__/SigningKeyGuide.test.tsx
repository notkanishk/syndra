// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { NOT_SEEN_AFTER_MS, SigningKeyGuide, replaceResult, type GuideStatus } from "@/components/system/SigningKeyGuide";
import { statesNothingChanged } from "@/lib/outcome";

// W4, W9, W11: each outcome of a replacement in its own words.
describe("replaceResult", () => {
  it("never says nothing changed when a key was, or may have been, replaced", () => {
    for (const outcome of ["unverified", "partial", "unknown"] as const) {
      const r = replaceResult({ outcome, replaced: [], reason: "Zitadel didn't answer." });
      expect(statesNothingChanged(r.outcome.kind)).toBe(false);
    }
    expect(replaceResult({ outcome: "partial", reason: "Zitadel replaced the claims action's key, but not the event listener's key. Zitadel refused." }).word).toBe("Partly replaced");
    expect(replaceResult({ outcome: "unknown" }).word).toBe("Not known");
  });

  it("says no test sign-in ran when none is set up, not that it failed", () => {
    const r = replaceResult({ outcome: "unverified", probe: { pass: false, class: "not_set_up" } });
    expect(r.outcome.message).toMatch(/No test sign-in could run, because SYNDRA_EXTERNAL_URL is not set/);
    expect(r.outcome.message).not.toMatch(/didn't get its claims/);
  });

  it("holds the old key for 7 days, not for ever", () => {
    const r = replaceResult({ outcome: "unverified", probe: { pass: false, class: "claims" } });
    expect(r.outcome.message).toMatch(/for 7 days/);
    expect(r.outcome.message).not.toMatch(/won't replace it again on its own/);
  });
});

/**
 * The by-hand guide (route-zitadel §2): it watches for the new key and moves
 * on by itself, checks it with a test sign-in, offers the long way round only
 * once the key is late, picks up where it was left, and changes nothing itself
 * except through "Let Syndra do it now".
 */
const api = vi.hoisted(() => ({
  watch: {} as Record<string, unknown>,
  verify: { probe: { pass: true }, next_due_at: "2027-02-01T03:00:00Z" } as unknown,
  rotate: { outcome: "verified", replaced: ["syndra-claim-injector"], next_due_at: "2027-02-01T03:00:00Z" } as unknown,
  calls: [] as Array<[string, unknown]>,
}));
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string, init?: { method?: string; body?: unknown }) => {
    api.calls.push([path, init?.body]);
    if (path === "/zitadel/action-key/verify") return api.verify;
    if (path === "/zitadel/action-key/rotate") return api.rotate;
    return api.watch;
  },
}));

const BEFORE = "2026-08-01T03:00:00Z";
const AFTER = "2026-11-05T10:12:00Z";
const installed: GuideStatus = {
  key_installed: true,
  last_rotated_at: BEFORE,
  rotate_command: "make zitadel-actions-rotate-key",
};

function open(status: GuideStatus = installed) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const onClose = vi.fn();
  const view = render(
    <QueryClientProvider client={client}>
      <SigningKeyGuide open status={status} onClose={onClose} />
    </QueryClientProvider>,
  );
  return { ...view, onClose };
}

const posts = (path: string) => api.calls.filter(([p]) => p === path);

beforeEach(() => {
  window.sessionStorage.clear();
  api.calls = [];
  api.watch = { ...installed };
  api.verify = { probe: { pass: true }, next_due_at: "2027-02-01T03:00:00Z" };
});
afterEach(() => vi.useRealTimers());

describe("SigningKeyGuide", () => {
  it("opens on step 1 with one command and Cancel focused, not the command's Copy", async () => {
    open();
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Step 1 of 2")).toBeInTheDocument();
    expect(within(dialog).getByText("Run this on the Syndra server.")).toBeInTheDocument();
    // The header is the shared one: chip, title (the dialog's name), lede.
    expect(dialog).toHaveAccessibleName("Make a new key");
    expect(within(dialog).getByText("make zitadel-actions-rotate-key")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
  });

  it("moves on by itself when Syndra sees the new key, checks it, and becomes the result", async () => {
    api.watch = { ...installed, last_rotated_at: AFTER };
    open();
    expect(await screen.findByText(/^Replaced\. A test sign-in got its claims\. Next due 1 Feb 2027\.$/)).toBeInTheDocument();
    expect(screen.getByText("Step 2 of 2")).toBeInTheDocument();
    expect(posts("/zitadel/action-key/verify")).toEqual([["/zitadel/action-key/verify", {}]]);
    // Watching and checking never replace anything.
    expect(posts("/zitadel/action-key/rotate")).toHaveLength(0);
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("says honestly when the test sign-in with the new key failed, and what to check", async () => {
    api.watch = { ...installed, last_rotated_at: AFTER };
    api.verify = { probe: { pass: false, class: "claims", detail: "append_claims was empty" } };
    open();
    expect(await screen.findByText("Not confirmed")).toBeInTheDocument();
    expect(screen.getByText(/didn't get its claims: append_claims was empty/)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing was changed/)).not.toBeInTheDocument();
  });

  it("stays on step 1 while the key is unchanged, and offers the long way round only once it is late", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    open();
    await screen.findByText(/Watching for the new key/);
    expect(screen.queryByRole("button", { name: /Not seen yet/ })).not.toBeInTheDocument();

    act(() => vi.advanceTimersByTime(NOT_SEEN_AFTER_MS - 1_000));
    expect(screen.queryByRole("button", { name: /Not seen yet/ })).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(1_000));

    const fold = await screen.findByRole("button", { name: /Not seen yet/ });
    expect(fold).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(fold);
    expect(screen.getByText("ZITADEL_EVENT_SIGNING_KEY")).toBeInTheDocument();
    // S10: the condition is one the reader can see on the page.
    expect(screen.getByText(/Only if the keyring line on this page says Syndra isn.t allowed to read the key from Zitadel/)).toBeInTheDocument();
    expect(screen.getByText("Step 1 of 2")).toBeInTheDocument();
    expect(posts("/zitadel/action-key/verify")).toHaveLength(0);
  });

  it("picks up where it was left: a key that changed while it was closed is checked on reopening", async () => {
    const first = open();
    await screen.findByText("Step 1 of 2");
    first.unmount();

    // Closed; the key changed meanwhile, so the page now reports the new one.
    api.watch = { ...installed, last_rotated_at: AFTER };
    open({ ...installed, last_rotated_at: AFTER });
    expect(await screen.findByText(/^Replaced\. A test sign-in got its claims/)).toBeInTheDocument();
  });

  it("replaces the key itself only through 'Let Syndra do it now'", async () => {
    open();
    fireEvent.click(await screen.findByRole("button", { name: "Let Syndra do it now" }));
    expect(await screen.findByText(/^Replaced\. A test sign-in got its claims\. Next due 1 Feb 2027\.$/)).toBeInTheDocument();
    expect(posts("/zitadel/action-key/rotate")).toEqual([["/zitadel/action-key/rotate", { force: true }]]);
    expect(posts("/zitadel/action-key/verify")).toHaveLength(0);
  });

  it("sets a key up the same way: register, watch for the key, check it", async () => {
    const setup: GuideStatus = { key_installed: false, register_command: "cd /srv/example && make zitadel-actions-register" };
    api.watch = { key_installed: false };
    const view = open(setup);
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Register Syndra with Zitadel")).toBeInTheDocument();
    expect(within(dialog).getByText("cd /srv/example && make zitadel-actions-register")).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Let Syndra do it now" })).not.toBeInTheDocument();
    view.unmount();

    api.watch = { key_installed: true, last_rotated_at: AFTER };
    open(setup);
    expect(await screen.findByText(/^Set up\. A test sign-in got its claims/)).toBeInTheDocument();
    expect(posts("/zitadel/action-key/rotate")).toHaveLength(0);
  });
});

// R8: no outcome of a replacement ends on the default "Nothing for you to do."
describe("replaceResult next steps", () => {
  it("partial: press Replace now again, or Do it by hand", () => {
    expect(replaceResult({ outcome: "partial" }).outcome.next).toMatch(/Replace now again to finish, or use Do it by hand/);
  });
  it("unverified: points at Claims delivery, or at SYNDRA_EXTERNAL_URL when no test sign-in could run", () => {
    expect(replaceResult({ outcome: "unverified", probe: { pass: false, class: "claims" } }).outcome.next).toMatch(/Claims delivery section on this page/);
    expect(replaceResult({ outcome: "unverified", probe: { pass: false, class: "not_set_up" } }).outcome.next).toMatch(/Set SYNDRA_EXTERNAL_URL on the Syndra server and restart it/);
  });
});
