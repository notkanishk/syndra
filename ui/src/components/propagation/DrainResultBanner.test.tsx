// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DrainResultBanner } from "./DrainResultBanner";

describe("DrainResultBanner", () => {
  it("renders nothing before a drain has run", () => {
    const { container } = render(<DrainResultBanner result={undefined} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("surfaces errored rows as an alert telling the operator to retry", () => {
    render(
      <DrainResultBanner
        result={{ applied: 2, failed: 0, requeued: 0, errored: 1, halted: false }}
      />,
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/could not be recorded/i);
    expect(alert).toHaveTextContent(/resume again to retry/i);
    expect(alert).toHaveTextContent(/2 applied/);
  });

  it("prefers the errored alert even when some rows applied", () => {
    render(
      <DrainResultBanner
        result={{ applied: 5, failed: 1, requeued: 2, errored: 3, halted: false }}
      />,
    );
    // errored takes precedence over the neutral success summary
    expect(screen.getByRole("alert")).toHaveTextContent(/3 changes could not be recorded/i);
    expect(screen.queryByText(/drain complete/i)).not.toBeInTheDocument();
  });

  it("explains a halt reason", () => {
    render(
      <DrainResultBanner
        result={{ applied: 0, failed: 0, requeued: 0, errored: 0, halted: true, reason: "drain_in_progress" }}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent(/another drain is already running/i);
  });

  it("shows a clean success summary when nothing errored or halted", () => {
    render(
      <DrainResultBanner
        result={{ applied: 4, failed: 0, requeued: 1, errored: 0, halted: false }}
      />,
    );
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent(/drain complete/i);
    expect(status).toHaveTextContent(/4/);
    expect(status).toHaveTextContent(/1 requeued/);
  });
});
