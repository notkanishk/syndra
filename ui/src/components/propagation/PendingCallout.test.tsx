// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PendingCallout } from "./PendingCallout";

describe("PendingCallout", () => {
  it("renders count + enabled Resume now when count > 0 and reachable", () => {
    render(
      <PendingCallout count={3} reachable dismissed={false} onResume={() => {}} onDismiss={() => {}} />,
    );
    expect(screen.getByText(/3/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /resume now/i })).toBeEnabled();
  });

  it("disables Resume and shows offline when unreachable", () => {
    render(
      <PendingCallout count={3} reachable={false} dismissed={false} onResume={() => {}} onDismiss={() => {}} />,
    );
    expect(screen.getByRole("button", { name: /resume now/i })).toBeDisabled();
    expect(screen.getByText(/offline/i)).toBeInTheDocument();
  });

  it("renders nothing when count is 0", () => {
    const { container } = render(
      <PendingCallout count={0} reachable dismissed={false} onResume={() => {}} onDismiss={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when dismissed", () => {
    const { container } = render(
      <PendingCallout count={5} reachable dismissed onResume={() => {}} onDismiss={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
