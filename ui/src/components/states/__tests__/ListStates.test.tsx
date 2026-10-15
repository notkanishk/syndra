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
  it("tells a member, in one neutral line, that the sign-in system can't be reached", () => {
    renderList(
      new ApiError(503, {
        error: "DIRECTORY_UNREACHABLE",
        message: "Zitadel isn't answering",
      }),
      false,
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      /can't reach the sign-in system/,
    );
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
