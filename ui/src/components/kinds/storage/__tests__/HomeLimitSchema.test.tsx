// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LimitDialog } from "@/components/kinds/storage/HomeLimit";
import { fieldSchemaMock } from "@/lib/__fixtures__/field-schema-mock";

// Save while the system's fields are unknown says so beside the button, before any other reason.

vi.mock("@/lib/queries/useFieldSchema", async () => ({
  ...(await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock,
  useFieldSchemaState: () => ({ schema: undefined, loading: true, error: null, refetch: () => {} }),
  useMappingFields: () => ({}),
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveRole: () => ({ value: { display_name: "Fabrication lead" }, resolved: true }) }),
}));

describe("Save while the fields are unknown", () => {
  it("says Syndra is still reading them", () => {
    expect(fieldSchemaMock.useSystemNames()("truenas")).toBe("TrueNAS");
    const client = new QueryClient();
    render(
      <QueryClientProvider client={client}>
        <LimitDialog target="truenas" name="Charlotte Flair" home={{ subject_id: "u", username: "c", gives_home: true }} onClose={() => {}} />
      </QueryClientProvider>,
    );
    expect(screen.getByText("Syndra is still reading TrueNAS's fields. This works once it has.")).toBeTruthy();
  });
});
