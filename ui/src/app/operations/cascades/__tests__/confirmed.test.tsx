// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import ChangeHistoryPage from "@/app/operations/cascades/page";
import type { CascadeGroupRow } from "@/lib/queries/useConfirmationMode";

/**
 * `status=applied` is Syndra's own record of having sent a write. It is not a
 * Zitadel-side fact until `confirmed_at` says a post-write read of Zitadel
 * agreed — collapsing the two used to render "3 sent … all of them went
 * through" for cascades nobody had verified landed anywhere.
 */

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
  BundleName: ({ id }: { id: string }) => <span>{id}</span>,
  RoleRef: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
  RoleInPlace: ({ roleKey }: { roleKey: string }) => <span>{roleKey}</span>,
}));

const rules = vi.hoisted(() => ({ data: [] as { id: string; source_project: string; source_role: string; target_project: string; target_role: string }[] }));
vi.mock("@/lib/queries/useMappingRules", () => ({
  useMappingRules: () => ({ data: rules.data }),
}));

let groups: CascadeGroupRow[] = [];

vi.mock("@/lib/queries/useConfirmationMode", () => ({
  useCascadeGroups: () => ({ data: groups, isLoading: false, error: null, refetch: () => {} }),
}));

function group(overrides: Partial<CascadeGroupRow>): CascadeGroupRow {
  return {
    cascade_id: "c1",
    source: "bundle",
    source_ref: "b1",
    applied: 0,
    waiting: 0,
    failed: 0,
    confirmed: 0,
    user_ids: ["u1"],
    writes: [
      {
        id: "w1",
        op_type: "add",
        user_id: "u1",
        project_id: "p1",
        role_keys: ["maker"],
        source: "bundle",
        status: "applied",
      },
    ],
    started_at: "2026-09-01T00:00:00Z",
    ...overrides,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ChangeHistoryPage />
    </QueryClientProvider>,
  );
}

describe("Change history: sent vs. confirmed", () => {
  it("says confirmed by Zitadel when every applied write has been read back", () => {
    groups = [group({ applied: 3, confirmed: 3 })];
    renderPage();

    expect(screen.getByText("3 confirmed by Zitadel")).toBeInTheDocument();
    expect(screen.getByText(/all of them went through/)).toBeInTheDocument();
  });

  it("distinguishes sent from confirmed when read-back is still pending", () => {
    groups = [group({ applied: 3, confirmed: 1 })];
    renderPage();

    expect(screen.getByText("3 sent, 1 confirmed")).toBeInTheDocument();
    expect(
      screen.getByText(/Sent; Zitadel has not yet confirmed 2\./),
    ).toBeInTheDocument();
    expect(screen.queryByText(/all of them went through/)).not.toBeInTheDocument();
  });

  it("titles a cascade by what was edited, with no c_ handle, and links Pending changes only while changes wait", () => {
    groups = [group({ cascade_id: "cas-8841-aaaa", waiting: 2 }), group({ cascade_id: "cas-9999-bbbb", applied: 1, confirmed: 1 })];
    renderPage();

    expect(screen.queryByText(/c_[0-9a-f]{4}/)).not.toBeInTheDocument();
    expect(screen.getAllByText("A bundle edit reached one person")).toHaveLength(2);
    expect(screen.getAllByRole("link", { name: /Pending changes/ })).toHaveLength(1);
  });

  // The bulk dialog asks for a reason; for a bundle change it shows here.
  it("says the operator's reason when the change came with one, and nothing when it didn't", () => {
    groups = [group({ cascade_id: "with", reason: "term start", applied: 1, confirmed: 1 }), group({ cascade_id: "without" })];
    renderPage();

    expect(screen.getAllByText("“term start”")).toHaveLength(1);
  });

  it("'and N more' is a violet link to the one edit, which lists every write (A-20 P17)", () => {
    const writes = Array.from({ length: 9 }, (_, n) => ({
      id: `w${n}`,
      op_type: "add" as const,
      user_id: `user-${n}`,
      project_id: "p1",
      role_keys: ["maker"],
      source: "bundle" as const,
      status: "applied" as const,
    }));
    groups = [group({ cascade_id: "cas 1", writes })];
    renderPage();

    expect(screen.getByRole("link", { name: "and 3 more" })).toHaveAttribute("href", "/operations/cascades?cascade=cas%201");
    expect(screen.queryByText("user-8")).not.toBeInTheDocument();
  });

  // Task 1.6: a bundle's change carries the rows of the rules it set off. The
  // entry is the bundle's, whichever write comes first; the rule's write names
  // the role that set it off.
  it("names a bundle's change by the bundle, and a rule's write by the role that set it off", () => {
    rules.data = [{ id: "r1", source_project: "p1", source_role: "trained", target_project: "p1", target_role: "advanced" }];
    const write = (id: string, role: string, source: string, ref: string) => ({
      id, op_type: "add", user_id: "u1", project_id: "p1", role_keys: [role], source, source_ref: ref, status: "pending",
    });
    groups = [group({ waiting: 2, writes: [write("w1", "advanced", "rule", "r1"), write("w2", "trained", "bundle", "b1")] })];
    renderPage();

    expect(screen.getByText("A bundle edit reached one person")).toBeInTheDocument();
    expect(screen.getByText(/^from/)).toHaveTextContent("from b1");
    expect(screen.getByText(/^by the automatic rule from/)).toHaveTextContent("by the automatic rule from trained");
    expect(screen.queryByText(/An automatic rule applied/)).toBeNull();
    rules.data = [];
  });

  // Task 1.6a: a rule's own edit in a chain. The edited rule's write says
  // nothing more (the entry is its own); the write one step further down names
  // the role kept on it, not the rule's first role now; a retired rule's write
  // is "by an automatic rule".
  it("names each hop of a rule's edit by the role kept on its write", () => {
    rules.data = [
      { id: "r1", source_project: "p1", source_role: "member", target_project: "p1", target_role: "trained" },
      // r2 has since been edited to start from visitor.
      { id: "r2", source_project: "p1", source_role: "visitor", target_project: "p1", target_role: "advanced" },
    ];
    const write = (id: string, role: string, ref: string, from?: string) => ({
      id, op_type: "add", user_id: "u1", project_id: "p1", role_keys: [role], source: "rule", source_ref: ref, status: "pending",
      ...(from ? { rule_from: { project_id: "p1", role_key: from } } : {}),
    });
    groups = [
      group({
        source: "rule",
        source_ref: "r1",
        waiting: 3,
        writes: [write("w1", "trained", "r1", "member"), write("w2", "advanced", "r2", "trained"), write("w3", "lead", "r-gone", "advanced")],
      }),
    ];
    renderPage();

    const named = screen.getAllByText(/^by (the|an) automatic rule/).map((el) => el.textContent);
    expect(named).toEqual(["by the automatic rule from trained", "by an automatic rule"]);
    expect(screen.queryByText(/from visitor/)).toBeNull();
    rules.data = [];
  });
});
