// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import IdentityProviderPage from "@/app/zitadel/page";

const rotation = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const health = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const projects = vi.hoisted(() => ({ value: [] as unknown[] | undefined }));
const refetch = vi.hoisted(() => vi.fn());

// The page reads rotation status and replaces the key through the raw request
// helper, and health through its own hook, so both are stubbed at their seams.
const rotate = vi.hoisted(() => ({ next: null as Promise<unknown> | (() => Promise<unknown>) | null, calls: [] as unknown[][] }));
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string, init?: { method?: string }) => {
    if (init?.method === "POST") {
      rotate.calls.push([path, init]);
      return typeof rotate.next === "function" ? rotate.next() : rotate.next;
    }
    return rotation.value;
  },
}));
const posts = () => rotate.calls;

vi.mock("@/lib/queries/useZitadel", () => ({
  useZitadelHealth: () => ({ data: health.value, isLoading: false, error: null, refetch, isFetching: false }),
}));

const keyring = vi.hoisted(() => ({ value: null as Record<string, unknown> | null }));
vi.mock("@/lib/queries/useClaimsDelivery", () => ({
  useClaimsDelivery: () => ({ data: keyring.value ? { keyring: keyring.value } : undefined, isLoading: !keyring.value, error: null }),
}));
// Claims delivery has its own file; here it is only a seat.
vi.mock("@/components/system/ClaimsDelivery", () => ({
  ClaimsDelivery: () => <section id="claims-delivery" aria-label="Claims delivery" />,
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: projects.value, isLoading: projects.value === undefined, error: null }),
}));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <IdentityProviderPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  health.value = { status: "ok", mode: "live", latency_ms: 21, domain: "auth.example.org" };
  projects.value = [];
  keyring.value = null;
  rotate.next = null;
  rotate.calls = [];
  rotation.value = {
    key_installed: true,
    status: "ok",
    age_days: 12,
    threshold_days: 90,
    last_rotated_at: "2026-07-01T00:00:00Z",
    last_rotated_source: "env",
    due_at: "2026-09-29T00:00:00Z",
    rotate_command: "make zitadel-actions-rotate-key",
    auto: { on: true, lead_days: 5, window: "03:00–05:00", next_attempt_at: "2026-09-25T03:00:00Z" },
    rotating: false,
  };
});

/**
 * (1) One page, one fact about reachability — not two cards computing it
 * differently.
 *
 * The "Connection" tile used to key off `mode` alone, which reads "live" for
 * both a healthy connection and a live one whose last call just failed — so a
 * real outage showed "Connected" here directly under "Unreachable" in the
 * banner above it. These assert the tile always agrees with the banner's
 * verdict, for every reachability state the backend can report.
 */
describe("Identity provider · one verdict about reachability", () => {
  it("calls it 'Not connected' everywhere when Zitadel was never set up", async () => {
    health.value = {
      status: "disabled",
      mode: "local-policy-only",
      error: "Management client not initialized — check ZITADEL_DOMAIN, ZITADEL_MACHINE_KEY_PATH, and backend startup logs",
    };
    renderPage();
    expect(await screen.findAllByText(/Not connected/)).not.toHaveLength(0);
    expect(screen.queryByText(/^Unreachable/)).not.toBeInTheDocument();
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
  });

  // A-20 P5: one word per state. The seat and the Notice never disagree, and "Unreachable" is gone.
  it("says 'Not answering' in the seat, with the pips, and never 'Unreachable' or 'Connected'", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    renderPage();
    expect(await screen.findByText("Not answering")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /Last 3 checks: missed, no check yet, no check yet/ })).toBeInTheDocument();
    expect(screen.queryByText(/Unreachable/)).not.toBeInTheDocument();
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
    expect(screen.queryByText(/^Not connected/)).not.toBeInTheDocument();
    // One or two missed checks raise no Notice.
    expect(screen.queryByText("Not answering.")).not.toBeInTheDocument();
  });

  it("is Offline, with a Notice of the same word, once three checks in a row have gone unanswered", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    const { rerender } = renderPage();
    for (const n of [1, 2]) {
      health.value = { status: "error", mode: "live", error: `miss ${n}` };
      rerender(
        <QueryClientProvider client={new QueryClient()}>
          <IdentityProviderPage />
        </QueryClientProvider>,
      );
    }
    expect(await screen.findByText("Offline")).toBeInTheDocument();
    expect(screen.getByText("Offline.")).toBeInTheDocument();
    expect(screen.queryByText("Not answering")).not.toBeInTheDocument();
  });

  it("has a Check again that reads again", async () => {
    health.value = { status: "error", mode: "live" };
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Check again" }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});

/**
 * Health is one line (route-zitadel, 7c), with nothing folded behind it: a fold
 * that repeated the line above it was deleted (A-17 §2.22). The project count
 * sits in the header and names its source: Zitadel's own number once it
 * answers, Syndra's remembered one, marked last known, when it can't.
 */
describe("Identity provider · health line", () => {
  // "0 projects, last known" used to be said while the projects were still loading.
  it("never says a count it has not read: while projects load the header says no count", async () => {
    health.value = { status: "error", mode: "live" };
    projects.value = undefined;
    renderPage();
    expect(await screen.findByText("Not answering")).toBeInTheDocument();
    expect(screen.queryByText(/\d+ projects?/)).not.toBeInTheDocument();
  });

  it("is one line with the reply time and no fold behind it", async () => {
    health.value = { status: "ok", mode: "live", latency_ms: 42, projects_total: 6 };
    renderPage();
    expect(await screen.findByText("Answering")).toBeInTheDocument();
    expect(screen.getByText(/^in 42 ms, and sees \d+ projects?$/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /checks/ })).not.toBeInTheDocument();
  });

  it("counts projects as Zitadel reported them once the connection is healthy", async () => {
    health.value = { status: "ok", mode: "live", projects_total: 6 };
    projects.value = [{ id: "cached-only" }];
    renderPage();
    expect(await screen.findByText("6 projects")).toBeInTheDocument();
  });

  it("falls back to Syndra's remembered count, marked last known, and has nothing to fold", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    projects.value = [{ id: "p1" }, { id: "p2" }, { id: "p3" }];
    renderPage();
    expect(await screen.findByText("3 projects, last known")).toBeInTheDocument();
    expect(screen.getByText("Not answering")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /checks/ })).not.toBeInTheDocument();
  });
});

/**
 * The signing key is one compact row: a word and a sentence about when it was
 * last replaced and when Syndra replaces it next. Replace now opens a dialog;
 * Do it by hand opens the guide (components/system/SigningKeyGuide).
 */
describe("Identity provider · signing key row", () => {
  it("says when the next replacement is due, and that Syndra does it on its own", async () => {
    renderPage();
    expect(await screen.findByText("Within its limit")).toBeInTheDocument();
    expect(screen.getByText(/^Due on \d+ \w+( \d{4})?; Syndra replaces it on its own before then\.$/)).toBeInTheDocument();
    expect(screen.queryByText(/Replace it now/)).not.toBeInTheDocument();
  });

  it("says Syndra replaced it on its own, and when the next one is due", async () => {
    rotation.value = {
      ...rotation.value,
      last_rotated_source: "syndra",
      last_outcome: { trigger: "auto", outcome: "verified", started_at: "2026-11-03T03:10:00Z", finished_at: "2026-11-03T03:10:20Z" },
    };
    renderPage();
    expect(await screen.findByText(/^Replaced on its own on 3 Nov( 2026)?\. Next by \d+ \w+( \d{4})?\.$/)).toBeInTheDocument();
  });

  it("says a button replacement without 'on its own'", async () => {
    rotation.value = {
      ...rotation.value,
      last_rotated_source: "syndra",
      last_outcome: { trigger: "button", outcome: "verified", started_at: "2026-11-03T10:00:00Z", finished_at: "2026-11-03T10:00:20Z" },
    };
    renderPage();
    expect(await screen.findByText(/^Replaced on 3 Nov( 2026)?\. Next by /)).toBeInTheDocument();
  });

  it("past the limit with Syndra on: the Notice and the row say it is replaced in the next quiet hours", async () => {
    rotation.value = {
      ...rotation.value,
      status: "warn",
      age_days: 104,
      auto: { on: true, lead_days: 5, window: "03:00–05:00", next_attempt_at: "2026-11-06T03:00:00Z" },
    };
    renderPage();
    expect(await screen.findByText("Past its limit")).toBeInTheDocument();
    // The age is said once, by the Notice under the header; the card does not repeat it.
    expect(screen.getByText("The action signing key is 104 days old.")).toBeInTheDocument();
    expect(screen.getAllByText(/104 days old/)).toHaveLength(1);
    expect(screen.getByText(/Syndra replaces it on its own in the next quiet hours, and sign-ins carry on/)).toBeInTheDocument();
    expect(screen.getByText(/^Syndra replaces it on its own in the next quiet hours, on 6 Nov/)).toBeInTheDocument();
    expect(screen.queryByText(/walks through it/)).not.toBeInTheDocument();
  });

  it("with Syndra switched off, says so and points at Replace now", async () => {
    rotation.value = { ...rotation.value, status: "warn", age_days: 104, auto: { on: false, off_reason: "switched_off", lead_days: 5, window: "03:00–05:00" } };
    renderPage();
    expect(await screen.findByText(/Replacing it on its own is switched off. Replace now, below, does it/)).toBeInTheDocument();
    expect(screen.getByText("Replacing it on its own is switched off. Replace it now, or by hand below.")).toBeInTheDocument();
  });

  // W5: "on its own" only when the schedule would actually act.
  it("never promises a replacement on its own while it waits for a test sign-in", async () => {
    rotation.value = {
      ...rotation.value,
      status: "warn",
      age_days: 104,
      auto: { on: true, lead_days: 5, window: "03:00–05:00", next_attempt_at: "2026-11-06T03:00:00Z", waiting_for: "test_sign_in_setup" },
    };
    renderPage();
    expect(await screen.findByText(/none can run until SYNDRA_EXTERNAL_URL is set. Replace now, below, does it/)).toBeInTheDocument();
    expect(screen.getByText(/^Syndra replaces it on its own only after a test sign-in passes, and none can run until SYNDRA_EXTERNAL_URL is set\. Replace it now, or by hand below\.$/)).toBeInTheDocument();
    expect(screen.queryByText(/in the next quiet hours/)).not.toBeInTheDocument();
  });

  it("after a failure, says it tries again only when the schedule is on and will", async () => {
    const failed = { trigger: "button", outcome: "failed", started_at: "2026-11-05T03:00:00Z", finished_at: "2026-11-05T03:00:02Z", reason: "Zitadel didn't answer. Syndra read the key again: it didn't change." };
    rotation.value = { ...rotation.value, auto: { on: true, lead_days: 5, window: "03:00–05:00", next_attempt_at: "2026-11-06T03:00:00Z", waiting_for: "test_sign_in" }, last_outcome: failed };
    const { unmount } = renderPage();
    expect(await screen.findByText(/none has passed recently\. Replace it now, or by hand below\.$/)).toBeInTheDocument();
    expect(screen.queryByText(/tries again/)).not.toBeInTheDocument();
    unmount();

    rotation.value = { ...rotation.value, auto: { on: false, off_reason: "switched_off", lead_days: 5, window: "03:00–05:00" }, last_outcome: failed };
    const off = renderPage();
    expect(await screen.findByText(/switched off\. Replace it now, or by hand below\.$/)).toBeInTheDocument();
    expect(screen.queryByText(/tries again/)).not.toBeInTheDocument();
    off.unmount();

    rotation.value = { ...rotation.value, auto: { on: true, lead_days: 5, window: "03:00–05:00", next_attempt_at: "2026-11-06T03:00:00Z" }, last_outcome: failed };
    renderPage();
    expect(await screen.findByText(/Syndra tries again on its own on 6 Nov/)).toBeInTheDocument();
  });

  // W4: a partial or unknown outcome says so, with its plain reason; a stored raw status is never shown.
  it("names a partial replacement and an unknown one in their own words", async () => {
    rotation.value = {
      ...rotation.value,
      last_outcome: { trigger: "button", outcome: "partial", started_at: "2026-11-05T03:00:00Z", finished_at: "2026-11-05T03:00:02Z", replaced: ["syndra-claim-injector"], reason: "Zitadel replaced the claims action's key, but not the event listener's key. Zitadel refused." },
    };
    const { unmount } = renderPage();
    expect(await screen.findByText("Partly replaced")).toBeInTheDocument();
    expect(screen.getByText(/but not the event listener's key\. Zitadel refused\. Syndra uses the new key, and the old one still works for the other/)).toBeInTheDocument();
    unmount();

    rotation.value = { ...rotation.value, last_outcome: { trigger: "auto", outcome: "unknown", started_at: "2026-11-05T03:00:00Z", finished_at: "2026-11-05T03:00:02Z", reason: "Zitadel didn't answer: HTTP 503" } };
    renderPage();
    expect(await screen.findByText("Not known")).toBeInTheDocument();
    expect(screen.getByText(/can't tell whether it was replaced/)).toBeInTheDocument();
    expect(screen.queryByText(/HTTP 503/)).not.toBeInTheDocument();
  });

  it("says Replacing while a replacement is in flight, and offers no second one", async () => {
    rotation.value = { ...rotation.value, rotating: true, last_outcome: { trigger: "auto", outcome: "in_flight", started_at: "2026-11-05T03:00:00Z" } };
    renderPage();
    expect(await screen.findByText("Replacing")).toBeInTheDocument();
    expect(screen.getByText(/^Replacing… /)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Replace now" })).not.toBeInTheDocument();
  });

  it("says honestly when the test sign-in after a replacement failed, and what still works", async () => {
    rotation.value = {
      ...rotation.value,
      last_rotated_at: "2026-11-05T03:00:20Z",
      last_rotated_source: "syndra",
      last_outcome: { trigger: "auto", outcome: "unverified", started_at: "2026-11-05T03:00:00Z", finished_at: "2026-11-05T03:00:20Z", probe: { pass: false, class: "claims" } },
    };
    renderPage();
    expect(await screen.findByText("Not confirmed")).toBeInTheDocument();
    // W11: 7 days, not for ever; confirmed by a regular test sign-in; replaced again when next due.
    expect(screen.getByText(/didn't get its claims. Syndra accepts the old key and the new one for 7 days. It marks the new key confirmed once a regular test sign-in passes, and won't replace it again before it is next due/)).toBeInTheDocument();
  });

  // W9: with no test sign-in set up, none ran, so none "didn't get its claims".
  it("says no test sign-in could run when none is set up", async () => {
    rotation.value = {
      ...rotation.value,
      last_rotated_at: "2026-11-05T03:00:20Z",
      last_rotated_source: "syndra",
      last_outcome: { trigger: "button", outcome: "unverified", started_at: "2026-11-05T03:00:00Z", finished_at: "2026-11-05T03:00:20Z", probe: { pass: false, class: "not_set_up" } },
    };
    renderPage();
    expect(await screen.findByText(/No test sign-in could run, because SYNDRA_EXTERNAL_URL is not set/)).toBeInTheDocument();
    expect(screen.queryByText(/didn't get its claims/)).not.toBeInTheDocument();
  });

  it("says honestly when Zitadel didn't replace it, and that the old key still works", async () => {
    rotation.value = {
      ...rotation.value,
      auto: { on: true, lead_days: 5, window: "03:00–05:00", next_attempt_at: "2026-11-06T03:00:00Z" },
      last_outcome: { trigger: "auto", outcome: "failed", started_at: "2026-11-05T03:00:00Z", finished_at: "2026-11-05T03:00:02Z", reason: "Zitadel refused." },
    };
    renderPage();
    expect(await screen.findByText("Not replaced")).toBeInTheDocument();
    expect(screen.getByText(/Zitadel didn't replace it. Zitadel refused. The old key still works. Syndra tries again on its own on 6 Nov/)).toBeInTheDocument();
  });

  // "disabled" reads like "not set up yet" and means every inbound Action
  // request is being trusted unchecked. The copy has to say the second thing.
  it("treats a missing key as verification being off, with no Replace now", async () => {
    rotation.value = { key_installed: false, status: "disabled", threshold_days: 90 };
    renderPage();
    expect(await screen.findByText(/accepts every request that claims to come from Zitadel without checking/)).toBeInTheDocument();
    expect(screen.getByText("Not set up")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "How to set it up" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Replace now" })).not.toBeInTheDocument();
  });
});

describe("Identity provider · Replace now", () => {
  it("opens a dialog with one plain sentence, Cancel first and focused", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Replace now" }));
    const dialog = await screen.findByRole("dialog");
    // W9: both keys, named.
    expect(within(dialog).getByText(/Zitadel makes new keys for the claims action and the event listener, and Syndra starts using them at once/)).toBeInTheDocument();
    const buttons = within(dialog).getAllByRole("button").filter((b) => b.getAttribute("tabindex") !== "-1");
    expect(buttons.map((b) => b.textContent)).toEqual(["Cancel", "Replace the key"]);
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
  });

  it("Cancel closes it without asking the server for anything", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Replace now" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(posts()).toHaveLength(0);
  });

  it("says what Syndra will do while it runs, then the result", async () => {
    let answer!: (v: unknown) => void;
    rotate.next = new Promise((resolve) => (answer = resolve));
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Replace now" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Replace the key" }));
    expect(await screen.findByText("Will be replaced")).toBeInTheDocument();
    expect(posts()).toEqual([["/zitadel/action-key/rotate", { method: "POST", body: { force: true } }]]);

    answer({ outcome: "verified", replaced: ["syndra-claim-injector", "syndra-event-listener"], next_due_at: "2027-01-29T03:00:00Z", probe: { pass: true } });
    expect(await screen.findByText(/^Replaced\. A test sign-in got its claims\. Next due 29 Jan 2027\.$/)).toBeInTheDocument();
    expect(screen.queryByText("Will be replaced")).not.toBeInTheDocument();
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("says honestly when the test sign-in after it failed", async () => {
    rotate.next = Promise.resolve({ outcome: "unverified", replaced: ["syndra-claim-injector"], probe: { pass: false, class: "claims" } });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Replace now" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Replace the key" }));
    expect(await screen.findByText("Replaced, not confirmed")).toBeInTheDocument();
    expect(screen.getByText(/Replaced, but the test sign-in after it didn't get its claims/)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing was changed/)).not.toBeInTheDocument();
  });

  it("says honestly when Zitadel didn't replace it", async () => {
    rotate.next = Promise.resolve({ outcome: "failed", replaced: [], reason: "Zitadel refused." });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Replace now" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Replace the key" }));
    expect(await screen.findByText("Not replaced")).toBeInTheDocument();
    expect(screen.getByText(/Zitadel didn't replace the key. Zitadel refused. The old key still works/)).toBeInTheDocument();
  });

  // W4: partial and unknown are their own results, with the plain reason, never "Nothing was changed".
  it("reports a partial and an unknown replacement as what they are", async () => {
    rotate.next = Promise.resolve({
      outcome: "partial",
      replaced: ["syndra-claim-injector"],
      reason: "Zitadel replaced the claims action's key, but not the event listener's key. Zitadel refused.",
    });
    const { unmount } = renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Replace now" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Replace the key" }));
    expect(await screen.findByText("Partly replaced")).toBeInTheDocument();
    expect(screen.getByText(/the old one still works for the other/)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing was changed/)).not.toBeInTheDocument();
    unmount();

    rotate.next = Promise.resolve({
      outcome: "unknown",
      replaced: [],
      reason: "Zitadel didn't answer, and Syndra couldn't read the key back, so it can't tell whether Zitadel replaced it.",
    });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Replace now" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Replace the key" }));
    expect(await screen.findByText("Not known")).toBeInTheDocument();
    expect(screen.getByText(/can't tell whether Zitadel replaced it/)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing was changed/)).not.toBeInTheDocument();
  });

  it("keeps the act after a refusal, so it can be tried again", async () => {
    rotate.next = () => Promise.reject(new Error("Another replacement is running."));
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Replace now" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Replace the key" }));
    expect(await screen.findByText("Not replaced")).toBeInTheDocument();
    expect(within(screen.getByRole("dialog")).getByRole("button", { name: "Replace the key" })).toBeInTheDocument();
  });
});

describe("Identity provider · doing it by hand", () => {
  it("puts 'Do it by hand' beside Replace now, and keeps no command on the page", async () => {
    renderPage();
    expect(await screen.findByRole("button", { name: "Replace now" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Do it by hand" })).toBeInTheDocument();
    expect(screen.queryByText("make zitadel-actions-rotate-key")).not.toBeInTheDocument();
  });

  it("opens the guide on its first step, with the status's command verbatim", async () => {
    rotation.value = { ...rotation.value, rotate_command: "cd /srv/example && make zitadel-actions-rotate-key" };
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Do it by hand" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Step 1 of 2")).toBeInTheDocument();
    expect(within(dialog).getByText("cd /srv/example && make zitadel-actions-rotate-key")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
  });

  it("says when Syndra last read the keys from Zitadel", async () => {
    keyring.value = { read_at: new Date().toISOString(), changed_at: null, reads: 4 };
    renderPage();
    expect(await screen.findByText(/event listener's key from Zitadel every minute.*never needs a restart. Last read at \d\d:\d\d today/)).toBeInTheDocument();
  });

  it("tells the owner what to grant when Zitadel refuses the key read", async () => {
    keyring.value = { read_at: null, changed_at: null, error: { status: 403 }, reads: 2 };
    renderPage();
    expect(await screen.findByText(/isn't allowed to read the key.*IAM_OWNER_VIEWER/)).toBeInTheDocument();
  });

  it("keeps the claims delivery section in its seat", async () => {
    renderPage();
    expect(await screen.findByRole("region", { name: "Claims delivery" })).toHaveAttribute("id", "claims-delivery");
  });

  it("offers register, not rotate, when there is no key", async () => {
    rotation.value = { key_installed: false, status: "disabled", threshold_days: 90 };
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "How to set it up" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("make zitadel-actions-register")).toBeInTheDocument();
    expect(within(dialog).queryByText("make zitadel-actions-rotate-key")).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Let Syndra do it now" })).not.toBeInTheDocument();
  });
});

// The read-only doors used to be labelled by their routes ("/zitadel/projects"). A route is where
// the link goes, not what the row is.
describe("Identity provider · Look inside names each door", () => {
  it("gives each row a plain name and a line, and prints no route", async () => {
    renderPage();
    expect(await screen.findByText("Each with how it compares with Syndra's record")).toBeInTheDocument();
    expect(screen.getByText("Everyone Zitadel knows, and the roles it holds for each")).toBeInTheDocument();
    expect(screen.getByText("Every role on every person, in one list")).toBeInTheDocument();
    expect(screen.queryByText(/^\/zitadel\//)).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open people" })).toHaveAttribute("href", "/zitadel/users");
  });

  it("shows the names but no links while Zitadel is not answering, and says why", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    renderPage();
    expect(await screen.findByText("Open people")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Open people" })).not.toBeInTheDocument();
    expect(screen.getByText(/Unavailable while Zitadel isn.t answering: these read it live/)).toBeInTheDocument();
  });
});

/**
 * #72: the write half sits behind its disclosure, with the warning beside each
 * control. #75: each warning is its own surface's truth — a person's roles are
 * recorded (they go through Pending changes), a project or role edit is not.
 */
describe("Identity provider · Change Zitadel directly", () => {
  it("is folded: no write control is on the page until it is opened", async () => {
    renderPage();
    const fold = await screen.findByRole("button", { name: /Change Zitadel directly/ });
    expect(fold).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("last resort")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Change people.s roles/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Change projects and roles/ })).not.toBeInTheDocument();
  });

  it("puts a warning beside each control, each saying its own truth", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Change Zitadel directly/ }));

    const people = screen.getByRole("link", { name: /Change people.s roles/ });
    expect(people).toHaveAttribute("href", "/zitadel/users");
    const peopleRow = people.closest(".row-divider") as HTMLElement;
    expect(within(peopleRow).getByText(/Recorded in Syndra, but with no preview and no reason/)).toBeInTheDocument();
    expect(within(peopleRow).queryByText(/keeps no record/)).not.toBeInTheDocument();

    const projectsLink = screen.getByRole("link", { name: /Change projects and roles/ });
    expect(projectsLink).toHaveAttribute("href", "/zitadel/projects");
    const projectsRow = projectsLink.closest(".row-divider") as HTMLElement;
    expect(within(projectsRow).getByText(/Recorded in Syndra, then changed in Zitadel at once/)).toBeInTheDocument();
    expect(within(projectsRow).queryByText(/keeps no record/)).not.toBeInTheDocument();
    expect(within(projectsRow).getByRole("link", { name: "Projects" })).toHaveAttribute("href", "/projects");
  });

  it("offers no way in, with the reason, while Zitadel is not answering", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Change Zitadel directly/ }));
    expect(screen.queryByRole("link", { name: /Change people.s roles/ })).not.toBeInTheDocument();
    expect(screen.getByText(/Change people.s roles/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Change projects and roles/ })).not.toBeInTheDocument();
    expect(screen.getByText(/Unavailable while Zitadel isn.t answering: these change it directly/)).toBeInTheDocument();
  });
});
