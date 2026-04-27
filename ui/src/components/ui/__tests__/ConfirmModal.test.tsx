// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ConfirmModal } from "@/components/ui/ConfirmModal";

describe("ConfirmModal a11y contract (preserved through Modal refactor)", () => {
  it("renders with role=dialog, aria-modal, and labelled/described attributes", () => {
    render(
      <ConfirmModal
        open={true}
        title="Revoke grant"
        description="The user will lose access immediately."
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toHaveAttribute("aria-labelledby", "confirm-title");
    expect(dialog).toHaveAttribute("aria-describedby", "confirm-description");
    expect(screen.getByText("Revoke grant")).toBeInTheDocument();
    expect(
      screen.getByText("The user will lose access immediately."),
    ).toBeInTheDocument();
  });

  it("dismisses on Escape (cancels via onCancel)", () => {
    const onCancel = vi.fn();
    const onConfirm = vi.fn();
    render(
      <ConfirmModal
        open={true}
        title="Confirm"
        description="..."
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("invokes onConfirm when the confirm button is clicked", () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmModal
        open={true}
        title="Confirm"
        description="..."
        confirmLabel="Revoke"
        onConfirm={onConfirm}
        onCancel={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("invokes onCancel when the cancel button is clicked", () => {
    const onCancel = vi.fn();
    render(
      <ConfirmModal
        open={true}
        title="Confirm"
        description="..."
        cancelLabel="Keep"
        onConfirm={() => {}}
        onCancel={onCancel}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Keep" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("disables both buttons and sets aria-busy when isPending=true", () => {
    render(
      <ConfirmModal
        open={true}
        title="Confirm"
        description="..."
        cancelLabel="Cancel"
        onConfirm={() => {}}
        onCancel={() => {}}
        isPending
      />,
    );
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    const confirm = screen.getByRole("button", { name: /working/i });
    expect(confirm).toBeDisabled();
    expect(confirm).toHaveAttribute("aria-busy", "true");
  });

  it("does not render when open=false", () => {
    render(
      <ConfirmModal
        open={false}
        title="Confirm"
        description="..."
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
