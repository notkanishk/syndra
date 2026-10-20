// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UnexplainedAccess } from "@/components/review/UnexplainedAccess";
import type { DriftTriageItem } from "@/lib/queries/useDrift";

/**
 * The Drift list as the board draws it (route-drift): rows gathered by
 * direction into two foldable cards, a search seat, headings that sort, and
 * one selection that spans both. Everything here is asserted through the page.
 */
const drift = vi.hoisted(() => ({
  data: [] as DriftTriageItem[],
  isLoading: false,
  error: null as unknown,
  unread: [] as Array<{ target: string; since: string; reason?: string }>,
}));

// The reconciliation read: what the comparison shows beside the findings.
const emptyDiff = () => ({
  only_in_syndra: [] as Array<{ user_id: string; project_id: string; role_keys: string[] }>,
  only_in_zitadel: [] as Array<{ user_id: string; project_id: string; role_keys: string[] }>,
  drift: [] as Array<{
    user_id: string;
    project_id: string;
    only_in_zitadel: string[];
    only_in_syndra: string[];
  }>,
  generated_at: "2026-10-20T08:00:00Z",
  truncated: false,
});
const diff = vi.hoisted(() => ({ data: undefined as ReturnType<typeof emptyDiff> | undefined }));

const NAMES: Record<string, string> = { u1: "Meera Pillai", u2: "Arjun Das", u3: "Tomas Weber" };
const PROJECTS: Record<string, string> = { p1: "Electronics bench", p2: "Wood shop" };

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({
      value: NAMES[id] ? { display_name: NAMES[id] } : undefined,
      resolved: true,
    }),
    resolveProject: (id: string) => ({
      value: PROJECTS[id] ? { name: PROJECTS[id] } : undefined,
      resolved: true,
    }),
  }),
}));

vi.mock("@/lib/queries/useGrants", () => ({
  useReconciliationDiff: () => ({ data: diff.data, error: null, refetch: () => {} }),
}));

vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({ data: { unreconciled_targets: drift.unread } }),
}));

vi.mock("@/lib/queries/useDrift", () => {
  const mutation = () => ({ mutateAsync: vi.fn(), isPending: false });
  return {
    useDriftItems: () => ({
      data: drift.data,
      isLoading: drift.isLoading,
      error: drift.error,
      refetch: () => {},
      dataUpdatedAt: Date.now(),
    }),
    useAttributeDrift: mutation,
    useRevokeDrift: mutation,
    useMarkExternalDrift: mutation,
    useRehearseAdoptDrift: mutation,
    useRehearseMarkExternalDrift: mutation,
    useBulkAttributeDrift: mutation,
    useBulkMarkExternalDrift: mutation,
    useReconcileNow: mutation,
    useDriftOrigin: () => ({ data: undefined, isLoading: false, isError: false }),
  };
});

function item(over: Partial<DriftTriageItem> = {}): DriftTriageItem {
  return {
    id: "d1",
    user_id: "u1",
    project_id: "p1",
    role_keys: ["operator"],
    drift_type: "target_only",
    detection_source: "reconciliation_sweep",
    detected_at: "2026-10-18T06:00:00Z",
    target: "zitadel",
    role_in_catalogue: true,
    role_catalogue_applies: true,
    user_is_service_account: false,
    other_items_for_user: 0,
    ...over,
  } as DriftTriageItem;
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UnexplainedAccess />
    </QueryClientProvider>,
  );
}

const IN_ZITADEL = /In Zitadel, not given by Syndra/;
const GONE = /Given by Syndra, gone from Zitadel/;

/** Names of the people in a section, in the order they are drawn. */
function peopleIn(section: HTMLElement): string[] {
  return within(section)
    .getAllByRole("link", { name: /Meera|Arjun|Tomas/ })
    .map((link) => link.textContent ?? "");
}

beforeEach(() => {
  drift.data = [
    item({ id: "a", user_id: "u1", detected_at: "2026-10-18T06:00:00Z" }),
    item({ id: "b", user_id: "u2", detected_at: "2026-10-12T06:00:00Z" }),
    item({ id: "c", user_id: "u3", drift_type: "syndra_only", detected_at: "2026-10-15T06:00:00Z" }),
  ];
  drift.isLoading = false;
  drift.error = null;
  drift.unread = [];
  diff.data = emptyDiff();
});

describe("Drift — sections", () => {
  it("gathers rows by direction, each section with its count", () => {
    renderPage();
    const zitadel = screen.getByRole("region", { name: IN_ZITADEL });
    const gone = screen.getByRole("region", { name: GONE });

    expect(within(zitadel).getByTestId("card-count")).toHaveTextContent("2");
    expect(peopleIn(zitadel)).toEqual(["Meera Pillai", "Arjun Das"]);
    expect(within(gone).getByTestId("card-count")).toHaveTextContent("1");
    expect(peopleIn(gone)).toEqual(["Tomas Weber"]);
  });

  it("folds a section to its heading and opens it again", () => {
    renderPage();
    const zitadel = screen.getByRole("region", { name: IN_ZITADEL });
    const heading = within(zitadel).getByRole("button", { name: IN_ZITADEL });
    expect(heading).toHaveAttribute("aria-expanded", "true");

    fireEvent.click(heading);
    expect(heading).toHaveAttribute("aria-expanded", "false");
    expect(within(zitadel).queryByText("Meera Pillai")).toBeNull();
    // The other direction is untouched.
    expect(screen.getByText("Tomas Weber")).toBeInTheDocument();

    fireEvent.click(heading);
    expect(within(zitadel).getByText("Meera Pillai")).toBeInTheDocument();
  });

  it("shows ten rows a section, then offers the rest", () => {
    drift.data = Array.from({ length: 12 }, (_, at) =>
      item({ id: `d${at}`, user_id: "u1", detected_at: `2026-10-${String(at + 1).padStart(2, "0")}T06:00:00Z` }),
    );
    renderPage();
    const zitadel = screen.getByRole("region", { name: IN_ZITADEL });

    expect(within(zitadel).getAllByRole("button", { name: "Adopt" })).toHaveLength(10);
    expect(within(zitadel).getByText("Showing 10 of 12")).toBeInTheDocument();

    fireEvent.click(within(zitadel).getByRole("button", { name: "Show 2 more" }));
    expect(within(zitadel).getAllByRole("button", { name: "Adopt" })).toHaveLength(12);
  });

  it("names each section in a chip on the read line, and a chip opens a folded section", () => {
    renderPage();
    const nav = screen.getByRole("navigation", { name: "Sections of drift" });
    expect(within(nav).getByRole("link", { name: /Not given by Syndra\s*2/ })).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: /Gone from Zitadel\s*1/ })).toBeInTheDocument();

    const gone = screen.getByRole("region", { name: GONE });
    fireEvent.click(within(gone).getByRole("button", { name: GONE }));
    expect(within(gone).queryByText("Tomas Weber")).toBeNull();

    fireEvent.click(within(nav).getByRole("link", { name: /Gone from Zitadel/ }));
    expect(within(gone).getByText("Tomas Weber")).toBeInTheDocument();
  });

  it("says how many are waiting and how long the oldest has", () => {
    renderPage();
    expect(screen.getByText(/3 to decide/)).toBeInTheDocument();
    expect(screen.getByText(/oldest found/)).toBeInTheDocument();
  });
});

describe("Drift — search", () => {
  it("finds a person by name, across both sections", () => {
    renderPage();
    fireEvent.change(screen.getByRole("textbox", { name: "Search drift by person or role" }), {
      target: { value: "tomas" },
    });

    expect(screen.queryByRole("region", { name: IN_ZITADEL })).toBeNull();
    expect(within(screen.getByRole("region", { name: GONE })).getByText("Tomas Weber")).toBeInTheDocument();
  });

  it("finds a role by its project and its key", () => {
    drift.data = [
      item({ id: "a", user_id: "u1", project_id: "p1", role_keys: ["operator"] }),
      item({ id: "b", user_id: "u2", project_id: "p2", role_keys: ["member"] }),
    ];
    renderPage();
    const box = screen.getByRole("textbox", { name: "Search drift by person or role" });

    fireEvent.change(box, { target: { value: "wood" } });
    expect(screen.queryByText("Meera Pillai")).toBeNull();
    expect(screen.getByText("Arjun Das")).toBeInTheDocument();

    fireEvent.change(box, { target: { value: "operator" } });
    expect(screen.getByText("Meera Pillai")).toBeInTheDocument();
    expect(screen.queryByText("Arjun Das")).toBeNull();
  });

  it("says nothing matches, and clearing is the way back", () => {
    renderPage();
    fireEvent.change(screen.getByRole("textbox", { name: "Search drift by person or role" }), {
      target: { value: "zzz" },
    });

    expect(screen.getByText("Nothing unexplained matches those filters.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(screen.getByText("Meera Pillai")).toBeInTheDocument();
  });
});

describe("Drift — sort", () => {
  it("starts with the most recently found on top", () => {
    renderPage();
    expect(peopleIn(screen.getByRole("region", { name: IN_ZITADEL }))).toEqual([
      "Meera Pillai",
      "Arjun Das",
    ]);
  });

  it("orders by who, and reverses on a second press", () => {
    renderPage();
    const zitadel = screen.getByRole("region", { name: IN_ZITADEL });

    fireEvent.click(within(zitadel).getByRole("button", { name: "Sort by who" }));
    expect(peopleIn(zitadel)).toEqual(["Arjun Das", "Meera Pillai"]);

    fireEvent.click(within(zitadel).getByRole("button", { name: "Sort by who" }));
    expect(peopleIn(zitadel)).toEqual(["Meera Pillai", "Arjun Das"]);
  });

  it("gathers equal reasons under a label with their count", () => {
    drift.data = [
      item({ id: "a", user_id: "u1", upstream_actor: "x1", detected_at: "2026-10-18T06:00:00Z" }),
      item({ id: "b", user_id: "u2", detected_at: "2026-10-17T06:00:00Z" }),
      item({ id: "c", user_id: "u3", upstream_actor: "x1", detected_at: "2026-10-16T06:00:00Z" }),
    ];
    renderPage();
    const zitadel = screen.getByRole("region", { name: IN_ZITADEL });
    fireEvent.click(within(zitadel).getByRole("button", { name: "Sort by why" }));

    // A to Z by reason; inside a run, newest first.
    const labels = within(zitadel).getAllByTestId("drift-run").map((el) => el.textContent);
    expect(labels).toEqual(["Made in Zitadel by a person2", "Maker not known1"]);
    expect(peopleIn(zitadel)).toEqual(["Meera Pillai", "Tomas Weber", "Arjun Das"]);
  });
});

describe("Drift — states", () => {
  it("says everything is explained when nothing is waiting, with no sections and no Select", () => {
    drift.data = [];
    renderPage();
    expect(screen.getByText("Everything is explained.")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: IN_ZITADEL })).toBeNull();
    expect(screen.queryByRole("button", { name: "Select" })).toBeNull();
  });

  it("shows no counts and no sections while the list is being read", () => {
    drift.isLoading = true;
    drift.data = [];
    renderPage();
    expect(screen.queryByText(/to decide/)).toBeNull();
    expect(screen.queryByRole("region", { name: IN_ZITADEL })).toBeNull();
  });

  it("says it could not load, and that nothing was changed", () => {
    drift.error = new Error("boom");
    drift.data = [];
    renderPage();
    expect(screen.getByText("Couldn't load drift.")).toBeInTheDocument();
  });

  it("says which system nobody could read, above the list", () => {
    drift.unread = [{ target: "truenas", since: "2026-10-01T22:10:00Z", reason: "read_refused" }];
    renderPage();
    expect(screen.getByText(/TrueNAS hasn.t been read since/)).toBeInTheDocument();
    expect(screen.getByText(/not in this list/)).toBeInTheDocument();
  });
});

describe("Drift — selection", () => {
  it("is one selection across both sections", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Select" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 3 items/ }));

    expect(screen.getByText(/3 items selected/)).toBeInTheDocument();
    expect(screen.getByText("Revoking is one row at a time.")).toBeInTheDocument();
  });

  it("keeps the evidence behind its own control, not the person's name", () => {
    drift.data = [item({ zitadel_grant_id: "zg_9" })];
    renderPage();

    expect(screen.getByRole("link", { name: "Meera Pillai" })).toHaveAttribute("href", "/users/u1");
    fireEvent.click(screen.getByRole("button", { name: "Show evidence" }));
    expect(screen.getByText("zg_9")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hide evidence" })).toBeInTheDocument();
  });

  it("marks every safety-gated row with the danger edge, not only the first", () => {
    drift.data = [
      item({ id: "a", user_id: "u1", role_group: "Safety-gated", detected_at: "2026-10-18T06:00:00Z" }),
      item({ id: "b", user_id: "u2", role_group: "Safety-gated", detected_at: "2026-10-12T06:00:00Z" }),
      item({ id: "c", user_id: "u3", detected_at: "2026-10-11T06:00:00Z" }),
    ];
    renderPage();
    expect(document.querySelectorAll("[data-danger-edge]")).toHaveLength(2);
  });
});

describe("Drift — what the comparison adds to the findings", () => {
  it("puts a both-sides row's roles in the card of the side that holds each", () => {
    drift.data = [];
    diff.data = {
      ...emptyDiff(),
      drift: [{ user_id: "u1", project_id: "p1", only_in_zitadel: ["laser"], only_in_syndra: ["kiln"] }],
    };
    renderPage();

    const zitadel = screen.getByRole("region", { name: IN_ZITADEL });
    const gone = screen.getByRole("region", { name: GONE });
    expect(within(zitadel).getByText("laser")).toBeInTheDocument();
    expect(within(zitadel).queryByText("kiln")).toBeNull();
    expect(within(gone).getByText("kiln")).toBeInTheDocument();
    expect(within(gone).queryByText("laser")).toBeNull();
    expect(within(zitadel).getByTestId("card-count")).toHaveTextContent("1");
    // Nothing to decide on a row that is not a finding: no answers, and no Select.
    expect(screen.queryByRole("button", { name: "Adopt" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Select" })).toBeNull();
    expect(screen.getByText("Everything else in Zitadel agrees with Syndra.")).toBeInTheDocument();
  });

  it("shows a role only one side has, and counts it with the findings", () => {
    diff.data = {
      ...emptyDiff(),
      only_in_zitadel: [{ user_id: "u2", project_id: "p2", role_keys: ["member"] }],
      only_in_syndra: [{ user_id: "u3", project_id: "p2", role_keys: ["booking"] }],
    };
    renderPage();

    const zitadel = screen.getByRole("region", { name: IN_ZITADEL });
    expect(within(zitadel).getByText("member")).toBeInTheDocument();
    expect(within(zitadel).getByTestId("card-count")).toHaveTextContent("3");
    expect(within(screen.getByRole("region", { name: GONE })).getByText("booking")).toBeInTheDocument();
    // The headline counts what can be decided, not what can only be read.
    expect(screen.getByText(/3 to decide/)).toBeInTheDocument();
  });

  it("does not repeat a role a finding already says", () => {
    drift.data = [item({ id: "a", user_id: "u1", project_id: "p1", role_keys: ["operator"] })];
    diff.data = {
      ...emptyDiff(),
      only_in_zitadel: [{ user_id: "u1", project_id: "p1", role_keys: ["operator"] }],
    };
    renderPage();

    const zitadel = screen.getByRole("region", { name: IN_ZITADEL });
    expect(within(zitadel).getAllByText("operator")).toHaveLength(1);
    expect(within(zitadel).getByTestId("card-count")).toHaveTextContent("1");
  });

  it("does not say everything agrees when the comparison was cut short", () => {
    diff.data = { ...emptyDiff(), truncated: true };
    renderPage();

    expect(screen.getByText("The comparison stopped early.")).toBeInTheDocument();
    expect(screen.queryByText("Everything else in Zitadel agrees with Syndra.")).toBeNull();
  });
});
