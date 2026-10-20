// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";
import type { TargetHealth, TargetInventory, TargetSummary } from "@/lib/queries/useTargets";
import type { TargetSystemHealth } from "@/lib/queries/useTargetSystemHealth";

/**
 * route-system-targets-id, as the board draws it and A-01 (14 Oct) decides it:
 * jump chips, "Whether changes can land" as two cards, Maintenance as two action
 * rows. Layout only: every call below is the one the page made before.
 */

const miss = { value: undefined, resolved: true } as const;
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: () => miss,
    resolveProject: () => miss,
    resolveRole: () => miss,
    resolveBundle: () => miss,
  }),
}));

const state = {
  roster: [] as TargetSummary[],
  health: {} as TargetHealth | undefined,
  healthLoading: false,
  healthError: null as Error | null,
  inventory: {} as TargetInventory,
  system: undefined as TargetSystemHealth | undefined,
  systemLoading: false,
  lifecycleCalls: [] as Array<{ state: string; reason: string }>,
  lifecycleError: null as Error | null,
  refetched: 0,
};

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({ data: state.system, isLoading: state.systemLoading, isError: false }),
}));

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
  return {
    ...actual,
    useTargets: () => ({ data: state.roster, isLoading: false, error: null }),
    useTargetHealth: () => ({
      data: state.health,
      isLoading: state.healthLoading,
      error: state.healthError,
      refetch: () => {
        state.refetched++;
      },
    }),
    useTargetInventory: () => ({ data: state.inventory, isLoading: false, error: null, refetch: vi.fn() }),
    useMergeFindings: () => ({ data: [], isLoading: false, error: null }),
    useSetLifecycle: () => ({
      mutate: (input: { state: string; reason: string }) => state.lifecycleCalls.push(input),
      isPending: false,
      error: state.lifecycleError,
    }),
    useTargetHomes: () => ({ data: { read: true, usage_read: true, accounts: [] }, isLoading: false, error: null }),
    useTargetSettings: () => ({ data: { read: true, settings: [] }, isLoading: false, error: null, refetch: vi.fn() }),
    useReconcileTarget: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
    useResolveBindingConflict: inert,
    useResolveLogFinding: inert,
  };
});
vi.mock("@/components/targets/MergeFindings", () => ({ MergeFindings: () => null }));
vi.mock("@/components/targets/DormantAccounts", () => ({ DormantAccounts: () => null }));
vi.mock("@/components/targets/PeopleOnTarget", () => ({ PeopleOnTarget: () => null }));

function renderTarget() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TargetOverview target="truenas" />
    </QueryClientProvider>,
  );
}

function ready(health: Partial<TargetHealth> = {}) {
  state.roster = [
    {
      target: "truenas",
      registered: true,
      auth_mode: "derived",
      callable: true,
      operations: [],
      circuit_open: false,
      project: { id: "p1", name: "Network storage" },
    },
  ];
  state.health = { reachable: true, lifecycle: "active", status: "answering", ...health } as TargetHealth;
  state.inventory = { target: "truenas", bound: 48, unmanaged: [], current: true };
}

beforeEach(() => {
  state.health = {} as TargetHealth;
  state.healthLoading = false;
  state.healthError = null;
  state.system = undefined;
  state.systemLoading = false;
  state.lifecycleCalls = [];
  state.lifecycleError = null;
  state.refetched = 0;
  ready();
});

describe("the jump chips", () => {
  it("are one link per region, with what each holds, in the board's order", () => {
    state.health = { reachable: true, lifecycle: "active", status: "answering", log_anchor: undefined } as TargetHealth;
    renderTarget();

    const nav = screen.getByRole("navigation", { name: "Regions of this page" });
    const links = within(nav).getAllByRole("link");
    expect(links.map((a) => a.getAttribute("href"))).toEqual(["#waiting", "#accounts", "#runs"]);
    expect(links[0]).toHaveTextContent("Waiting on you");
    expect(links[1]).toHaveTextContent("Accounts");
    expect(links[1]).toHaveTextContent("48");
    expect(links[2]).toHaveTextContent("What runs here");
    // Each lands on a region that keeps its seat.
    for (const id of ["land", "waiting", "accounts", "runs"]) expect(document.getElementById(id)).not.toBeNull();
  });

  it("leave out a count that has not been read", () => {
    state.inventory = { target: "truenas", unmanaged: [], current: true } as unknown as TargetInventory;
    renderTarget();

    const accounts = within(screen.getByRole("navigation", { name: "Regions of this page" })).getByRole("link", {
      name: /Accounts/,
    });
    expect(accounts.textContent).not.toMatch(/\d/);
  });
});

describe("whether changes can land", () => {
  it("is two cards, what Syndra can reach and what TrueNAS reports, then Maintenance", () => {
    state.system = {
      target: "truenas",
      readable: true,
      system: { hostname: "nas-01", version: "25.10.5" },
      alerts: [],
      pools: [],
      services: [],
    } as unknown as TargetSystemHealth;
    renderTarget();

    const land = document.getElementById("land") as HTMLElement;
    expect(within(land).getByRole("heading", { name: "Whether changes can land" })).toBeInTheDocument();
    const reach = within(land).getByText("Syndra's reach").closest(".card") as HTMLElement;
    const nas = within(land).getByText("What TrueNAS reports").closest(".card") as HTMLElement;
    expect(reach).not.toBe(nas);
    expect(within(reach).getByText("Answering")).toBeInTheDocument();
    expect(within(nas).getByText("All read")).toBeInTheDocument();
    expect(within(land).getByText("Maintenance")).toBeInTheDocument();
  });

  it("keeps the second card in its seat while TrueNAS has not answered, with no reading in it", () => {
    state.systemLoading = true;
    renderTarget();

    const nas = screen.getByText("What TrueNAS reports").closest(".card") as HTMLElement;
    expect(within(nas).queryByText(/Nothing raised/)).toBeNull();
    expect(within(nas).queryByText("All read")).toBeNull();
  });

  it("puts a finding about the connection in the first card, ending in where to look", () => {
    ready({ key_expiry: "unrecorded" });
    renderTarget();

    const reach = screen.getByText("Syndra's reach").closest(".card") as HTMLElement;
    expect(within(reach).getByText("API key expiry not recorded")).toBeInTheDocument();
  });
});

// A-01 §1: the four readings, and which one wins.
describe("the states between answering and offline", () => {
  const reach = () => screen.getByText("Syndra's reach").closest(".card") as HTMLElement;

  it("answering after a missed check is still Answering, with a faint line and the check pips", () => {
    ready({
      health: { status: "online", recent: ["answered", "answered", "missed"], missed_since: "2026-10-20T09:40:00Z" },
    } as Partial<TargetHealth>);
    renderTarget();

    expect(within(reach()).getByText("Answering")).toBeInTheDocument();
    expect(within(reach()).getByText(/Missed 1 check/)).toBeInTheDocument();
    expect(within(reach()).getByRole("img", { name: "Last 3 checks: answered, answered, missed" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("offline is one amber Notice, and the word Offline with its pips in the card", () => {
    ready({
      status: "offline",
      reachable: false,
      health: { status: "offline", recent: ["missed", "missed", "missed"], reachable_since: "2026-10-20T08:33:00Z" },
    } as Partial<TargetHealth>);
    renderTarget();

    expect(screen.getByText("TrueNAS isn't answering.")).toBeInTheDocument();
    expect(within(reach()).getByText("Offline")).toBeInTheDocument();
    expect(screen.getAllByRole("img", { name: /Last 3 checks: missed, missed, missed/ })).toHaveLength(1);
    expect(screen.getAllByText(/Look at TrueNAS\./).length).toBeGreaterThan(0);
  });

  it("checking after a restart says so, and offers no pips of answers", () => {
    ready({ status: "checking", health: { status: "unknown", recent: [] } } as Partial<TargetHealth>);
    renderTarget();

    expect(within(reach()).getByText("Checking")).toBeInTheDocument();
    expect(within(reach()).getByRole("img", { name: "Last 3 checks: no check yet, no check yet, no check yet" })).toBeInTheDocument();
  });

  it("the breaker is Waiting to retry, never Paused", () => {
    ready({ status: "waiting_to_retry", circuit_open: true } as Partial<TargetHealth>);
    renderTarget();

    expect(within(reach()).getByText("Waiting to retry")).toBeInTheDocument();
    expect(screen.queryByText(/Paused/)).toBeNull();
  });

  it("a page that could not be read says Syndra's own server failed, and tries again", () => {
    state.health = undefined;
    state.healthError = new Error("500");
    renderTarget();

    expect(screen.getByText("This page for TrueNAS could not be read.")).toBeInTheDocument();
    expect(screen.queryByText("All read")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(state.refetched).toBe(1);
  });
});

// The writes below reach TrueNAS. The calls are the ones the segmented control made.
describe("maintenance, as two action rows", () => {
  const maintenance = () => screen.getByText("Maintenance").closest(".card") as HTMLElement;

  it("while active, offers Start draining and Set read-only, each with what it does", () => {
    renderTarget();

    const card = maintenance();
    expect(within(card).getByText("Active")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Start draining" })).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Set read-only" })).toBeInTheDocument();
    expect(within(card).getByText(/Neither takes anyone's access away/)).toBeInTheDocument();
    expect(within(card).queryByRole("button", { name: "Make active" })).toBeNull();
    expect(within(card).queryByLabelText("Reason")).toBeNull();
  });

  it("asks why before sending, and sends the same call as before", () => {
    renderTarget();

    fireEvent.click(within(maintenance()).getByRole("button", { name: "Start draining" }));
    const confirm = within(maintenance()).getByRole("button", { name: "Start draining" });
    expect(confirm).toBeDisabled();
    expect(within(maintenance()).getByText(/Say why first/)).toBeInTheDocument();

    fireEvent.change(within(maintenance()).getByLabelText("Reason"), { target: { value: "rotating the key" } });
    expect(confirm).not.toBeDisabled();
    fireEvent.click(confirm);

    expect(state.lifecycleCalls).toEqual([{ state: "draining", reason: "rotating the key" }]);
  });

  it("sets read-only with its own reason", () => {
    renderTarget();

    fireEvent.click(within(maintenance()).getByRole("button", { name: "Set read-only" }));
    fireEvent.change(within(maintenance()).getByLabelText("Reason"), { target: { value: "disk work" } });
    fireEvent.click(within(maintenance()).getByRole("button", { name: "Set read-only" }));

    expect(state.lifecycleCalls).toEqual([{ state: "read_only", reason: "disk work" }]);
  });

  it("while draining, says so and offers Make active and Set read-only", () => {
    ready({ lifecycle: "draining", lifecycle_note: "rotating the API key" });
    renderTarget();

    const card = maintenance();
    expect(within(card).getByText("Finishing up")).toBeInTheDocument();
    expect(within(card).queryByRole("button", { name: "Start draining" })).toBeNull();
    fireEvent.click(within(card).getByRole("button", { name: "Make active" }));
    fireEvent.change(within(card).getByLabelText("Reason"), { target: { value: "done" } });
    fireEvent.click(within(card).getByRole("button", { name: "Make active" }));

    expect(state.lifecycleCalls).toEqual([{ state: "active", reason: "done" }]);
  });

  it("keeps the typed reason when TrueNAS refused, and says the state is unchanged", () => {
    state.lifecycleError = new Error("refused");
    renderTarget();

    fireEvent.click(within(maintenance()).getByRole("button", { name: "Set read-only" }));
    fireEvent.change(within(maintenance()).getByLabelText("Reason"), { target: { value: "disk work" } });

    expect(within(maintenance()).getByText(/That did not go through/)).toBeInTheDocument();
    expect((within(maintenance()).getByLabelText("Reason") as HTMLTextAreaElement).value).toBe("disk work");
  });
});

// Compare 20 Oct, dev box: the page spoke in raw ids and device text, had no way back, and drew
// "Unnamed project" where the board draws the project's link.
describe("the page in plain words (route-system-targets-id)", () => {
  it("has the way back to Connected systems above the title", () => {
    renderTarget();
    const back = screen.getByRole("link", { name: /Connected systems/ });
    expect(back.getAttribute("href")).toBe("/system/targets");
    expect(
      back.compareDocumentPosition(screen.getByRole("heading", { name: "TrueNAS" })) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("names each thing it can do in a plain verb, with the id beneath and how it runs", () => {
    state.roster[0].operations = [
      { id: "account.provision", scope: "member", confirm: false, available: true },
      { id: "account.purge", scope: "admin", confirm: true, secret_params: ["elevated_key"], available: true },
      { id: "thing.unheard_of", scope: "member", confirm: false, available: true },
    ] as TargetSummary["operations"];
    renderTarget();

    const row = (id: string) => screen.getByText(id).closest(".flex-wrap") as HTMLElement;
    expect(within(row("account.provision")).getByText("Creates an account")).toBeInTheDocument();
    expect(within(row("account.provision")).getByText("Runs by itself")).toBeInTheDocument();
    expect(within(row("account.purge")).getByText("Deletes an account and its files")).toBeInTheDocument();
    expect(within(row("account.purge")).getByText("Stops and asks")).toBeInTheDocument();
    // No scope, no secret parameter name; an id Syndra has no word for is not made the label.
    expect(screen.queryByText("elevated_key")).toBeNull();
    expect(screen.queryByText(/never logged: /)).toBeNull();
    expect(within(row("thing.unheard_of")).getByText(/Syndra has no name for/)).toBeInTheDocument();
    // Beside the reconciliation card, not under it.
    expect(screen.getByText("What it can do").closest(".grid")).toBe(
      screen.getByText("Check for differences").closest(".grid"),
    );
  });

  it("quotes the device's words as the device's, with when it raised them", () => {
    state.system = {
      target: "truenas",
      readable: true,
      alerts: [
        {
          level: "WARNING",
          klass: "SMARTUncorrectedErrors",
          text: "1 uncorrectable errors reported for sde (SERIAL0000).",
          at: "2026-09-14T09:00:00Z",
          dismissed: false,
        },
      ],
      pools: [
        { name: "tank", status: "ONLINE", healthy: true, warning: false, free_bytes: 1, allocated_bytes: 1, size_bytes: 2 },
      ],
    };
    renderTarget();

    expect(screen.getByText("A disk reports errors it could not correct")).toBeInTheDocument();
    expect(screen.getByText(/TrueNAS raised this on 14 Sept and has not cleared it\./)).toBeInTheDocument();
    expect(screen.getByText(/its words, not Syndra.s: .1 uncorrectable errors reported for sde/)).toBeInTheDocument();
    expect(screen.getByText(/^Online/)).toBeInTheDocument();
    expect(screen.queryByText(/^ONLINE/)).toBeNull();
  });

  it("links the project by its name, or says it could not be named, never 'Unnamed project'", () => {
    state.roster[0].project = { id: "p1", name: "" };
    renderTarget();
    expect(screen.queryByText("Unnamed project")).toBeNull();
    expect(screen.getByRole("link", { name: "Unknown project" }).getAttribute("href")).toBe(
      "/projects/p1",
    );
  });

  it("says in the header that no project is chosen yet", () => {
    state.roster[0].project = null;
    renderTarget();
    expect(screen.getByText("no Zitadel project yet")).toBeInTheDocument();
  });
});
