// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AppTokenScreen } from "@/components/apps/AppTokenScreen";

const view = vi.hoisted(() => ({ advanced: false }));

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
vi.mock("@/components/apps/TokenFormatEditor", () => ({ TokenFormatEditor: () => null }));
vi.mock("@/components/apps/TokenPreview", () => ({ TokenPreview: () => null }));
vi.mock("@/components/names", () => ({ ProjectName: () => <span>Laser Lab</span> }));

beforeEach(() => {
  view.advanced = false;
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
});
