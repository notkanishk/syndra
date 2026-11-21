// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { css } from "@/__tests__/cssTokens";
import { GESTURE, GO_TO_FLARE, GoTo } from "@/components/ui/GoTo";
import { SETTLE_MS } from "@/lib/motion";
import { forgetLately, keepLately } from "@/lib/go-to";
import { NAME_CATALOG_KEY } from "@/lib/queries/useNameCatalog";
import { ADVANCED_NAV, BASIC_NAV, memberNav } from "@/lib/nav";
import { motionInTests } from "@/test-utils/motion";

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
  return { ...render(<QueryClientProvider client={client}>{node}</QueryClientProvider>), client };
}

const type = (text: string) => fireEvent.change(screen.getByRole("combobox"), { target: { value: text } });
const names = () => screen.getAllByRole("option").map((o) => within(o).getAllByText(/./)[0].textContent);

beforeEach(() => {
  view.audience = "advanced";
  forgetLately();
});

describe("Go to's placeholder says what this session can reach (A-26 §0)", () => {
  it.each([
    ["member", memberNav([]), "A page"],
    ["basic", BASIC_NAV, "A person, a role, or a page"],
    ["advanced", ADVANCED_NAV, "A person, a role, a page, or a question"],
  ] as const)("%s", (audience, nav, words) => {
    view.audience = audience;
    renderIn(<GoTo open onClose={() => {}} nav={nav} />);
    expect(screen.getByRole("combobox")).toHaveAttribute("placeholder", words);
  });
});

const field = () => screen.getByRole("combobox");
const press = (key: string, extra: object = {}) => fireEvent.keyDown(field(), { key, ...extra });
const pill = () => screen.queryByRole("button", { name: /^Looking only among/ });
const scrim = () => screen.getByRole("dialog", { name: "Go to" });

describe("Go to, as chosen (A-26 §0)", () => {
  it("is one glass pane pinned at the top, the lamp the field's first glyph, and its dialog wears the hand", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    expect(scrim()).toHaveClass("go-to-scrim", "items-start");
    expect(scrim()).not.toHaveClass("items-end");
    // The edge under your hand, the same rule as every card's: the dialog opts in, the pane wears `rim`.
    expect(scrim()).toHaveAttribute("data-hand");
    const pane = scrim().querySelector(".go-to-pane")!;
    expect(pane).toHaveClass("rim");
    expect(within(scrim()).queryByRole("heading")).toBeNull();
    const row = field().parentElement!;
    expect(row).toHaveClass("go-to-field");
    expect(row.firstElementChild).toHaveAttribute("data-lamp");
    // No keys line: what the keys do is said by the offer and the pill.
    expect(screen.queryByText("move")).toBeNull();
  });

  it("at rest holds one quiet line: the last place, with Lately at its right; ↓ brings the rest", () => {
    keepLately("/applications");
    keepLately("/users/u2");
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    expect(names()).toEqual(["Primo Colon"]);
    expect(within(screen.getByRole("option")).getByText("Lately")).toBeInTheDocument();
    press("ArrowDown");
    expect(names()).toEqual(["Primo Colon", "Apps"]);
  });

  it("lists what needs you only after ↓, last, and only when something waits", () => {
    keepLately("/users/u2");
    const { client } = renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    expect(screen.queryByText("Needs you")).toBeNull();
    act(() => client.setQueryData(["governance", "indicators"], { pending_propagation: 3 }));
    expect(screen.queryByText("Needs you")).toBeNull();
    press("ArrowDown");
    const rows = screen.getAllByRole("option");
    expect(within(rows.at(-1)!).getByText("Needs you")).toBeInTheDocument();
    expect(rows.at(-1)).toHaveTextContent("3 changes are waiting for you to send");
  });

  it("grows one list as you type, best match first, seven at most, each with its kind", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("pri");
    // "Primo", "Print Lab", "Print Queue", "Printing basics" start with it; "Damian Priest" has a word that does.
    expect(names()).toEqual(["Primo Colon", "Print Lab", "Print Queue", "Printing basics", "Damian Priest"]);
    expect(within(screen.getByRole("option", { name: /Primo Colon/ })).getByText("Person")).toBeInTheDocument();
    expect(screen.queryAllByRole("group")).toHaveLength(0);
  });

  it("moves with the arrow keys, wrapping, and says which row is active", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("pri");
    const options = screen.getAllByRole("option");
    expect(field()).toHaveAttribute("aria-activedescendant", options[0].id);
    press("ArrowDown");
    expect(field()).toHaveAttribute("aria-activedescendant", options[1].id);
    press("ArrowUp");
    press("ArrowUp");
    expect(field()).toHaveAttribute("aria-activedescendant", options[options.length - 1].id);
  });

  it("never moves the selection under the hand: hovering a row only tints it", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("pri");
    const options = screen.getAllByRole("option");
    fireEvent.pointerMove(options[2]);
    fireEvent.mouseEnter(options[2]);
    fireEvent.mouseOver(options[2]);
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    expect(options[2]).toHaveAttribute("aria-selected", "false");
    // The row's hover is a list row's own (`hover-tint`), worn only by a row that is not chosen: a utility
    // outranks a component rule, so the chosen row's no-fill can only win by not wearing it.
    expect(options[0]).toHaveClass("go-to-row");
    expect(options[0]).not.toHaveClass("hover-tint");
    for (const option of options.slice(1)) expect(option).toHaveClass("go-to-row", "hover-tint");
    press("ArrowDown");
    expect(screen.getAllByRole("option")[0]).toHaveClass("hover-tint");
    expect(screen.getAllByRole("option")[1]).not.toHaveClass("hover-tint");
  });

  it("opens the selected row on Enter, as a link does, and closes", () => {
    const onClose = vi.fn();
    renderIn(<GoTo open onClose={onClose} nav={ADVANCED_NAV} />);
    type("primo");
    const link = screen.getByRole("option", { name: /Primo Colon/ });
    expect(link).toHaveAttribute("href", "/users/u2");
    const click = vi.fn((event: Event) => event.preventDefault());
    link.addEventListener("click", click);
    press("Enter");
    expect(click).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalled();
  });

  it("says Nothing by that name, beside the list and not in it, and tells a screen reader", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("zzzz");
    const said = screen.getAllByText("Nothing by that name.").find((el) => !el.closest('[role="status"]'))!;
    expect(said.closest('[role="listbox"]')).toBeNull();
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Nothing by that name.");
  });

  it("holds only options in its listbox, and one status line outside it", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("pri");
    const listbox = screen.getByRole("listbox", { hidden: true });
    for (const child of Array.from(listbox.children)) expect(child).toHaveAttribute("role", "option");
    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(screen.getByRole("status").closest('[role="listbox"]')).toBeNull();
  });

  it("ignores every key while an input method is composing: Enter opens nothing, Esc clears nothing", () => {
    const onClose = vi.fn();
    renderIn(<GoTo open onClose={onClose} nav={ADVANCED_NAV} />);
    type("primo");
    const link = screen.getByRole("option", { name: /Primo Colon/ });
    const click = vi.fn((event: Event) => event.preventDefault());
    link.addEventListener("click", click);
    press("Enter", { isComposing: true });
    expect(click).not.toHaveBeenCalled();
    press("Enter", { keyCode: 229 });
    expect(click).not.toHaveBeenCalled();
    press("Escape", { isComposing: true });
    expect(field()).toHaveValue("primo");
    expect(onClose).not.toHaveBeenCalled();
    press("ArrowDown", { isComposing: true });
    expect(field()).toHaveAttribute("aria-activedescendant", link.id);
  });

  it("gives a member their pages and nothing else, and no scopes", () => {
    view.audience = "member";
    renderIn(<GoTo open onClose={() => {}} nav={memberNav([])} />);
    type("peo");
    expect(screen.queryByText(/only people/)).toBeNull();
    type("@");
    expect(pill()).toBeNull();
    type("storage");
    expect(names()).toEqual(["Network storage"]);
  });

  it("is closed when not open", () => {
    const { rerender } = renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <GoTo open={false} onClose={() => {}} nav={ADVANCED_NAV} />
      </QueryClientProvider>,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

describe("a scope: entered, kept, and left (A-26 §0, the state machine)", () => {
  it("offers a kind's scope while its word is typed, and Tab takes it; Tab with nothing offered is the browser's", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("p");
    expect(screen.queryByText(/only people/)).toBeNull();
    press("Tab"); // nothing is offered: Tab is not Go to's
    expect(pill()).toBeNull();
    type("peo");
    const offer = screen.getByRole("button", { name: /only people/ });
    expect(offer).toHaveClass("go-to-offer");
    expect(press("Tab")).toBe(false); // taken
    // The dialog's focus trap leaves a spent Tab alone: the caret stays, so Backspace can give the typing back.
    expect(field()).toHaveFocus();
    expect(pill()).toHaveTextContent("People");
    expect(pill()).toHaveAttribute("aria-label", "Looking only among people. Look everywhere again.");
    expect(field()).toHaveValue("");
    expect(field()).toHaveAttribute("placeholder", "A person");
    expect(scrim()).toHaveAttribute("data-scope", "people");
    // In a scope: all of that kind, A to Z, and no row repeats what the pill says.
    expect(names()).toEqual(["Damian Priest", "Primo Colon"]);
    expect(screen.queryByText("Person")).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("Looking only among people.");
  });

  it("is entered by a tap on the offer, too, and the caret comes back to the field (the keyboard stays up)", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("role");
    const offer = screen.getByRole("button", { name: /only roles/ });
    // A tap moves the focus to what was tapped, and the offer is gone once it is taken.
    offer.focus();
    expect(offer).toHaveFocus();
    fireEvent.click(offer);
    expect(pill()).toHaveTextContent("Roles");
    expect(names()).toEqual(["Trained"]);
    expect(field()).toHaveFocus();
    // So the next key is typing, not a press of the pill.
    type("tr");
    expect(pill()).toHaveTextContent("Roles");
    expect(field()).toHaveValue("tr");
  });

  it("takes a pasted sigil's words as the typing inside the scope", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("@pri");
    expect(pill()).toHaveTextContent("People");
    expect(field()).toHaveValue("pri");
    expect(names()).toEqual(["Damian Priest", "Primo Colon"]);
    // A sigil gives nothing back: Backspace on the emptied field leaves with an empty field.
    type("");
    press("Backspace");
    expect(field()).toHaveValue("");
  });

  it.each([
    ["@", "People"],
    ["#", "Roles"],
    ["/", "Pages"],
  ])("is entered by %s first, which is not kept as typing", (sigil, word) => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type(sigil);
    expect(pill()).toHaveTextContent(word);
    expect(field()).toHaveValue("");
    // Backspace on the empty field leaves, and gives nothing back: a key is not a word.
    press("Backspace");
    expect(pill()).toBeNull();
    expect(field()).toHaveValue("");
  });

  it("gives the typing back on Backspace in an empty field: 'peo' comes back", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("peo");
    press("Tab");
    type("pr");
    expect(names()).toEqual(["Damian Priest", "Primo Colon"]);
    // With words typed, Backspace is the field's own.
    type("");
    press("Backspace");
    expect(pill()).toBeNull();
    expect(field()).toHaveValue("peo");
    expect(scrim()).not.toHaveAttribute("data-scope");
    expect(screen.getByRole("status")).toHaveTextContent("Looking everywhere again.");
  });

  it("leaves and clears on a press of the pill", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("@");
    type("da");
    fireEvent.click(pill()!);
    expect(pill()).toBeNull();
    expect(field()).toHaveValue("");
  });

  it("takes Esc in two stages: what is typed or chosen first, then the dialog", () => {
    const onClose = vi.fn();
    renderIn(<GoTo open onClose={onClose} nav={ADVANCED_NAV} />);
    type("@");
    type("da");
    press("Escape");
    expect(pill()).toBeNull();
    expect(field()).toHaveValue("");
    expect(onClose).not.toHaveBeenCalled();
    press("ArrowDown");
    press("Escape");
    expect(onClose).not.toHaveBeenCalled();
    press("Escape");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("answers the hand as a filled button does: the pill wears the edge, and the offer's hover is colour only", () => {
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("@");
    expect(pill()).toHaveClass("rim", "rim-filled");
    const offer = /\.go-to-offer \{([\s\S]*?)\n  \}\n/.exec(css)?.[1] ?? "";
    expect(offer).toMatch(/@media \(hover: hover\) \{\s*&:hover \{\s*color: var\(--ink\);\s*\}/);
    expect(offer).not.toMatch(/:hover \{[^}]*(font-weight|padding|scale|transform)/);
  });
});

describe("entering a scope: one gesture at the lamp, and still under less motion", () => {
  const animated = () => {
    const calls: string[] = [];
    const spy = vi.fn(function (this: Element) {
      calls.push(this.className.toString());
      return { cancel() {}, finished: Promise.resolve() } as unknown as Animation;
    });
    (Element.prototype as unknown as { animate: unknown }).animate = spy;
    return calls;
  };
  afterEach(() => {
    delete (Element.prototype as unknown as { animate?: unknown }).animate;
  });

  it("presses the bar 2% and runs the lamp's light through it, once, on entering", () => {
    motionInTests();
    const calls = animated();
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("peo");
    press("Tab");
    expect(calls.some((c) => c.includes("go-to-pane"))).toBe(true);
    expect(calls.some((c) => c.includes("go-to-beam"))).toBe(true);
    expect(calls.some((c) => c.includes("go-to-wave"))).toBe(true);
    expect(calls.some((c) => c.includes("go-to-halo"))).toBe(true);
    expect(calls.some((c) => c.includes("go-to-scrim"))).toBe(true);
    expect(GESTURE.press).toBe(0.98);
    // Never on leaving.
    calls.length = 0;
    press("Backspace");
    expect(calls).toEqual([]);
  });

  it("does nothing but turn the colour when less motion is asked for: no press, no beam, no wave", () => {
    motionInTests(false);
    const calls = animated();
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    type("peo");
    press("Tab");
    expect(calls).toEqual([]);
    expect(scrim()).toHaveAttribute("data-scope", "people");
    // The lamp's tint is a transition, and the one rule for less motion collapses every transition.
    expect(css).toMatch(/@media \(prefers-reduced-motion: reduce\) \{\s*\*,\s*\*::before,\s*\*::after \{[^}]*transition-duration: 0\.01ms !important;/);
  });
});

describe("a question, in Advanced (A-26 §0, §0a)", () => {
  const topology = {
    nodes: [
      { id: "role:p1:trained", kind: "role", label: "Trained", project_id: "p1", meta: { role_key: "trained" } },
      { id: "project:p1", kind: "project", label: "Print Lab" },
    ],
    edges: [],
  };

  it("answers in place from Insights' own reads, lists what it names beneath, and ⌘↵ from there follows the answer on the map", async () => {
    const { client } = renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    // The reads, as the server would answer them, and fresh: nothing here reaches a server.
    client.setQueryDefaults(["topology"], { staleTime: Infinity });
    client.setQueryDefaults(["insights"], { staleTime: Infinity });
    client.setQueryData(["topology", "graph"], topology);
    client.setQueryData(["insights", "who", "role:p1:trained", null], [{ user_id: "u2", name: "Primo Colon", how: [] }]);
    type("who has trained");
    const answer = await screen.findByRole("option", { name: /Follow it on the map/ });
    expect(answer).toHaveAttribute("href", expect.stringMatching(/^\/insights\?/));
    expect(answer.getAttribute("href")).toContain("q=who");
    const click = vi.fn((event: Event) => event.preventDefault());
    answer.addEventListener("click", click);
    // Beneath the answer, the places the question names (the board's three at most), best first.
    expect(names().slice(1)).toEqual(["Trained"]);
    press("ArrowDown");
    expect(screen.getAllByRole("option")[1]).toHaveAttribute("aria-selected", "true");
    expect(answer).toHaveAttribute("aria-selected", "false");
    expect(answer).toHaveTextContent("⌘↵");
    press("Enter", { metaKey: true });
    expect(click).toHaveBeenCalledTimes(1);
  });

  it("is not asked in Basic or by a member: Basic has no Insights to follow it to, and no read goes out", async () => {
    const fetched = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}", { status: 200 }));
    for (const [audience, nav] of [
      ["basic", BASIC_NAV],
      ["member", memberNav([])],
    ] as const) {
      view.audience = audience;
      const { unmount } = renderIn(<GoTo open onClose={() => {}} nav={nav} />);
      type("who has trained");
      await act(() => new Promise((done) => setTimeout(done, SETTLE_MS + 50)));
      expect(screen.queryByText(/Follow it on the map/)).toBeNull();
      expect(screen.queryByText(/isn't a question/)).toBeNull();
      unmount();
    }
    const reads = fetched.mock.calls.map(([url]) => String(url));
    expect(reads.filter((url) => /insights|topology/.test(url))).toEqual([]);
    fetched.mockRestore();
  });

  it("asks once the typing has rested, not on every key, and says it is working meanwhile", async () => {
    const fetched = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}", { status: 200 }));
    renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    for (const text of ["w", "wh", "who", "who ", "who h", "who ha", "who has"]) type(text);
    expect(screen.getByRole("status")).toHaveTextContent("Working it out.");
    expect(fetched.mock.calls.filter(([url]) => /topology/.test(String(url)))).toHaveLength(0);
    await act(() => new Promise((done) => setTimeout(done, SETTLE_MS + 50)));
    expect(fetched.mock.calls.filter(([url]) => /topology/.test(String(url)))).toHaveLength(1);
    fetched.mockRestore();
  });

  it("says when an answer could not be read, and what to do: try again, or open Insights", async () => {
    const { client } = renderIn(<GoTo open onClose={() => {}} nav={ADVANCED_NAV} />);
    client.setQueryDefaults(["topology"], { staleTime: Infinity });
    client.setQueryDefaults(["insights"], { staleTime: Infinity, retry: false });
    client.setQueryData(["topology", "graph"], topology);
    const fetched = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}", { status: 500 }));
    type("who has trained");
    await screen.findByRole("button", { name: "Try again" }, { timeout: 3000 });
    expect(screen.getByRole("status")).toHaveTextContent("Couldn't load this answer.");
    expect(screen.getByRole("link", { name: "open Insights" })).toHaveAttribute("href", expect.stringMatching(/^\/insights\?.*q=who/));
    const calls = fetched.mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await act(() => new Promise((done) => setTimeout(done, 50)));
    expect(fetched.mock.calls.length).toBeGreaterThan(calls);
    fetched.mockRestore();
  });
});

describe("the lamp's flare is spent inside the field (A-25 rule 1)", () => {
  it("never reaches past the field's half-height, on a laptop or a phone", () => {
    const halo = /\.go-to-halo \{\s*position: absolute;\s*inset: calc\(var\(--go-to-mark\) \* -([\d.]+)\);/.exec(css)!;
    const reach = 0.5 + Number(halo[1]);
    for (const [field, mark] of [
      [68, 32],
      [62, 28],
    ]) {
      expect(mark * reach * GO_TO_FLARE, `${field}px field`).toBeLessThanOrEqual(field / 2);
    }
    expect(css).toMatch(/--go-to-field: 68px;\s*--go-to-mark: 32px;/);
    expect(css).toMatch(/--go-to-field: 62px;\s*--go-to-mark: 28px;/);
  });
});
