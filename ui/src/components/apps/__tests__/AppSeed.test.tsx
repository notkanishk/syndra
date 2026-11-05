// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AppTokenScreen } from "@/components/apps/AppTokenScreen";
import { PageCrumbProvider, usePageCrumb } from "@/lib/page-crumb";

/** A-19 P1 for an app: the name, kind and project arrive with the row. */

vi.mock("@/lib/queries/useClaimShape", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useClaimShape")>()),
  useAppClaimShape: () => ({ data: undefined, isLoading: true, error: null, refetch: () => {} }),
}));

function Crumb() {
  return <p data-testid="crumb">{usePageCrumb().crumb ?? "none"}</p>;
}

function renderApp(seeded: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (seeded) {
    client.setQueryData(["applications"], [
      {
        application: { id: "a1", name: "Laser Booking", project_id: "p1", description: "", consumer: "oidc", claim_name: "", format_type: "" },
        consumed_roles: [],
        assigned_user_count: 0,
      },
    ]);
  }
  return render(
    <QueryClientProvider client={client}>
      <PageCrumbProvider>
        <Crumb />
        <AppTokenScreen applicationId="a1" />
      </PageCrumbProvider>
    </QueryClientProvider>,
  );
}

describe("an app opened from its row", () => {
  it("paints the name, its kind and the breadcrumb on the first frame", () => {
    renderApp(true);
    expect(screen.getByRole("heading", { name: "Laser Booking" })).toBeInTheDocument();
    expect(screen.getByText(/OIDC · roles from/)).toBeInTheDocument();
    expect(screen.getByTestId("crumb")).toHaveTextContent("Laser Booking");
  });

  it("with no row to seed from, shows no name until the read lands", () => {
    renderApp(false);
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
    expect(screen.getByTestId("crumb")).toHaveTextContent("none");
  });
});
