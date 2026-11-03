// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AppTokenScreen } from "@/components/apps/AppTokenScreen";

const view = vi.hoisted(() => ({ advanced: false, signIns: undefined as unknown }));

vi.mock("@/lib/ui-view", () => ({ useIsAdvanced: () => view.advanced }));
vi.mock("@/lib/page-crumb", () => ({ useCrumb: () => {} }));
vi.mock("@/lib/queries/useApplications", () => ({
  useApplications: () => ({
    data: [{ application: { id: "367263700000000001", name: "Slicer", project_id: "p1" } }],
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
}));
vi.mock("@/lib/queries/useClaimShape", () => ({ useClaimShape: () => ({ data: undefined }) }));
vi.mock("@/lib/queries/useSignIns", () => ({
  useAppSignIns: () => ({ data: view.signIns }),
  useLookUpSignIns: () => ({ mutate: () => {}, isPending: false }),
}));
vi.mock("@/components/apps/TokenFormatEditor", () => ({ TokenFormatEditor: () => null }));
vi.mock("@/components/apps/TokenPreview", () => ({ TokenPreview: () => null }));
vi.mock("@/components/names", () => ({ ProjectName: () => <span>Laser Lab</span> }));

beforeEach(() => {
  view.advanced = false;
  view.signIns = undefined;
});

// "id 3672637…" stood in the meta line in Basic, beside the project the app uses.
describe("an app's header", () => {
  it("names the app and its project, with no id in Basic", () => {
    render(<AppTokenScreen applicationId="367263700000000001" />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Slicer");
    expect(screen.getByText(/uses roles from/)).toHaveTextContent("Laser Lab");
    expect(screen.queryByText("367263700000000001")).not.toBeInTheDocument();
  });

  it("keeps the id in Advanced", () => {
    view.advanced = true;
    render(<AppTokenScreen applicationId="367263700000000001" />);
    expect(screen.getByText("367263700000000001")).toBeInTheDocument();
  });

  // The board (route-applications-id): "Apps" over the title, and the project an
  // app reads from is a link to it.
  it("carries the Apps eyebrow over the title", () => {
    render(<AppTokenScreen applicationId="367263700000000001" />);
    const title = screen.getByRole("heading", { level: 1 });
    expect(title.previousElementSibling).toHaveTextContent("Apps");
  });

  it("links the project it reads roles from", () => {
    render(<AppTokenScreen applicationId="367263700000000001" />);
    const link = screen.getByRole("link", { name: /Laser Lab/ });
    expect(link).toHaveAttribute("href", "/projects/p1");
  });
});

// A-10 §7: one line in the meta, Syndra's own record; the strip names nobody.
describe("an app's sign-ins", () => {
  it("says nothing while sign-ins aren't arriving", () => {
    view.signIns = { receiving: false };
    render(<AppTokenScreen applicationId="367263700000000001" />);
    expect(screen.queryByText(/in 7 days|No sign-in yet/)).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Sign-ins" })).not.toBeInTheDocument();
  });

  it("puts people in 7 days and the last sign-in in the meta, and draws the 30 days", () => {
    const last = new Date(Date.now() - 10 * 60 * 1000).toISOString();
    const today = new Date();
    const day = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`;
    view.signIns = { receiving: true, people_7d: 9, people_30d: 23, last_at: last, days: [{ day, count: 142 }] };
    render(<AppTokenScreen applicationId="367263700000000001" />);
    expect(screen.getByText("9 people in 7 days · last sign-in 10 min ago")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Sign-ins" })).toBeInTheDocument();
    expect(screen.getByText("142 sign-ins in 30 days, by 23 people.")).toBeInTheDocument();
    // "9 people" doesn't link: People can't filter by sign-in.
    expect(screen.queryByRole("link", { name: /9 people/ })).not.toBeInTheDocument();
  });

  it("says no sign-in yet when there has never been one", () => {
    view.signIns = { receiving: true, people_7d: 0, people_30d: 0, last_at: null, days: [] };
    render(<AppTokenScreen applicationId="367263700000000001" />);
    expect(screen.getByText("No sign-in yet")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Sign-ins" })).not.toBeInTheDocument();
  });
});
