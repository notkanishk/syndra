// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it } from "vitest";

import { ViewSwitch } from "@/components/shell/ViewSwitch";
import { useUiView, UiViewProvider } from "@/lib/ui-view";
import { parseUiView, VIEW_COOKIE } from "@/lib/ui-view-cookie";

/**
 * The pill painted Basic for seconds after every full page load: the stored
 * view was read from localStorage after mount, so the server HTML (and the
 * first client paint) was always Basic. The server now reads the cookie, so the
 * very first markup is the stored view and hydration has nothing to correct.
 */

afterEach(() => {
  window.localStorage.clear();
  document.cookie = `${VIEW_COOKIE}=; path=/; max-age=0`;
});

function Probe() {
  return <span data-testid="view">{useUiView().view}</span>;
}

describe("the stored Basic / Advanced view", () => {
  it("is in the server's first HTML, so no navigation paints Basic first", () => {
    const html = renderToString(
      <UiViewProvider isOperator initialView="advanced">
        <ViewSwitch />
      </UiViewProvider>,
    );
    expect(html).toContain('aria-checked="true" title="Advanced view');
    expect(html).toContain('aria-checked="false" title="Basic view');
  });

  it("is the first client render too, with no effect needed", () => {
    render(
      <UiViewProvider isOperator initialView="advanced">
        <Probe />
      </UiViewProvider>,
    );
    expect(screen.getByTestId("view")).toHaveTextContent("advanced");
  });

  it("writes the cookie the server reads when the operator switches", () => {
    render(
      <UiViewProvider isOperator>
        <ViewSwitch />
      </UiViewProvider>,
    );
    screen.getByRole("radio", { name: "Advanced" }).click();
    expect(document.cookie).toContain(`${VIEW_COOKIE}=advanced`);
  });

  it("moves a view stored before the cookie existed into the cookie", () => {
    window.localStorage.setItem("syndra_ui_view", "advanced");
    render(
      <UiViewProvider isOperator>
        <Probe />
      </UiViewProvider>,
    );
    expect(screen.getByTestId("view")).toHaveTextContent("advanced");
    expect(document.cookie).toContain(`${VIEW_COOKIE}=advanced`);
  });

  it("never trusts a cookie that is not one of the two views", () => {
    expect(parseUiView("advanced")).toBe("advanced");
    expect(parseUiView("basic")).toBe("basic");
    expect(parseUiView("expert")).toBeUndefined();
    expect(parseUiView(undefined)).toBeUndefined();
  });

  it("is always Basic for a member, whatever the cookie says", () => {
    render(
      <UiViewProvider isOperator={false} initialView="advanced">
        <Probe />
      </UiViewProvider>,
    );
    expect(screen.getByTestId("view")).toHaveTextContent("basic");
  });
});
