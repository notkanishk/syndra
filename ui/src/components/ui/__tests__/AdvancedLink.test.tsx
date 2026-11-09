// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AdvancedLink } from "@/components/ui/AdvancedLink";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { NextStep } from "@/components/ui/NextStep";

const view = vi.hoisted(() => ({ value: "basic" as "basic" | "advanced", setView: vi.fn() }));
vi.mock("@/lib/ui-view", () => ({ useUiView: () => ({ view: view.value, setView: view.setView }) }));

// A click that ends in a navigation would make jsdom complain; the order is what is tested.
const click = (el: HTMLElement) => {
  el.addEventListener("click", (event) => event.preventDefault());
  fireEvent.click(el);
};

beforeEach(() => {
  view.value = "basic";
  view.setView.mockClear();
});

describe("AdvancedLink (A-20 P10)", () => {
  it("switches Basic to Advanced on a click to an Advanced-only page, and keeps the link", () => {
    render(<AdvancedLink href="/governance/drift">Drift</AdvancedLink>);
    const link = screen.getByRole("link", { name: "Drift" });
    expect(link).toHaveAttribute("href", "/governance/drift");
    click(link);
    expect(view.setView).toHaveBeenCalledTimes(1);
    expect(view.setView).toHaveBeenCalledWith("advanced");
  });

  it("does not touch the view when it is already Advanced", () => {
    view.value = "advanced";
    render(<AdvancedLink href="/governance/drift">Drift</AdvancedLink>);
    click(screen.getByRole("link"));
    expect(view.setView).not.toHaveBeenCalled();
  });

  it("never touches the view for a page Basic has", () => {
    render(<AdvancedLink href="/users/u1?tab=access">Cody</AdvancedLink>);
    click(screen.getByRole("link"));
    expect(view.setView).not.toHaveBeenCalled();
  });

  it("still runs the caller's own onClick", () => {
    const onClick = vi.fn();
    render(
      <AdvancedLink href="/audit" onClick={onClick}>
        Audit
      </AdvancedLink>,
    );
    click(screen.getByRole("link"));
    expect(onClick).toHaveBeenCalled();
    expect(view.setView).toHaveBeenCalledWith("advanced");
  });

  it("is what ArrowLink and NextStep are, with no extra words", () => {
    render(
      <>
        <ArrowLink href="/bundles">Open bundles</ArrowLink>
        <NextStep person="u1" />
      </>,
    );
    click(screen.getByRole("link", { name: "Open bundles" }));
    click(screen.getByRole("link", { name: /Waiting for you to send/ }));
    expect(view.setView).toHaveBeenCalledTimes(2);
    expect(document.body.textContent).not.toMatch(/in Advanced/i);
  });
});
