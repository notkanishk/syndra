// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AppsPage from "@/app/applications/page";

const state = vi.hoisted(() => ({ apps: [] as unknown[] }));

vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({ data: state.apps, isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: ({ id }: { id: string }) => <span>{id === "p1" ? "Laser Lab" : "Studio"}</span>,
}));

function app(id: string, name: string, project: string, format: string, people: number, claim = `${id}.roles`) {
  return {
    application: {
      id,
      name,
      project_id: project,
      description: "",
      consumer: "oidc",
      claim_name: claim,
      format_type: format,
    },
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
  state.apps = [
    app("kiosk", "Kiosk", "p2", "csv", 9),
    app("bookings", "Bookings", "p1", "array", 41),
    app("badge", "Badge reader", "p1", "csv", 22),
  ];
});

// The board (route-applications): App with "OIDC · claim key" under the name,
// Reads roles from, Format, People. Kind is the sub-line, not a column.
describe("the apps index, as drawn", () => {
  it("reads App, Reads roles from, Format, People, with no Kind column", () => {
    render(<AppsPage />);
    expect(headings()).toEqual(["App", "Reads roles from", "Format", "People"]);
  });

  it("puts the protocol and the claim key under the app's name", () => {
    render(<AppsPage />);
    const row = screen.getByRole("link", { name: /Kiosk/ });
    expect(row.children[0].textContent).toBe("KioskOIDC · kiosk.roles");
  });

  it("keeps the cells in the board's order: app, project, format, people", () => {
    render(<AppsPage />);
    const cells = Array.from(screen.getByRole("link", { name: /Kiosk/ }).children).map((c) => c.textContent ?? "");
    expect(cells[1]).toBe("Studio");
    expect(cells[2]).toBe("csv");
    expect(cells[3]).toMatch(/^9/);
  });

  it("flags a project read in two formats where it applies, in amber", () => {
    render(<AppsPage />);
    // Bookings (array) and Badge reader (csv) both read Laser Lab; Kiosk reads Studio alone.
    for (const name of [/Bookings/, /Badge reader/]) {
      const cell = screen.getByRole("link", { name }).children[2];
      expect(cell.textContent).toMatch(/mixed in Laser Lab$/);
      expect(cell.className).toMatch(/text-warn-text/);
    }
    const kiosk = screen.getByRole("link", { name: /Kiosk/ }).children[2];
    expect(kiosk.textContent).toBe("csv");
    expect(kiosk.className).not.toMatch(/text-warn-text/);
  });

  it("sorts on App A to Z at first, and on People when asked", () => {
    render(<AppsPage />);
    expect(screen.getByRole("button", { name: "Sort by app" })).toHaveAttribute("data-sort", "asc");
    expect(order()).toEqual(["Badge reader", "Bookings", "Kiosk"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by people" }));
    expect(order()).toEqual(["Kiosk", "Badge reader", "Bookings"]);
  });

  it("never cuts the name, the claim key or the project", () => {
    render(<AppsPage />);
    const row = screen.getByRole("link", { name: /Kiosk/ });
    expect(row.innerHTML).not.toMatch(/truncate/);
  });
});
