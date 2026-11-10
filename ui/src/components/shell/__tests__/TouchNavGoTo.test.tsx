// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TouchNav } from "@/components/shell/TouchNav";
import { NAME_CATALOG_KEY } from "@/lib/queries/useNameCatalog";

/** A-19 P4 on a phone: Go to is the existing sheet, with an Input at its top. */

const view = vi.hoisted(() => ({ audience: "advanced" as string }));

vi.mock("next/navigation", () => ({ usePathname: () => "/" }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ audience: view.audience, isOperator: true, view: view.audience, setView: () => {}, revealInAdvanced: () => {} }),
}));
vi.mock("next/link", () => ({
  default: ({ href, replace, children, ...rest }: React.ComponentProps<"a"> & { replace?: boolean }) => (
    <a href={href} data-replace={String(Boolean(replace))} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [] }) }));
vi.mock("@/lib/queries/useIndicators", () => ({ useIndicatorsRead: () => ({ data: undefined, isPlaceholderData: false }) }));

function openSheet() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(NAME_CATALOG_KEY, {
    users: [
      { id: "u2", name: "Primo Colon", email: "", title: "", team: "Fabrication" },
      { id: "u1", name: "Damian Priest", email: "", title: "", team: "" },
    ],
    projects: [],
    applications: [],
  });
  render(
    <QueryClientProvider client={client}>
      <TouchNav />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: /Go to|Home/ }));
  return screen.getByRole("dialog", { name: "Go to a page" });
}

beforeEach(() => {
  view.audience = "advanced";
});

describe("the phone's Go to sheet", () => {
  it("has the Input at the top, above the page list it keeps", () => {
    const sheet = openSheet();
    const box = within(sheet).getByRole("combobox");
    const bundles = within(sheet).getByRole("link", { name: /Bundles/ });
    expect(box.compareDocumentPosition(bundles) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(box).toHaveAttribute("aria-expanded", "false");
  });

  it("swaps the page list for matches once something is typed, A to Z, opening as a replace", () => {
    const sheet = openSheet();
    fireEvent.change(within(sheet).getByRole("combobox"), { target: { value: "pri" } });
    expect(within(sheet).queryByRole("link", { name: /Bundles/ })).toBeNull();
    const options = within(sheet).getAllByRole("option");
    expect(options.map((o) => o.getAttribute("href"))).toEqual(["/users/u1", "/users/u2"]);
    expect(options[0]).toHaveAttribute("data-replace", "true");
  });

  it("closes the sheet when a match is opened", () => {
    const sheet = openSheet();
    fireEvent.change(within(sheet).getByRole("combobox"), { target: { value: "primo" } });
    fireEvent.click(within(sheet).getByRole("option", { name: /Primo Colon/ }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
