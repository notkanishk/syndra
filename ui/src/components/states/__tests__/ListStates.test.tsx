// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ListStates } from "@/components/states";
import { ApiError } from "@/lib/api-client";

function renderList(error: unknown) {
  return render(
    <ListStates
      isLoading={false}
      error={error}
      isEmpty={false}
      onRetry={() => {}}
      errorTitle="People couldn't be loaded."
      empty={<p>Nobody yet</p>}
    >
      <p>Aanya Rao</p>
    </ListStates>,
  );
}

describe("ListStates", () => {
  // N23: a directory Zitadel has never answered has nothing to show. The
  // degraded Notice above the page says so; the list adds no failure of its own.
  it("renders nothing for a directory that has never been read", () => {
    const { container } = renderList(
      new ApiError(503, { error: "DIRECTORY_UNREACHABLE", message: "Zitadel isn't answering" }),
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("says any other failed read as the danger Notice, ending Nothing was changed", () => {
    renderList(new ApiError(500, { error: "VIEW_ERROR", message: "The read failed" }));
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("People couldn't be loaded.");
    expect(alert).toHaveTextContent(/The read failed\. Nothing was changed\./);
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
  });
});
