// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AnswerPanel, type Panel } from "@/components/insights/AnswerPanel";
import { ToneDot, nodeSurface } from "@/components/insights/tone";

afterEach(cleanup);

// A-20 P12: Insights to the rules.

describe("a node with no part in the answer", () => {
  it("recedes to the canvas ground with a line-strong edge, solid, never dashed", () => {
    const s = nodeSurface(undefined, false, true, false);
    expect(s.background).toBe("var(--canvas)");
    expect(s.borderColor).toBe("var(--line-strong)");
    expect(s.borderStyle).toBe("solid");
  });

  it("is solid whatever it is, so dashed keeps meaning NotOpenYet", () => {
    for (const unused of [true, false]) for (const dim of [true, false]) expect(nodeSurface(undefined, false, unused, dim).borderStyle).toBe("solid");
  });
});

describe("tone dots", () => {
  it("are StatusDots: a named tone takes the tone class, a shade of the accent takes a token fill", () => {
    const { container } = render(
      <>
        <ToneDot tone="warn" />
        <ToneDot tone="accent2" />
      </>,
    );
    const [warn, shade] = Array.from(container.querySelectorAll("[data-dot]")) as HTMLElement[];
    expect(warn.className).toContain("bg-warn");
    expect(shade.style.background).toContain("var(--accent)");
  });
});

const panel = (tone: Panel["tone"]): Panel => ({ tone, eyebrow: "Eyebrow", title: "Title", lines: [{ tone: "ink", text: "A sentence." }] });

describe("the answer panel", () => {
  it.each([
    ["warn", "text-warn-text"],
    ["danger", "text-danger-text"],
  ] as const)("is one surface for a %s answer, told by a toned line and not a tinted box", (tone, text) => {
    render(<AnswerPanel panel={panel(tone)} />);
    const box = screen.getByRole("region", { name: "Title" });
    expect(box.className).toContain("bg-surface-1");
    expect(box.className).not.toMatch(/-soft/);
    expect(screen.getByText("Eyebrow").className).toContain(text);
    expect(box.querySelectorAll("[data-dot]").length).toBeGreaterThanOrEqual(2); // the line's dot, and the sentence's
  });

  it("keeps a neutral answer's eyebrow plain, with no dot", () => {
    render(<AnswerPanel panel={panel("neutral")} />);
    expect(screen.getByText("Eyebrow").className).toContain("text-label");
    expect(screen.getByText("Eyebrow").querySelector("[data-dot]")).toBeNull();
  });

  it("names the objects its buttons act on", () => {
    const onBack = vi.fn();
    const onPin = vi.fn();
    render(<AnswerPanel panel={panel("accent")} onBack={onBack} onPin={onPin} />);
    fireEvent.click(screen.getByRole("button", { name: "Back to the map" }));
    fireEvent.click(screen.getByRole("button", { name: "Keep this answer" }));
    expect(onBack).toHaveBeenCalledOnce();
    expect(onPin).toHaveBeenCalledOnce();
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Pin/ })).toBeNull();
  });
});
