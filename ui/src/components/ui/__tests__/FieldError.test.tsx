// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FieldError } from "@/components/ui/Input";
import { motionInTests } from "@/test-utils/motion";

/** Motion v2 (field error): it appears still, on tint, under its field, and goes on tint when the field is edited. */

beforeEach(() => {
  vi.useFakeTimers();
  motionInTests();
});
afterEach(() => vi.useRealTimers());

describe("FieldError", () => {
  it("says nothing, and takes no room, when there is no error", () => {
    const { container } = render(<FieldError>{undefined}</FieldError>);
    expect(container).toBeEmptyDOMElement();
  });

  it("appears on tint, as an alert", () => {
    render(<FieldError>Name the term.</FieldError>);
    expect(screen.getByRole("alert")).toHaveClass("tint-in");
    expect(screen.getByRole("alert")).toHaveTextContent("Name the term.");
  });

  it("goes on tint when the error clears, still reading what it said, then is gone", () => {
    const { rerender } = render(<FieldError>Name the term.</FieldError>);
    rerender(<FieldError>{undefined}</FieldError>);

    const leaving = screen.getByRole("alert");
    expect(leaving).toHaveClass("tint-out");
    expect(leaving).toHaveTextContent("Name the term.");

    act(() => void vi.advanceTimersByTime(120));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("is simply gone under reduced motion", () => {
    motionInTests(false);
    const { rerender } = render(<FieldError>Name the term.</FieldError>);
    rerender(<FieldError>{undefined}</FieldError>);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
