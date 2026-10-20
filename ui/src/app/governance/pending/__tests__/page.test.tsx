// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PendingChangesPage from "@/app/governance/pending/page";
import type { PendingRow } from "@/lib/queries/usePropagation";
import type { BundleRow } from "@/lib/queries/useBundles";
import type { MappingRuleRow } from "@/lib/queries/useMappingRules";

const pending = vi.hoisted(() => ({ data: [] as PendingRow[] }));
const summary = vi.hoisted(() => ({ reachable: true }));
const bundles = vi.hoisted(() => ({ data: [] as BundleRow[] }));
const rules = vi.hoisted(() => ({ data: [] as MappingRuleRow[] }));
const load = vi.hoisted(() => ({ loading: false }));
const names = vi.hoisted(() => ({ users: {} as Record<string, string> }));

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => {
    const miss = { value: undefined, resolved: true } as const;
    return {
      resolveUser: (id: string) =>
        names.users[id] ? { value: { display_name: names.users[id], email: "" }, resolved: true } : miss,
      resolveProject: () => miss,
      resolveRole: () => miss,
      resolveBundle: () => miss,
    };
  },
}));

vi.mock("@/lib/queries/usePropagation", () => ({
  usePendingPropagations: () => ({ ...pending, isLoading: load.loading, error: null, refetch: () => {} }),
  useDrainPropagations: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useBundles", () => ({
  useBundles: () => ({ ...bundles, isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({ ...rules, isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useGovernance", () => ({
  useGovernanceSummary: () => ({
    data: { pending_propagation: { count: 0, zitadel_reachable: summary.reachable } },
  }),
}));

function row(overrides: Partial<PendingRow> = {}): PendingRow {
  return {
    id: "o1",
    op_type: "add",
    user_id: "u1",
    project_id: "p1",
    role_keys: ["trained"],
    source: "rule",
    source_ref: "rule-1111-2222",
    cascade_id: "cas-8841-aaaa",
    status: "pending",
    attempts: 0,
    created_at: "2026-07-31T09:42:00Z",
    ...overrides,
  } as PendingRow;
}

function renderPending() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PendingChangesPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  pending.data = [];
  summary.reachable = true;
  bundles.data = [];
  rules.data = [];
  load.loading = false;
  names.users = {};
});

describe("Pending changes", () => {
  it("says why each row waits, warning only when a send stopped", () => {
    pending.data = [
      row({ id: "o1", role_keys: ["supervisor"], wait_reason: "role_waits", cascade_id: "c1" }),
      row({ id: "o2", role_keys: ["trained"], wait_reason: "halted:zitadel_offline", cascade_id: "c2" }),
      row({ id: "o3", cascade_id: "c3" }),
    ];
    renderPending();
    // Said in the change's own cell, not in a column of its own that is blank for most rows.
    expect(screen.queryByText("Why it\u2019s waiting")).not.toBeInTheDocument();
    const chosen = screen.getByText("Supervisor always waits for you");
    expect(chosen).toHaveClass("text-muted");
    const stopped = screen.getByText(/^Zitadel didn't answer at \d\d:\d\d$/);
    expect(stopped).toHaveClass("text-warn-text");
  });

  it("says two writes belong to one cascade rather than leaving them looking unrelated", () => {
    pending.data = [row({ id: "o1" }), row({ id: "o2", role_keys: ["door"] })];
    renderPending();
    expect(
      screen.getByText(/These 2 changes come from one edit/),
    ).toBeInTheDocument();
    expect(screen.getByText(/They are sent together or not at all\./)).toBeInTheDocument();
  });

  it("does not claim a lone write is a cascade", () => {
    pending.data = [row()];
    renderPending();
    expect(screen.queryByText(/share cascade/)).not.toBeInTheDocument();
  });

  it("keeps writes from different events apart", () => {
    pending.data = [
      row({ id: "o1", cascade_id: "cas-aaaa" }),
      row({ id: "o2", cascade_id: "cas-bbbb" }),
    ];
    renderPending();
    // Two separate single-write groups, so neither gets the shared-cascade line.
    expect(screen.queryByText(/share cascade/)).not.toBeInTheDocument();
  });

  it("explains a disabled confirm in visible copy, not a tooltip", () => {
    summary.reachable = false;
    pending.data = [row()];
    renderPending();

    expect(screen.getByText(/Sending waits until Zitadel answers/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Send / })).toBeDisabled();
  });

  // Zitadel not answering is ONE Notice, the shell's, above the page title. The
  // page used to add its own beneath the lede ("Zitadel is not answering."), so
  // the same fact was said twice, in two places, on this page only.
  it("leaves the Zitadel-not-answering Notice to the shell", () => {
    summary.reachable = false;
    pending.data = [row()];
    renderPending();

    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByText(/Zitadel is not answering/)).toBeNull();
  });
});

/**
 * Caused by: a name, never a bare handle standing in for one.
 */
describe("Pending changes · caused by", () => {
  it("names the bundle, with the handle kept alongside for tracing", () => {
    bundles.data = [{ id: "bundle-1111-2222", name: "Woodshop access" }];
    pending.data = [row({ source: "bundle", source_ref: "bundle-1111-2222" })];
    renderPending();

    expect(screen.getByText("Woodshop access")).toBeInTheDocument();
    expect(screen.getByText(/b_bund/)).toBeInTheDocument();
  });

  it("names the rule by its mapping when the bundle/rule list has not loaded", () => {
    rules.data = [
      {
        id: "rule-1111-2222",
        source_project: "p1",
        source_role: "trained",
        target_project: "p2",
        target_role: "door",
        holder_count: 3,
        created_at: "2026-01-01T00:00:00Z",
      },
    ];
    pending.data = [row({ source: "rule", source_ref: "rule-1111-2222" })];
    renderPending();

    expect(screen.getByText("trained → door")).toBeInTheDocument();
  });

  it("never leaves the handle as the only label, even before the lookup resolves", () => {
    pending.data = [row({ source: "bundle", source_ref: "bundle-9999-0000" })];
    renderPending();

    expect(screen.getByText("Bundle")).toBeInTheDocument();
    expect(screen.getByText(/b_bund/)).toBeInTheDocument();
  });

  it("links a c_ handle to Change history", () => {
    pending.data = [row({ cascade_id: "cas-8841-aaaa" })];
    renderPending();

    const link = screen.getByRole("link", { name: /Change history/i });
    expect(link).toHaveAttribute("href", "/operations/cascades?cascade=cas-8841-aaaa");
  });

  // A grant made by hand has no edit behind it. It used to carry a bare dash
  // under "Given by hand", which read as a value that failed to load.
  it("shows no handle, and no dash, for a change made by hand", () => {
    pending.data = [row({ source: "direct", source_ref: undefined, cascade_id: undefined })];
    renderPending();

    const cell = screen.getByText("Given by hand").parentElement as HTMLElement;
    expect(cell.textContent).toBe("Given by hand");
  });

  // A project nothing could name is words, not a dash before the role.
  it("reads Unknown project, not a dash, before the role of a change", () => {
    pending.data = [row()];
    renderPending();

    expect(screen.getByText(/Unknown project/)).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/give —/);
  });
});

/**
 * A connected system's change has no project or roles. It used to read
 * "give /" and "Given by hand", which said nothing true.
 */
describe("Pending changes · a connected system's change", () => {
  const apply = (overrides: Partial<PendingRow> = {}) =>
    row({
      op_type: "apply",
      project_id: "",
      role_keys: [],
      source: "direct",
      source_ref: undefined,
      cascade_id: undefined,
      target: "truenas",
      ...overrides,
    });

  it("says which system and why, and that the check queued it", () => {
    pending.data = [apply({ initiated_by: "system:reconcile", reason: "Joins Staff." })];
    renderPending();

    expect(screen.getByText(/TrueNAS account:/)).toBeInTheDocument();
    expect(screen.getByText(/Joins Staff\./)).toBeInTheDocument();
    expect(screen.getByText("Check for differences")).toBeInTheDocument();
    expect(screen.queryByText("Given by hand")).not.toBeInTheDocument();
    expect(screen.queryByText(/^give$/)).not.toBeInTheDocument();
  });

  it("says what every such row does when it was queued before the reason was kept", () => {
    pending.data = [apply({ initiated_by: "system:reconcile" })];
    renderPending();

    expect(screen.getByText(/brought in line with their roles/)).toBeInTheDocument();
  });
});

/** route-pending.dc.html: search, a status on every row, a folding band for one edit. */
describe("Pending changes · board layout", () => {
  it("keeps Send enabled while there are changes to send, counting all of them", () => {
    pending.data = [row({ id: "o1" }), row({ id: "o2", cascade_id: "c2" })];
    renderPending();
    expect(screen.getByRole("button", { name: "Send 2 changes" })).toBeEnabled();
  });

  it("names no count until the list has been read", () => {
    load.loading = true;
    renderPending();
    expect(screen.getByRole("button", { name: "Send changes" })).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /Search pending changes/ })).not.toBeInTheDocument();
  });

  it("has four columns, and the extra reason column is gone", () => {
    pending.data = [row()];
    renderPending();
    for (const heading of ["Who", "Change", "Caused by", "Waiting"]) {
      expect(screen.getByText(heading)).toBeInTheDocument();
    }
  });

  it("marks a role that always waits, and says a revocation also sends on its own", () => {
    pending.data = [
      row({ id: "o1", role_keys: ["supervisor"], wait_reason: "role_waits", cascade_id: "c1" }),
      row({ id: "o2", op_type: "revoke", cascade_id: "c2" }),
    ];
    renderPending();
    expect(screen.getByRole("img", { name: "Waits for you" })).toBeInTheDocument();
    expect(screen.getByText(/also sends on its own within a few minutes/)).toBeInTheDocument();
  });

  it("shows a failed change's status in its own row with what the system said", () => {
    pending.data = [row({ status: "failed", last_error: "no such role" })];
    renderPending();
    expect(screen.getByText(/Failed, and Syndra will try again: no such role/)).toBeInTheDocument();
  });

  it("says a batch's shared reason once, in its band, not on every row", () => {
    const waits = { role_keys: ["supervisor"], wait_reason: "role_waits" };
    pending.data = [
      row({ id: "o1", user_id: "u1", ...waits }),
      row({ id: "o2", user_id: "u2", ...waits }),
    ];
    renderPending();
    const said = screen.getAllByText(/Supervisor always waits for you/);
    expect(said).toHaveLength(1);
    expect(said[0]).toHaveTextContent(/These 2 changes come from one edit/);
  });

  it("folds a batch to its band and opens it again", () => {
    pending.data = [
      row({ id: "o1", role_keys: ["alpha"] }),
      row({ id: "o2", role_keys: ["beta"] }),
    ];
    renderPending();
    const fold = screen.getByRole("button", { name: "Fold these 2 changes" });
    expect(fold).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(fold);
    expect(screen.queryByText("alpha")).not.toBeInTheDocument();
    expect(screen.getByText(/These 2 changes come from one edit/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open these 2 changes" }));
    expect(screen.getByText("alpha")).toBeInTheDocument();
  });

  it("gives a lone change no fold", () => {
    pending.data = [row()];
    renderPending();
    expect(screen.queryByRole("button", { name: /these \d changes/ })).not.toBeInTheDocument();
  });

  describe("search", () => {
    beforeEach(() => {
      names.users = { u1: "Lena Brandt", u2: "Omar Haddad" };
      pending.data = [
        row({ id: "o1", user_id: "u1", role_keys: ["booking"], cascade_id: "c1" }),
        row({ id: "o2", user_id: "u2", role_keys: ["welder"], cascade_id: "c2" }),
      ];
    });
    const type = (value: string) =>
      fireEvent.change(screen.getByRole("textbox", { name: /Search pending changes/ }), {
        target: { value },
      });

    it("narrows by person, and says sending still sends all of them", () => {
      renderPending();
      type("omar");
      expect(screen.queryByText("Lena Brandt")).not.toBeInTheDocument();
      expect(screen.getByText("Omar Haddad")).toBeInTheDocument();
      expect(screen.getByText(/1 of the 2 changes match\. Sending sends all 2, in order\./)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Send 2 changes" })).toBeEnabled();
    });

    it("keeps a narrowed batch's band, counting the whole edit and saying how much is shown", () => {
      pending.data = [
        row({ id: "o1", user_id: "u1", role_keys: ["booking"] }),
        row({ id: "o2", user_id: "u2", role_keys: ["welder"] }),
        row({ id: "o3", user_id: "u2", role_keys: ["laser"] }),
      ];
      renderPending();
      type("omar");
      expect(screen.getByText(/These 3 changes come from one edit/)).toBeInTheDocument();
      expect(screen.getByText(/They are sent together or not at all\. 2 of 3 shown\./)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Fold these 3 changes" })).toBeInTheDocument();
    });

    it("keeps the band even when one row of the batch is all that matches", () => {
      pending.data = [
        row({ id: "o1", user_id: "u1", role_keys: ["booking"] }),
        row({ id: "o2", user_id: "u2", role_keys: ["welder"] }),
      ];
      renderPending();
      type("omar");
      expect(screen.getByText(/These 2 changes come from one edit/)).toBeInTheDocument();
      expect(screen.getByText(/1 of 2 shown\./)).toBeInTheDocument();
    });

    it("narrows by role", () => {
      renderPending();
      type("booking");
      expect(screen.getByText("Lena Brandt")).toBeInTheDocument();
      expect(screen.queryByText("Omar Haddad")).not.toBeInTheDocument();
    });

    it("says so when nothing matches, rather than claiming nothing is waiting", () => {
      renderPending();
      type("zzz");
      expect(screen.getByText(/No pending change matches/)).toBeInTheDocument();
      expect(screen.queryByText("Nothing is waiting for you to send.")).not.toBeInTheDocument();
    });

    it("says they are sent in order when nothing is searched", () => {
      renderPending();
      expect(screen.getByText("They are sent in this order.")).toBeInTheDocument();
    });
  });

  it("shows the resolved empty state with no search, and Send off", () => {
    renderPending();
    expect(screen.getByText("Nothing is waiting for you to send.")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /Search pending changes/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Send / })).toBeDisabled();
  });
});
