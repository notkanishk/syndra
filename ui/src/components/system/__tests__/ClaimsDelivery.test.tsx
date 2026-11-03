// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ClaimsDelivery } from "@/components/system/ClaimsDelivery";
import type { ClaimsDelivery as Delivery, ClaimFinding, ConfigRow } from "@/lib/queries/useClaimsDelivery";

const q = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
vi.mock("@/lib/queries/useClaimsDelivery", () => ({ useClaimsDelivery: () => q.value }));
vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [{ project: { id: "pNet", name: "Network" } }] }),
}));

const now = () => new Date().toISOString();
const DAY = 24 * 3600_000;

function delivery(over: Partial<Delivery> = {}): Delivery {
  return {
    window: "24h",
    since: now(),
    totals: { served: 40 },
    findings: [],
    apps: [
      {
        client_id: "c-net",
        app_id: "a-net",
        name: "NetBird",
        project_id: "pNet",
        last_sign_in_at: now(),
        last_call_at: now(),
        last_served_at: now(),
        last_outcome: "served",
        calls: 40,
        outcomes: { served: 40 },
        p99_le_ms: 5,
      },
      {
        client_id: "c-wiki",
        app_id: "a-wiki",
        name: "Wiki",
        project_id: "pNet",
        last_sign_in_at: now(),
        last_call_at: null,
        last_served_at: null,
        calls: 0,
        outcomes: {},
        p99_le_ms: null,
      },
      { client_id: "9912", last_sign_in_at: null, last_call_at: now(), last_served_at: null, calls: 2, outcomes: { no_grants: 2 }, p99_le_ms: 1 },
    ],
    sign_ins: { checking_since: new Date(Date.now() - 3 * DAY).toISOString(), last_24h: { delivered: 40 }, waiting: 0, dropped: 0, last_sign_in_at: now() },
    configuration: {
      checked_at: now(),
      took_ms: 80,
      rows: [
        { check: "R1", subject: "R1:present", state: "pass" },
        { check: "R1", subject: "R1:endpoint", state: "pass", expected: "https://syndra.example/zitadel/actions/claims", found: "https://syndra.example/zitadel/actions/claims" },
        { check: "R8", subject: "R8", state: "not_needed" },
      ],
    },
    keyring: { read_at: now(), changed_at: null, reads: 3 },
    alert: { configured: true, notes: [] },
    ...over,
  };
}

const finding = (over: Partial<ClaimFinding>): ClaimFinding => ({
  kind: "CLAIMS_NOT_CALLED",
  severity: "hard",
  subject: "c-net",
  first_seen: now(),
  last_seen: now(),
  count: 1,
  resolved_at: null,
  ...over,
});

const set = (d: Delivery) => {
  q.value = { data: d, error: null, isLoading: false, dataUpdatedAt: Date.now(), refetch: () => {} };
};
const renderIt = () => render(<ClaimsDelivery domain="auth.example.org" connected />);

beforeEach(() => set(delivery()));

describe("Claims delivery, calm", () => {
  it("is one sentence a part, with no Notice and the detail folded", () => {
    renderIt();
    expect(screen.getByRole("region", { name: "Claims delivery" })).toHaveAttribute("id", "claims-delivery");
    expect(screen.getByText("Every sign-in in the last 24h got its claims.")).toBeInTheDocument();
    expect(screen.getByText(/^Zitadel's setup matches what claim delivery needs\. Checked \d\d:\d\d\.$/)).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.getByRole("button", { name: "Show each app" })).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("NetBird")).toBeNull();
    expect(screen.queryByRole("list", { name: "Configuration checklist" })).toBeNull();
  });

  it("lists every app on demand, names linking, numbers last, an unknown app said plainly", () => {
    renderIt();
    fireEvent.click(screen.getByRole("button", { name: "Show each app" }));
    expect(screen.getByRole("link", { name: "NetBird" })).toHaveAttribute("href", "/applications/a-net");
    expect(screen.getByText("40 served")).toBeInTheDocument();
    expect(screen.getByText("under 5 ms")).toBeInTheDocument();
    expect(screen.getByText("No calls")).toBeInTheDocument();
    expect(screen.getByText("Not an app in Syndra")).toBeInTheDocument();
    expect(screen.getByText("9912")).toBeInTheDocument();
    expect(screen.getByText(/the token named no project: 2/)).toBeInTheDocument();
  });

  it("says sign-ins before a restart aren't checked, never failed", () => {
    // Midday, so "an hour ago" is today and reads as a time, not a date
    // (it failed just after midnight, 4 Nov 2026).
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 10, 4, 12, 0, 0));
    try {
      set(delivery({ sign_ins: { ...delivery().sign_ins, checking_since: new Date(Date.now() - 3600_000).toISOString() } }));
      renderIt();
      expect(screen.getByText(/^Every sign-in since \d\d:\d\d got its claims\.$/)).toBeInTheDocument();
      expect(screen.getByText(/Sign-ins before that aren.t checked, and none is counted as failed/)).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("says plainly when the setup hasn't been checked", () => {
    set(delivery({ configuration: null }));
    render(<ClaimsDelivery connected={false} />);
    expect(screen.getByText(/Not checked: Syndra isn't connected to Zitadel/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Show the checklist" })).toBeNull();
  });

  it("notes a missing outside alert quietly, not as a Notice", () => {
    set(delivery({ alert: { configured: false, notes: ["NO_EXTERNAL_ALERT"] } }));
    renderIt();
    expect(screen.getByText(/No alert outside Syndra is set up/)).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("claims nothing while loading", () => {
    q.value = { data: undefined, error: null, isLoading: true, dataUpdatedAt: 0, refetch: () => {} };
    renderIt();
    expect(screen.getByRole("region", { name: "Claims delivery" })).toBeInTheDocument();
    expect(screen.queryByText(/got its claims|matches/)).toBeNull();
  });
});

describe("Claims delivery, with problems", () => {
  it("draws one Notice per dysfunction, naming every app it touches, and opens the apps", () => {
    set(
      delivery({
        findings: [
          finding({ subject: "c-net" }),
          finding({ subject: "c-wiki" }),
          finding({ kind: "CLAIMS_NOT_CALLED", subject: "c-old", resolved_at: now() }),
        ],
      }),
    );
    renderIt();
    const notices = screen.getAllByRole("status");
    expect(notices).toHaveLength(1);
    expect(within(notices[0]).getByText("Zitadel didn't reach Syndra for 2 apps")).toBeInTheDocument();
    // Notices stay plain prose: the names are there, the links live in the table.
    expect(notices[0]).toHaveTextContent("NetBird");
    expect(notices[0]).toHaveTextContent("Wiki");
    expect(within(notices[0]).queryByRole("link")).toBeNull();
    expect(notices[0].className).toMatch(/danger/);
    expect(screen.getByText("2 apps need a look in the last 24h.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hide each app" })).toHaveAttribute("aria-expanded", "true");
    // The settled one is a quiet line, not a box.
    expect(screen.getByText(/Settled in the last 24h: 1 problem/)).toBeInTheDocument();
  });

  it("says why a call went without claims, in plain words, amber for a soft class", () => {
    set(delivery({ findings: [finding({ kind: "CLAIMS_NOT_SERVED", severity: "soft", sample: { outcome: "degraded_no_facts" } })] }));
    renderIt();
    const notice = screen.getByRole("status");
    expect(notice).toHaveTextContent("Syndra answered 1 app without people's claims");
    expect(notice).toHaveTextContent("NetBird: their access wasn't built yet.");
    expect(notice.className).toMatch(/warn/);
  });

  it("shows a drifted row's expected and found, and the one place to fix it", () => {
    const drift: ConfigRow = {
      check: "R2",
      subject: "R2:preaccesstoken",
      state: "drift",
      expected: ["syndra-claim-injector"],
      found: ["syndra-claim-injector", "old-script"],
    };
    set(delivery({ findings: [finding({ kind: "CONFIG_DRIFT", subject: "R2:preaccesstoken" })], configuration: { ...delivery().configuration!, rows: [drift] } }));
    renderIt();
    expect(screen.getByRole("status")).toHaveTextContent("Zitadel's setup differs from what claim delivery needs");
    expect(screen.getByText(/^1 check differs from what claim delivery needs/)).toBeInTheDocument();
    const list = screen.getByRole("list", { name: "Configuration checklist" });
    const row = within(list).getByText("Making an access token calls only the claims action").closest("li")!;
    expect(row).toHaveTextContent("Expected syndra-claim-injector · found syndra-claim-injector, old-script");
    expect(within(row).getByRole("link", { name: /Open Actions in Zitadel/ })).toHaveAttribute(
      "href",
      "https://auth.example.org/ui/console/actions",
    );
  });

  it("tells the owner what to grant when Zitadel refuses a read", () => {
    set(
      delivery({
        findings: [finding({ kind: "CANNOT_CHECK", severity: "soft", subject: "R3", sample: { status: 403 } })],
        configuration: { ...delivery().configuration!, rows: [{ check: "R3", subject: "R3", state: "cannot", facts: { status: 403 } }] },
      }),
    );
    renderIt();
    expect(screen.getByRole("status")).toHaveTextContent(/couldn.t read the event listener\. Zitadel refused Syndra.s service account\. Give it the read-only IAM_OWNER_VIEWER role/);
    const row = screen.getByText("Couldn't read the event listener in Zitadel").closest("li")!;
    expect(row).toHaveTextContent(/IAM_OWNER_VIEWER/);
    expect(screen.getByText(/^Syndra couldn't read 1 part of Zitadel's setup/)).toBeInTheDocument();
  });

  it("names an app's settings in Zitadel as the fix for R5", () => {
    const r5: ConfigRow = {
      check: "R5",
      subject: "R5:a-net",
      state: "drift",
      expected: "access_token",
      found: { access_token_type: "BEARER", id_token_userinfo_assertion: false },
      facts: { app_id: "a-net", project_id: "pNet", reads: "access_token" },
    };
    set(delivery({ configuration: { ...delivery().configuration!, rows: [r5] } }));
    renderIt();
    const row = screen.getByText("NetBird's claims reach where its shape sends them").closest("li")!;
    expect(row).toHaveTextContent("Expected The access token · found BEARER access token, userinfo outside the ID token");
    expect(within(row).getByRole("link", { name: /Open NetBird in Zitadel/ })).toHaveAttribute(
      "href",
      "https://auth.example.org/ui/console/projects/pNet/apps/a-net",
    );
  });
});
