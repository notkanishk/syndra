// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { HomeChoice } from "@/components/kinds/storage/HomeMapping";

const props = { target: "truenas", name: "TrueNAS", hadHome: false, home: false, onHome: () => {}, limit: "none" as const, onLimit: () => {}, gb: "", onGb: () => {} };

// D-1 §5: a sentence and the link that follows it are a row, with the link in its seat.
describe("choosing where home folders go", () => {
  it("puts the link in the narrow seat, beside the sentence, not inside it", () => {
    render(<HomeChoice {...props} homeRoot="" />);
    const link = screen.getByRole("link", { name: /Choose where home folders go/ });
    expect(link.closest("[data-action-seat]")).toHaveAttribute("data-seat", "narrow");
    const sentence = screen.getByText(/No mapping can give one/);
    expect(sentence).not.toContainElement(link);
    expect(sentence.textContent).toMatch(/give it a home folder later by changing it\.$/);
  });

  it("draws no link once a place is chosen", () => {
    render(<HomeChoice {...props} homeRoot="main › home_dir" />);
    expect(screen.queryByRole("link", { name: /Choose where home folders go/ })).toBeNull();
  });
});
