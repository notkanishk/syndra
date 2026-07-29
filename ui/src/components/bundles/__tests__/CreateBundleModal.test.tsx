// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import CreateBundleModal from "@/components/bundles/CreateBundleModal";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { makeProxyFetch } from "@/test-utils/proxyFetch";

vi.mock("@/lib/toast", () => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));

let proxy: ReturnType<typeof makeProxyFetch>;

beforeEach(() => {
  proxy = makeProxyFetch();
  global.fetch = proxy.fetchImpl;
  proxy.register("POST", /\/api\/proxy\/bundles(\?|$)/, ({ body }) => ({
    id: "b-new",
    name: (body as { name?: string } | undefined)?.name ?? "",
  }));
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
        <CreateBundleModal open onClose={onClose} onCreated={onCreated} />
      </NameResolverProvider>
    </QueryClientProvider>,
  );
  return { ...utils, onClose, onCreated };
}

describe("CreateBundleModal (Stage 4)", () => {
  it("disables Create until a name is entered, then POSTs the bundle on submit", async () => {
    const { onClose, onCreated } = renderModal();
    const submit = screen.getByRole("button", { name: /Create bundle/i });
    expect(submit).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/Bundle name/i), {
      target: { value: "Workshop Mentors" },
    });
    fireEvent.change(screen.getByLabelText(/Description/i), {
      target: { value: "Hands-on workshop access" },
    });
    expect(submit).not.toBeDisabled();

    fireEvent.click(submit);

    await waitFor(() => {
      expect(
        proxy.calls.some(
          (c) =>
            c.method === "POST" &&
            c.url.includes("/bundles") &&
            !c.url.includes("/roles") &&
            !c.url.includes("/impact"),
        ),
      ).toBe(true);
    });
    await waitFor(() => {
      expect(onCreated).toHaveBeenCalledWith("b-new");
      expect(onClose).toHaveBeenCalled();
    });
  });

  it("submits confirmation_mode defaulted from the global default (Task 22)", async () => {
    proxy.register("GET", /\/api\/proxy\/config\/confirmation-mode-default(\?|$)/, () => ({
      mode: "manual",
    }));

    renderModal();
    fireEvent.change(screen.getByLabelText(/Bundle name/i), {
      target: { value: "Ops Team" },
    });

    // Wait for the global-default query to resolve before submitting so the
    // select reflects the fetched value rather than the pre-fetch "auto" fallback.
    await waitFor(() => {
      expect(screen.getByLabelText(/Confirmation mode/i)).toHaveValue("manual");
    });

    fireEvent.click(screen.getByRole("button", { name: /Create bundle/i }));

    await waitFor(() => {
      const call = proxy.calls.find(
        (c) => c.method === "POST" && c.url.includes("/bundles") && !c.url.includes("/roles"),
      );
      expect(call).toBeDefined();
      expect(call?.body).toMatchObject({ confirmation_mode: "manual" });
    });
  });
});
