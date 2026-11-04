// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { SyndraMark, WELCOMED } from "@/components/shell/SyndraMark";

const dot = (c: HTMLElement) => c.querySelector(".syndra-mark-dot")!;

beforeEach(() => window.sessionStorage.clear());

describe("the mark's welcome", () => {
  it("lights once at the session's first paint, and never again on a remount", () => {
    const first = render(<SyndraMark welcome />);
    expect(dot(first.container)).toHaveClass("bloom");
    first.unmount();
    const second = render(<SyndraMark welcome />);
    expect(dot(second.container)).not.toHaveClass("bloom");
  });

  it("lights again after sign-out clears the memory", () => {
    render(<SyndraMark welcome />).unmount();
    window.sessionStorage.removeItem(WELCOMED);
    expect(dot(render(<SyndraMark welcome />).container)).toHaveClass("bloom");
  });

  it("is still unless asked (the favicon drawing, the login orb)", () => {
    expect(dot(render(<SyndraMark />).container)).not.toHaveClass("bloom");
  });
});
