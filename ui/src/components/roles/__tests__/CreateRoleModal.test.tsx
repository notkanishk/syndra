// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import CreateRoleModal from "@/components/roles/CreateRoleModal";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { makeProxyFetch, respondWith } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastInfo: vi.fn(),
}));

const PROJECT_ID = "33333333-3333-4333-8333-333333333333";

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;
  proxy.register("GET", /\/api\/proxy\/roles(\?|$)/, () => [
    {
      project_id: PROJECT_ID,
      project_name: "Lab Ops",
      role_key: "mentor",
      display_name: "Mentor",
      description: "",
      bundle_count: 0,
      rule_count: 0,
      assigned_user_count: 0,
      is_unused: true,
      source: "mkauth",
      display_label: "Lab Ops: Mentor",
    },
  ]);
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderModal() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  const onClose = vi.fn();
  const onCreated = vi.fn();
  const utils = render(
    <QueryClientProvider client={client}>
      <NameResolverProvider>
        <CreateRoleModal open onClose={onClose} onCreated={onCreated} />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
  return { ...utils, onClose, onCreated };
}

describe("CreateRoleModal (Stage 4)", () => {
  it("derives a slug-cased role_key from the display name and submits the role", async () => {
    proxy.register("POST", /\/api\/proxy\/roles(\?|$)/, ({ body }) => ({
      id: "r-new",
      project_id: (body as { project_id?: string } | undefined)?.project_id,
      role_key: (body as { role_key?: string } | undefined)?.role_key,
      display_name: (body as { display_name?: string } | undefined)?.display_name,
      description: "",
    }));

    const { onCreated } = renderModal();

    // Wait for catalog so the project select is populated.
    await waitFor(() => {
      const select = screen.getByLabelText(/Project/) as HTMLSelectElement;
      expect(select.value).toBe(PROJECT_ID);
    });

    fireEvent.change(screen.getByLabelText(/Display name/i), {
      target: { value: "Workshop Mentor" },
    });

    const keyField = screen.getByLabelText(/Role key/i) as HTMLInputElement;
    expect(keyField.value).toBe("workshop_mentor");

    fireEvent.click(screen.getByRole("button", { name: /Create role/i }));

    await waitFor(() => {
      const calls = proxy.calls.filter((c) => c.method === "POST" && c.url.includes("/roles"));
      expect(calls.length).toBe(1);
      const body = calls[0].body as Record<string, unknown>;
      expect(body.project_id).toBe(PROJECT_ID);
      expect(body.role_key).toBe("workshop_mentor");
      expect(body.display_name).toBe("Workshop Mentor");
    });
    await waitFor(() => expect(onCreated).toHaveBeenCalled());
  });

  it("surfaces a 409 conflict inline without calling onCreated", async () => {
    proxy.register("POST", /\/api\/proxy\/roles(\?|$)/, () =>
      respondWith(409, { error: "CONFLICT", message: "duplicate role" }),
    );

    const { onCreated, onClose } = renderModal();

    await waitFor(() => {
      const select = screen.getByLabelText(/Project/) as HTMLSelectElement;
      expect(select.value).toBe(PROJECT_ID);
    });
    fireEvent.change(screen.getByLabelText(/Display name/i), {
      target: { value: "Mentor" }, // collides with seeded "mentor"
    });

    fireEvent.click(screen.getByRole("button", { name: /Create role/i }));

    await screen.findByText(/already exists in this project/i);
    expect(onCreated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
