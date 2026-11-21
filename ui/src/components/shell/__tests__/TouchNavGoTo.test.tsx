// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OPEN_GO_TO } from "@/components/shell/GoToHost";
import { TouchNav } from "@/components/shell/TouchNav";
import { NAME_CATALOG_KEY } from "@/lib/queries/useNameCatalog";

/** A-19 P4 on a phone, as chosen (A-26 §0): one Go to. The sheet opens it; it is not a second one. */

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

describe("the phone's Go to sheet: one Go to (A-26 §0)", () => {
  it("has a door to Go to at the top, above the page list it keeps, and no second Go to inside it", () => {
    const sheet = openSheet();
    const door = within(sheet).getByRole("button", { name: "Go to by name" });
    expect(door).toHaveTextContent("A name, or a question");
    const bundles = within(sheet).getByRole("link", { name: /Bundles/ });
    expect(door.compareDocumentPosition(bundles) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(sheet).queryByRole("combobox")).toBeNull();
  });

  it("leaves by history as any dismissal does, and only once it has gone asks for the one Go to", async () => {
    const asked = vi.fn();
    window.addEventListener(OPEN_GO_TO, asked);
    const back = vi.spyOn(window.history, "back");
    const sheet = openSheet();
    fireEvent.click(within(sheet).getByRole("button", { name: "Go to by name" }));
    expect(back).toHaveBeenCalledTimes(1);
    // Still here until history answers: Go to never opens over another dialog.
    expect(asked).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Go to a page" })).toBeNull());
    expect(asked).toHaveBeenCalledTimes(1);
    window.removeEventListener(OPEN_GO_TO, asked);
    back.mockRestore();
  });

  it("asks for nothing when it is dismissed any other way", async () => {
    const asked = vi.fn();
    window.addEventListener(OPEN_GO_TO, asked);
    const sheet = openSheet();
    fireEvent.click(within(sheet).getByRole("button", { name: "Close this sheet" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Go to a page" })).toBeNull());
    expect(asked).not.toHaveBeenCalled();
    window.removeEventListener(OPEN_GO_TO, asked);
  });
});
