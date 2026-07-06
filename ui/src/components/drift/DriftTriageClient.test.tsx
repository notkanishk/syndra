// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

import { DriftTriageClient } from "./DriftTriageClient";

const USER_ID = "11111111-1111-4111-8111-111111111111";
const PROJECT_ID = "22222222-2222-4222-8222-222222222222";

let proxy: ReturnType<typeof makeProxyFetch>;
let driftRows: ReturnType<typeof driftItem>[];

function driftItem(id: string) {
  return {
    id,
    user_id: USER_ID,
    project_id: PROJECT_ID,
    role_keys: ["mentor"],
    drift_type: "zitadel_only",
    detection_source: "webhook",
    detected_at: "2026-07-06T00:00:00Z",
  };
}

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;
  driftRows = [driftItem("d-1")];

  proxy.register("GET", /\/api\/proxy\/governance\/drift(\?|$)/, () => ({
    drift: driftRows,
  }));
  proxy.register("GET", /\/api\/proxy\/bundles\/b-no-role\/roles/, () => [
    { bundle_id: "b-no-role", zitadel_project_id: PROJECT_ID, zitadel_role_key: "other-role" },
  ]);
  proxy.register("GET", /\/api\/proxy\/bundles\/b-has-role\/roles/, () => [
    { bundle_id: "b-has-role", zitadel_project_id: PROJECT_ID, zitadel_role_key: "mentor" },
  ]);
  proxy.register("GET", /\/api\/proxy\/bundles(\?|$)/, () => [
    { id: "b-no-role", name: "No Role Bundle" },
    { id: "b-has-role", name: "Has Role Bundle" },
  ]);
  proxy.register("POST", /\/api\/proxy\/governance\/drift\/d-1\/revoke/, () => ({
    status: "revoked",
    outbox_id: "o-1",
  }));
  proxy.register("POST", /\/api\/proxy\/governance\/drift\/d-1\/attribute/, () => ({
    status: "attributed",
  }));
  proxy.register("POST", /\/api\/proxy\/lookup/, () => ({
    users: {}, projects: {}, roles: {}, bundles: {},
  }));
  proxy.register("POST", /\/api\/proxy\/governance\/drift\/reconcile/, () => ({ status: "ok" }));
});

afterEach(() => {
  proxy.fetchImpl.mockClear?.();
});

function renderClient() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <DriftTriageClient />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
}

describe("DriftTriageClient", () => {
  it("renders drift rows from the list query", async () => {
    renderClient();
    await screen.findByText(/Drift items \(1\)/);
    expect(screen.getByText(/mentor/)).toBeInTheDocument();
  });

  it("does not highlight rows present on the initial load", async () => {
    renderClient();
    await screen.findByText(/Drift items \(1\)/);
    expect(document.querySelector('[data-new]')).not.toBeInTheDocument();
  });

  it("marks a row that appears after a refetch as new", async () => {
    renderClient();
    await screen.findByText(/Drift items \(1\)/);
    expect(document.querySelector('[data-new]')).not.toBeInTheDocument();

    driftRows = [driftItem("d-1"), driftItem("d-2")];
    fireEvent.click(screen.getByRole("button", { name: /reconcile now/i }));

    await screen.findByText(/Drift items \(2\)/);
    await waitFor(() => {
      expect(document.querySelector('[data-new]')).toBeInTheDocument();
    });
  });

  it("clicking Revoke calls the revoke mutation with the row id", async () => {
    renderClient();
    await screen.findByText(/Drift items \(1\)/);

    fireEvent.click(screen.getByRole("button", { name: /^Revoke$/ }));

    await waitFor(() => {
      expect(
        proxy.calls.some(
          (c) => c.method === "POST" && c.url.includes("/governance/drift/d-1/revoke"),
        ),
      ).toBe(true);
    });
  });

  it("attribute-to-bundle disables a bundle lacking the drift role", async () => {
    renderClient();
    await screen.findByText(/Drift items \(1\)/);

    fireEvent.click(screen.getByRole("button", { name: /^Attribute$/ }));
    fireEvent.change(screen.getByLabelText("Attribution source"), {
      target: { value: "bundle" },
    });

    const bundleSelect = await screen.findByLabelText("Bundle");
    await waitFor(() => {
      const noRoleOption = screen.getByRole("option", { name: /No Role Bundle/i });
      expect(noRoleOption).toBeDisabled();
    });
    const hasRoleOption = screen.getByRole("option", { name: /Has Role Bundle/i });
    expect(hasRoleOption).not.toBeDisabled();

    fireEvent.change(bundleSelect, { target: { value: "b-has-role" } });
    fireEvent.click(screen.getByRole("button", { name: /Confirm attribute/i }));

    await waitFor(() => {
      const call = proxy.calls.find(
        (c) => c.method === "POST" && c.url.includes("/governance/drift/d-1/attribute"),
      );
      expect(call).toBeDefined();
      expect(call?.body).toMatchObject({ source: "bundle", source_ref: "b-has-role" });
    });
  });
});
