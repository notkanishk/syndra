// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { GoTo, GoToPanel } from "@/components/ui/GoTo";
import { NAME_CATALOG_KEY } from "@/lib/queries/useNameCatalog";
import { ADVANCED_NAV, BASIC_NAV, memberNav } from "@/lib/nav";

const view = vi.hoisted(() => ({ audience: "advanced" as "basic" | "advanced" | "member" }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ audience: view.audience, view: view.audience, isOperator: view.audience !== "member", setView: () => {}, revealInAdvanced: () => {} }),
  useIsAdvanced: () => view.audience === "advanced",
}));

const catalog = {
  users: [
    { id: "u2", name: "Primo Colon", email: "p@example.edu", title: "Student Staff", team: "Fabrication" },
    { id: "u1", name: "Damian Priest", email: "a@example.edu", title: "", team: "Wood shop" },
  ],
  projects: [{ id: "p1", name: "Print Lab", kind: "", description: "", roles: [{ key: "trained", label: "Trained", description: "" }] }],
  applications: [{ id: "a1", name: "Print Queue", project_id: "p1", description: "", consumer: "oidc", claim_name: "", format_type: "" }],
};

function renderIn(node: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(NAME_CATALOG_KEY, catalog);
  client.setQueryData(["name-bundles"], [{ id: "b1", name: "Printing basics" }]);
  return render(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
}

const type = (text: string) => fireEvent.change(screen.getByRole("combobox"), { target: { value: text } });
const names = () => screen.getAllByRole("option").map((o) => within(o).getAllByText(/./)[0].textContent);

beforeEach(() => {
  view.audience = "advanced";
});

describe("Go to's placeholder says what this session can reach (A-20 P17)", () => {
  it.each([
    ["member", memberNav([]), "A page"],
    ["basic", BASIC_NAV, "A person, a project, an app, or a page"],
    ["advanced", ADVANCED_NAV, "A person, a project, an app, a bundle, or a page"],
  ] as const)("%s", (audience, nav, words) => {
    view.audience = audience;
    renderIn(<GoTo open onClose={() => {}} nav={nav} />);
    expect(screen.getByRole("combobox")).toHaveAttribute("placeholder", words);
  });
});

describe("Go to", () => {
  it("is a dialog named Go to, holding a combobox that owns a listbox", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    const dialog = screen.getByRole("dialog", { name: "Go to" });
    const box = within(dialog).getByRole("combobox");
    expect(box).toHaveAttribute("aria-controls", within(dialog).getByRole("listbox").id);
    // Empty: pages only.
    expect(screen.queryAllByRole("group")).toHaveLength(1);
    expect(screen.getAllByRole("option").length).toBeGreaterThan(5);
  });

  it("is closed when not open, and reopens clean", () => {
    const { rerender } = renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("pri");
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <GoTo open={false} onClose={() => {}} nav={ADVANCED_NAV} />
      </QueryClientProvider>,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("groups results in a fixed order, each A to Z, naming the kind on the right", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("pri");
    const groups = screen.getAllByRole("group").map((g) => g.getAttribute("aria-labelledby")).map((id) => document.getElementById(id!)!.textContent);
    expect(groups).toEqual(["People", "Projects", "Apps", "Bundles"]);
    expect(names()).toEqual(["Damian Priest", "Primo Colon", "Print Lab", "Print Queue", "Printing basics"]);
    expect(within(screen.getByRole("option", { name: /Primo Colon/ })).getByText("Person")).toBeInTheDocument();
  });

  it("shows Advanced pages only in Advanced, and no Bundles in Basic", () => {
    view.audience = "basic";
    renderIn(<GoTo open onClose={() => {}} nav={BASIC_NAV} />);
    expect(screen.queryByRole("option", { name: /Pending changes/ })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Requests/ })).toBeInTheDocument();
    type("pri");
    expect(screen.queryByText("Printing basics")).not.toBeInTheDocument();
    expect(screen.getAllByRole("option", { name: /Print Lab/ }).length).toBeGreaterThan(0);
  });

  it("gives a member their pages and nothing else", () => {
    view.audience = "member";
    renderIn(<GoTo open onClose={() => {}} nav={memberNav([])} />);
    type("pri");
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    expect(screen.getByText("Nothing by that name.")).toBeInTheDocument();
    type("storage");
    expect(names()).toEqual(["Network storage"]);
  });

  it("moves with the arrow keys, wrapping, and says which row is active", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("pri");
    const box = screen.getByRole("combobox");
    const options = screen.getAllByRole("option");
    expect(box).toHaveAttribute("aria-activedescendant", options[0].id);
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    fireEvent.keyDown(box, { key: "ArrowDown" });
    expect(box).toHaveAttribute("aria-activedescendant", options[1].id);
    fireEvent.keyDown(box, { key: "ArrowUp" });
    fireEvent.keyDown(box, { key: "ArrowUp" });
    expect(box).toHaveAttribute("aria-activedescendant", options[options.length - 1].id);
  });

  it("opens the selected row on Enter, as a link does, and closes", () => {
    const onClose = vi.fn();
    renderIn(<GoTo open onClose={onClose} nav={ADVANCED_NAV} />);
    type("primo");
    const link = screen.getByRole("option", { name: /Primo Colon/ });
    expect(link).toHaveAttribute("href", "/users/u2");
    const click = vi.fn((event: Event) => event.preventDefault());
    link.addEventListener("click", click);
    fireEvent.keyDown(screen.getByRole("combobox"), { key: "Enter" });
    expect(click).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalled();
  });

  it("closes on Esc", () => {
    const onClose = vi.fn();
    renderIn(<GoTo open onClose={onClose} nav={ADVANCED_NAV} />);
    fireEvent.keyDown(screen.getByRole("combobox"), { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });

  it("says Nothing by that name, and nothing more", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("zzzz");
    expect(screen.getByText("Nothing by that name.")).toBeInTheDocument();
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
  });
});

describe("Go to inside the phone sheet", () => {
  it("keeps the sheet's own list until something is typed, then lists matches in place of it", () => {
    renderIn(<GoToPanel embedded nav={ADVANCED_NAV} onGo={() => {}} idle={<p>the sheet pages</p>} />);
    expect(screen.getByText("the sheet pages")).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toHaveAttribute("aria-expanded", "false");
    type("pri");
    expect(screen.queryByText("the sheet pages")).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Primo Colon/ })).toBeInTheDocument();
    // No key hints on a phone.
    expect(screen.queryByText("Esc")).not.toBeInTheDocument();
  });
});
