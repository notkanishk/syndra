// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { GoToHost } from "@/components/shell/GoToHost";

const view = vi.hoisted(() => ({ audience: "advanced" as "basic" | "advanced" | "member" }));
vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ audience: view.audience, view: view.audience, isOperator: view.audience !== "member", setView: () => {}, revealInAdvanced: () => {} }),
  useIsAdvanced: () => view.audience === "advanced",
}));
vi.mock("@/lib/queries/useTargets", () => ({ useTargets: () => ({ data: [] }) }));

function mount(extra?: React.ReactNode) {
  const client = new QueryClient();
  return render(
    <QueryClientProvider client={client}>
      <input aria-label="a field" />
      {extra}
      <GoToHost />
    </QueryClientProvider>,
  );
}

const key = (init: KeyboardEventInit, target: Element | Window = window) => fireEvent.keyDown(target, init);

beforeEach(() => {
  view.audience = "advanced";
});

describe("Go to's keys", () => {
  it("opens on / when nothing has the focus", () => {
    mount();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    key({ key: "/" });
    expect(screen.getByRole("dialog", { name: "Go to" })).toBeInTheDocument();
    // The input has the focus, so the slash is not typed into it.
    expect(screen.getByRole("combobox")).toHaveValue("");
  });

  it("opens on Cmd K and Ctrl K, and Cmd K again closes it", () => {
    mount();
    key({ key: "k", metaKey: true });
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    key({ key: "k", metaKey: true });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    key({ key: "K", ctrlKey: true });
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("never fires while typing in a field", () => {
    mount();
    const field = screen.getByLabelText("a field");
    field.focus();
    key({ key: "/" }, field);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("never fires while typing in a textarea or an editable region", () => {
    mount(<textarea aria-label="a note" />);
    key({ key: "/" }, screen.getByLabelText("a note"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    const editable = document.body.appendChild(document.createElement("div"));
    Object.defineProperty(editable, "isContentEditable", { value: true });
    key({ key: "/" }, editable);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("does not open on top of another dialog", () => {
    mount(<div role="dialog" aria-label="Something else" />);
    key({ key: "/" });
    expect(screen.queryByRole("dialog", { name: "Go to" })).not.toBeInTheDocument();
  });

  it("ignores a slash with a modifier", () => {
    mount();
    key({ key: "/", ctrlKey: true });
    key({ key: "/", altKey: true });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("lists Basic's pages in Basic and Advanced's in Advanced", () => {
    view.audience = "basic";
    const { unmount } = mount();
    key({ key: "/" });
    expect(screen.queryByRole("option", { name: /Pending changes/ })).not.toBeInTheDocument();
    unmount();
    view.audience = "advanced";
    mount();
    key({ key: "/" });
    expect(screen.getByRole("option", { name: /Pending changes/ })).toBeInTheDocument();
  });

  it("closes on Esc", () => {
    mount();
    key({ key: "/" });
    fireEvent.keyDown(screen.getByRole("combobox"), { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
