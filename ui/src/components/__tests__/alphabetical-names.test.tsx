// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, renderHook, screen, waitFor, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

/**
 * "Alphabetical, always" (owner, 4 Nov 2026): every list or picker of names
 * is A to Z by its shown name, whatever order the backend sent. The names
 * below arrive shuffled, with case mixed the way Zitadel holds them.
 */
const SHUFFLED = ["Kanishk Singh", "Admin Makerspace", "aleister Black", "Disha R", "booker T"];
const SORTED = ["Admin Makerspace", "aleister Black", "booker T", "Disha R", "Kanishk Singh"];

const routes: Record<string, unknown> = {
  "/catalog": {
    users: [
      { id: "sa1", name: "netbird", email: "", title: "", service_account: true },
      ...SHUFFLED.map((name, i) => ({ id: `u${i}`, name, email: "", title: "" })),
      { id: "sa2", name: "claude code", email: "", title: "", service_account: true },
    ],
    projects: SHUFFLED.map((name, i) => ({
      id: `p${i}`,
      name,
      roles: [
        { key: "z", label: "Trainee" },
        { key: "a", label: "Keeper" },
      ],
    })),
  },
  "/projects": SHUFFLED.map((name, i) => ({
    project: { id: `p${i}`, name, roles: [{ key: "z", label: "trainee" }, { key: "a", label: "Keeper" }] },
  })),
  "/bundles": SHUFFLED.map((name, i) => ({ id: `b${i}`, name })),
  "/applications": SHUFFLED.map((name, i) => ({ application: { id: `a${i}`, name } })),
  "/roles": SHUFFLED.map((name, i) => ({ project_id: `p${i}`, project_name: "P", role_key: `k${i}`, display_name: name })),
  "/service-accounts": { accounts: SHUFFLED.map((name, i) => ({ id: `s${i}`, name })) },
  "/claim-templates": { templates: SHUFFLED.map((name, i) => ({ id: `t${i}`, name })) },
  // System ids whose shown label differs from the id's own order.
  "/targets": { targets: ["truenas-lab", "nas-annex", "backup"].map((target) => ({ target })) },
  "/projects/p1/roles/op/members": {
    members: SHUFFLED.map((name, i) => ({ user: { id: `u${i}`, name }, reasons: [] })),
    observed_only: [{ id: "o1", name: "zack" }, { id: "o2", name: "Alicia" }],
  },
};

vi.mock("@/lib/api-client", () => ({ request: vi.fn(async (path: string) => routes[path]) }));

import { ClaimPreview } from "@/components/apps/ClaimPreview";
import { RolePicker } from "@/components/bundles/RolePicker";
import type { CheckContext } from "@/components/apps/claim-words";
import { useApplications } from "@/lib/queries/useApplications";
import { useBundles } from "@/lib/queries/useBundles";
import { useCatalogProjects, useCatalogUsers } from "@/lib/queries/useCatalogUsers";
import { useClaimTemplates } from "@/lib/queries/useClaimShape";
import { useProjects } from "@/lib/queries/useProjects";
import { targetLabel } from "@/lib/nav";
import { useRoleMembers } from "@/lib/queries/useRoleMembers";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { useTargets } from "@/lib/queries/useTargets";
import { useServiceAccounts } from "@/lib/queries/useServiceAccounts";

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>;
}

async function read<T>(use: () => { data?: T }) {
  const { result } = renderHook(use, { wrapper });
  await waitFor(() => expect((result.current.data as unknown[] | undefined)?.length).toBeGreaterThan(0));
  return result.current.data as T;
}

describe("every source a picker reads is A to Z", () => {
  it("people, then service accounts, each A to Z (the claims Preview's picker)", async () => {
    const users = await read(() => useCatalogUsers());
    expect(users.map((u) => u.name)).toEqual([...SORTED, "claude code", "netbird"]);
  });

  it("projects, and each project's roles", async () => {
    const projects = await read(() => useProjects());
    expect(projects.map((p) => p.project.name)).toEqual(SORTED);
    expect(projects[0].project.roles.map((r) => r.label)).toEqual(["Keeper", "trainee"]);
    const catalog = await read(() => useCatalogProjects());
    expect(catalog.map((p) => p.name)).toEqual(SORTED);
    expect(catalog[0].roles.map((r) => r.label)).toEqual(["Keeper", "Trainee"]);
  });

  it("bundles, apps, roles, service accounts and templates", async () => {
    expect((await read(() => useBundles())).map((b) => b.name)).toEqual(SORTED);
    expect((await read(() => useApplications())).map((a) => a.application.name)).toEqual(SORTED);
    expect((await read(() => useGlobalRoleCatalog())).map((r) => r.display_name)).toEqual(SORTED);
    expect((await read(() => useServiceAccounts())).map((a) => a.name)).toEqual(SORTED);
    expect((await read(() => useClaimTemplates())).map((t) => t.name)).toEqual(SORTED);
  });

  it("systems by their shown label, and a role's holders", async () => {
    const labels = (await read(() => useTargets())).map((t) => targetLabel(t.target));
    expect(labels).toEqual([...labels].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" })));
    const { result } = renderHook(() => useRoleMembers("p1", "op"), { wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(result.current.data!.members.map((m) => m.user.name)).toEqual(SORTED);
    expect(result.current.data!.observed_only!.map((p) => p.name)).toEqual(["Alicia", "zack"]);
  });
});

describe("the shared pickers render A to Z from shuffled props", () => {
  it("Preview for: people A to Z, service accounts after them under their own label, the chosen one not moved", () => {
    const people = [
      { id: "sa1", name: "netbird", title: "", email: "", service_account: true },
      ...SHUFFLED.map((name, i) => ({ id: `u${i}`, name, title: "", email: "" })),
      { id: "sa2", name: "claude code", title: "", email: "", service_account: true },
    ];
    render(
      <ClaimPreview
        isLoading
        error={null}
        onRetry={() => {}}
        people={people}
        personId="u0"
        onPerson={() => {}}
        ctx={{ person: "Kanishk Singh" } as CheckContext}
      />,
    );
    const picker = screen.getByRole("combobox", { name: "Preview for" }) as HTMLSelectElement;
    expect(Array.from(picker.options, (o) => o.text)).toEqual([...SORTED, "claude code", "netbird"]);
    expect(picker.value).toBe("u0");
    const group = picker.querySelector("optgroup")!;
    expect(group.label).toBe("Service accounts");
    expect(within(group as unknown as HTMLElement).getAllByRole("option").map((o) => o.textContent)).toEqual(["claude code", "netbird"]);
  });

  it("the one-role picker: places A to Z, and roles A to Z inside each", () => {
    const places = SHUFFLED.map((name, i) => ({
      id: `p${i}`,
      name,
      roles: [
        { key: "z", name: "trainee", note: { kind: "none" as const } },
        { key: "a", name: "Keeper", note: { kind: "none" as const } },
      ],
    }));
    render(<RolePicker mode="one" audience="member" places={places} value={null} onChange={() => {}} cameFrom="p0" />);
    const folds = screen.getAllByRole("button").filter((b) => b.hasAttribute("aria-expanded"));
    expect(folds.map((f) => SORTED.find((n) => f.textContent?.includes(n)))).toEqual(SORTED);
    // The place a link named (Kanishk Singh) opens where it sits; it is not moved up.
    expect(folds[4]).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByRole("radio").map((r) => r.closest("label")?.textContent ?? r.getAttribute("aria-label"))).toEqual([
      expect.stringContaining("Keeper"),
      expect.stringContaining("trainee"),
    ]);
  });
});
