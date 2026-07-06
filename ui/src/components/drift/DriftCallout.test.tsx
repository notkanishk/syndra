// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DriftCallout } from "./DriftCallout";

const item = (id: string) => ({
  id,
  user_id: "u",
  project_id: "p",
  role_keys: ["r"],
  drift_type: "zitadel_only",
  detection_source: "webhook",
  detected_at: "2026-07-06T00:00:00Z",
});

describe("DriftCallout", () => {
  it("renders count, a top-3 preview, and Triage all when drift exists", () => {
    render(<DriftCallout count={5} top={[item("1"), item("2"), item("3")]} />);
    expect(screen.getByRole("alert")).toHaveTextContent(/5/);
    expect(screen.getByRole("link", { name: /triage all/i })).toBeInTheDocument();
  });

  it("renders nothing when count is 0", () => {
    const { container } = render(<DriftCallout count={0} top={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("has no dismiss control (undismissible)", () => {
    render(<DriftCallout count={2} top={[item("1")]} />);
    expect(screen.queryByRole("button", { name: /dismiss/i })).not.toBeInTheDocument();
  });

  it("applies a motion-safe emphasis class to the count when it increases", () => {
    const { rerender } = render(<DriftCallout count={2} top={[item("1")]} />);
    expect(screen.getByText("2")).not.toHaveClass("motion-safe:animate-count-emphasis");

    rerender(<DriftCallout count={3} top={[item("1")]} />);
    expect(screen.getByText("3")).toHaveClass("motion-safe:animate-count-emphasis");
  });
});
