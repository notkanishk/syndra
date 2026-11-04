// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AppsPage from "@/app/applications/page";

const state = vi.hoisted(() => ({ apps: [] as unknown[], loading: false }));

vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({
    data: state.loading ? undefined : state.apps,
    isLoading: state.loading,
    error: null,
    refetch: () => {},
  }),
}));
vi.mock("@/lib/queries/useClaimShape", () => ({
  useClaimTemplates: () => ({
    data: [
      { id: "netbird", name: "NetBird" },
      { id: "custom", name: "Custom" },
    ],
  }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: ({ id }: { id: string }) => <span>{id === "p1" ? "Laser Lab" : "Studio"}</span>,
}));

type Line = { source: string; keys: string[]; lands: string; template_id?: string; template_changed?: boolean; copied_from_application_id?: string; reaches?: boolean };

function app(id: string, name: string, project: string, claims: Line | undefined, people: number) {
  return {
    application: { id, name, project_id: project, description: "", consumer: "oidc", claim_name: "", format_type: "" },
    claims,
    consumed_roles: [],
    assigned_user_count: people,
    observed_user_count: people,
    confirmed_user_count: people,
  };
}

const headings = () => {
  const row = document.querySelector("[class*='tracking-[0.09em]']") as HTMLElement;
  return Array.from(row.children).map((cell) => (cell.textContent ?? "").replace(/[↑↓]/g, "").trim());
};
const order = () =>
  screen
    .getAllByRole("link")
    .map((a) => ["Bookings", "Badge reader", "Kiosk"].find((n) => a.textContent?.includes(n)));

beforeEach(() => {
  state.loading = false;
  state.apps = [
    app("kiosk", "Kiosk", "p2", { source: "builtin", keys: ["roles"], lands: "both" }, 9),
    app("bookings", "Bookings", "p1", { source: "app", keys: ["roles", "is_admin", "bundles"], lands: "both", template_id: "custom", template_changed: true }, 41),
    app("badge", "Badge reader", "p1", { source: "app", keys: ["groups"], lands: "access_token", template_id: "netbird", reaches: false }, 22),
  ];
});

// A-14 §7: the Format column becomes Claims, the app's real key and where it
// lands; the sub-line names where the shape comes from.
describe("the apps index, as drawn", () => {
  it("reads App, Reads roles from, Claims, People, with no Kind column", () => {
    render(<AppsPage />);
    expect(headings()).toEqual(["App", "Reads roles from", "Claims", "People"]);
  });

  it("names where each app's shape comes from under its name", () => {
    render(<AppsPage />);
    expect(screen.getByRole("link", { name: /Kiosk/ }).children[0].textContent).toBe("KioskStudio’s default");
    expect(screen.getByRole("link", { name: /Bookings/ }).children[0].textContent).toBe("BookingsCustom, changed");
    expect(screen.getByRole("link", { name: /Badge reader/ }).children[0].textContent).toBe("Badge readerNetBird template");
  });

  it("shows the real key and where it lands, in the board's order: app, project, claims, people", () => {
    render(<AppsPage />);
    const cells = Array.from(screen.getByRole("link", { name: /Bookings/ }).children).map((c) => c.textContent ?? "");
    expect(cells[1]).toBe("Laser Lab");
    expect(cells[2]).toBe("roles and 2 more");
    expect(cells[3]).toMatch(/^41/);
  });

  it("marks an app whose claims don't reach where it reads, in amber, and says so once above", () => {
    render(<AppsPage />);
    const cell = screen.getByRole("link", { name: /Badge reader/ }).children[2];
    expect(cell.textContent).toBe("groupsDoesn’t reach it");
    expect(cell.querySelector(".text-warn-text")).not.toBeNull();
    expect(screen.getByText("Badge reader doesn't get its claims.")).toBeInTheDocument();
    // An app never read says nothing rather than guess.
    expect(screen.getByRole("link", { name: /Kiosk/ }).children[2].textContent).toBe("roles");
  });

  it("says where claims land only when it is not the usual, and the protocol only when it is not OIDC", () => {
    state.apps = [
      app("a", "Bookings", "p1", { source: "app", keys: ["groups"], lands: "access_token" }, 1),
      { ...app("b", "Kiosk", "p2", { source: "builtin", keys: ["roles"], lands: "both" }, 1), application: { id: "b", name: "Kiosk", project_id: "p2", description: "", consumer: "saml", claim_name: "", format_type: "" } },
    ];
    render(<AppsPage />);
    expect(screen.getByRole("link", { name: /Bookings/ }).children[2].textContent).toBe("groups · access token");
    expect(screen.getByRole("link", { name: /Bookings/ }).children[0].textContent).not.toMatch(/OIDC/);
    expect(screen.getByRole("link", { name: /Kiosk/ }).children[0].textContent).toBe("KioskSAML · Studio’s default");
  });

  it("sorts on App A to Z at first, and on People when asked", () => {
    render(<AppsPage />);
    expect(screen.getByRole("button", { name: "Sort by app" })).toHaveAttribute("data-sort", "asc");
    expect(order()).toEqual(["Badge reader", "Bookings", "Kiosk"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by people" }));
    expect(order()).toEqual(["Kiosk", "Badge reader", "Bookings"]);
  });

  // No read is not a stale one: the freshness line waits for an answer.
  it("states how fresh the read is only once there is one", () => {
    state.loading = true;
    const { unmount } = render(<AppsPage />);
    expect(screen.queryByText(/What Zitadel confirmed/)).toBeNull();
    unmount();
    state.loading = false;
    render(<AppsPage />);
    expect(screen.getByText(/What Zitadel confirmed/)).toBeTruthy();
  });

  it("never cuts the name, the claim key or the project", () => {
    render(<AppsPage />);
    const row = screen.getByRole("link", { name: /Kiosk/ });
    expect(row.innerHTML).not.toMatch(/truncate/);
  });
});
