// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CheckTrail } from "@/components/ui/CheckTrail";
import { TokenBlock } from "@/components/ui/TokenBlock";

describe("TokenBlock", () => {
  it("marks a draft's lines in a gutter and in words, so the difference never rests on colour", () => {
    render(
      <TokenBlock
        lines={[
          { key: "groups", text: '"groups": ["netbird-staff"]', mark: "−" },
          { key: "groups", text: '"groups": ["network-staff"]', mark: "+" },
          { key: "sub", text: '"sub": "1"', mark: " " },
        ]}
      />,
    );
    expect(screen.getByText("−")).toBeInTheDocument();
    expect(screen.getByText("+")).toBeInTheDocument();
    expect(screen.getByText("Removed:", { exact: false })).toHaveClass("sr-only");
    expect(screen.getByText("Added:", { exact: false })).toHaveClass("sr-only");
    expect(screen.getByText(/"groups": \["netbird-staff"\]/)).toHaveClass("line-through");
  });

  it("draws no gutter for a token with nothing to compare", () => {
    render(<TokenBlock lines={[{ key: "roles", text: '"roles": []' }]} />);
    expect(screen.queryByText("+")).toBeNull();
    expect(screen.queryByText("Added:", { exact: false })).toBeNull();
  });

  it("in raw mode wraps anywhere and never scrolls sideways", () => {
    const { container } = render(<TokenBlock raw={{ user: { email: "ada@example.edu" } }} />);
    const pre = container.querySelector("pre")!;
    expect(pre.className).toMatch(/whitespace-pre-wrap/);
    expect(pre.className).toMatch(/overflow-wrap:anywhere/);
    expect(pre.className).not.toMatch(/overflow-x/);
  });
});

describe("CheckTrail", () => {
  it("names each check's verdict for a screen reader, and in compact form keeps details only where one didn't pass", () => {
    render(
      <CheckTrail
        compact
        checks={[
          { tone: "healthy", sentence: "Their claims are ready.", faint: ["Built at 09:52."] },
          { tone: "danger", sentence: "Zitadel's settings keep its claims out.", faint: ["The token type is Bearer."], link: { label: "Set it to JWT", href: "https://auth.example.edu/x" } },
        ]}
      />,
    );
    expect(screen.getByText("Check 1, passes:")).toBeInTheDocument();
    expect(screen.getByText("Check 2, fails:")).toBeInTheDocument();
    expect(screen.queryByText("Built at 09:52.")).toBeNull();
    expect(screen.getByText("The token type is Bearer.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Set it to JWT →" })).toHaveAttribute("href", "https://auth.example.edu/x");
  });
});
