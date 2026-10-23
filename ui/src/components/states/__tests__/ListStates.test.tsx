// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ListStates } from "@/components/states";
import { ApiError } from "@/lib/api-client";
import { UiViewProvider } from "@/lib/ui-view";

function renderList(error: unknown, isOperator = true) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <UiViewProvider isOperator={isOperator}>
        <ListStates
          isLoading={false}
          error={error}
          isEmpty={false}
          onRetry={() => {}}
          errorTitle="People couldn't be loaded."
          empty={<p>Nobody yet</p>}
        >
          <p>Aanya Rao</p>
        </ListStates>
      </UiViewProvider>
    </QueryClientProvider>,
  );
}

describe("ListStates arrival", () => {
  const list = (isEmpty: boolean) => (
    <QueryClientProvider client={new QueryClient()}>
      <UiViewProvider isOperator>
        <ListStates
          isLoading={false}
          error={null}
          isEmpty={isEmpty}
          errorTitle="People couldn't be loaded."
          empty={<p>Nobody yet</p>}
        >
          <p>Aanya Rao</p>
        </ListStates>
      </UiViewProvider>
    </QueryClientProvider>
  );

  // Filtering to nothing and back is not a first paint (S11): the rows that
  // return are simply there, however soon after the first paint they come.
  it("does not replay the arrival when a filter empties the list and fills it again", () => {
    const { container, rerender } = render(list(false));
    expect(container.querySelector(".arrive-list")).not.toHaveAttribute("data-settled");
    rerender(list(true));
    expect(container.querySelector(".arrive-list")).toBeNull();
    rerender(list(false));
    expect(container.querySelector(".arrive-list")).toHaveAttribute("data-settled");
  });
});

describe("ListStates", () => {
  // N23: a directory Zitadel has never answered has nothing to show. The
  // degraded Notice above the page says so; the list adds no failure of its own.
  it("renders nothing for a directory that has never been read", () => {
    const { container } = renderList(
      new ApiError(503, {
        error: "DIRECTORY_UNREACHABLE",
        message: "Zitadel isn't answering",
      }),
    );
    expect(container).toBeEmptyDOMElement();
  });

  // A member has no degraded Notice above the page, so the list says it once.
  it("tells a member, in an amber Notice, that the sign-in system can't be reached", () => {
    renderList(
      new ApiError(503, {
        error: "DIRECTORY_UNREACHABLE",
        message: "Zitadel isn't answering",
      }),
      false,
    );
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Syndra can't reach the sign-in system.");
    expect(status).not.toHaveTextContent("right now");
    expect(status).toHaveTextContent(
      "Nothing about your access has changed. This page tries again on its own.",
    );
    // Amber, with the dot breathing: not the neutral of a network that is gone.
    expect(status.className).toContain("warn");
    expect(status.querySelector(".breathe")).not.toBeNull();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByText("Aanya Rao")).not.toBeInTheDocument();
  });

  it("says any other failed read as the danger Notice, ending Nothing was changed", () => {
    renderList(
      new ApiError(500, { error: "VIEW_ERROR", message: "The read failed" }),
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("People couldn't be loaded.");
    expect(alert).toHaveTextContent(/The read failed\. Nothing was changed\./);
    expect(
      screen.getByRole("button", { name: "Try again" }),
    ).toBeInTheDocument();
  });
});
