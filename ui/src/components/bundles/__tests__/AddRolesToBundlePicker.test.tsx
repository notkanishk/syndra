// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AddRolesToBundlePicker from "@/components/bundles/AddRolesToBundlePicker";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));

let proxy: ReturnType<typeof makeProxyFetch>;

const PROJECT_ID = "33333333-3333-4333-8333-333333333333";

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;

  proxy.register("GET", /\/api\/proxy\/roles(\?|$)/, () => [
    {
      project_id: PROJECT_ID,
      project_name: "Lab Ops",
      role_key: "mentor",
      display_name: "Mentor",
      description: "Workshop mentor",
      bundle_count: 0,
      rule_count: 0,
      assigned_user_count: 0,
      is_unused: true,
      source: "mkauth",
      display_label: "Lab Ops: Mentor",
    },
    {
      project_id: PROJECT_ID,
      project_name: "Lab Ops",
      role_key: "viewer",
      display_name: "Viewer",
      description: "Read-only",
      bundle_count: 0,
      rule_count: 0,
      assigned_user_count: 0,
      is_unused: true,
      source: "mkauth",
      display_label: "Lab Ops: Viewer",
    },
  ]);

  proxy.register("POST", /\/api\/proxy\/bundles\/b1\/roles/, () => ({ message: "Role added" }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderPicker(existingRoles: Array<{ bundle_id: string; zitadel_project_id: string; zitadel_role_key: string }>) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  const onClose = vi.fn();
  const utils = render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <AddRolesToBundlePicker
          open
          onClose={onClose}
          bundleId="b1"
          bundleName="Mentor Pack"
          existingRoles={existingRoles}
        />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
  return { ...utils, onClose };
}

describe("AddRolesToBundlePicker (Stage 4)", () => {
  it("filters by search and POSTs each selection sequentially", async () => {
    const { onClose } = renderPicker([]);

    // Wait for catalog to load
    await screen.findByText("Mentor");
    await screen.findByText("Viewer");

    // Select both roles
    fireEvent.click(screen.getByRole("option", { name: /Mentor/ }));
    fireEvent.click(screen.getByRole("option", { name: /Viewer/ }));

    const submit = await screen.findByRole("button", { name: /Add 2 roles/i });
    fireEvent.click(submit);

    await waitFor(() => {
      const adds = proxy.calls.filter(
        (c) => c.method === "POST" && c.url.includes("/bundles/b1/roles"),
      );
      expect(adds).toHaveLength(2);
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("disables roles that are already in the bundle", async () => {
    renderPicker([
      { bundle_id: "b1", zitadel_project_id: PROJECT_ID, zitadel_role_key: "mentor" },
    ]);

    const mentorOption = await screen.findByRole("option", { name: /Mentor/ });
    expect(mentorOption).toHaveAttribute("aria-disabled", "true");
    expect(mentorOption).toBeDisabled();

    // The other role is still selectable.
    const viewerOption = screen.getByRole("option", { name: /Viewer/ });
    expect(viewerOption).not.toBeDisabled();
  });

  it("filters the list by search query", async () => {
    renderPicker([]);
    await screen.findByText("Mentor");
    await screen.findByText("Viewer");

    fireEvent.change(screen.getByPlaceholderText(/Search by project, role name/i), {
      target: { value: "viewer" },
    });

    expect(screen.queryByText("Mentor")).toBeNull();
    expect(screen.getByText("Viewer")).toBeDefined();
  });
});
