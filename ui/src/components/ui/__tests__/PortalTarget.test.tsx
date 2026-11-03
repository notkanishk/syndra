// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Modal, ModalHeader, PortalTarget } from "@/components/ui/Modal";

// The app never sets PortalTarget: overlays go under <body>. The design canvas
// sets it to a frame, and an open dialog must then stay inside that frame and
// leave the board's <body> alone.
describe("PortalTarget", () => {
  it("puts a dialog under <body> when nothing sets a target", () => {
    render(
      <Modal open onClose={() => {}}>
        <ModalHeader title="Revoke access" />
      </Modal>,
    );
    expect(screen.getByRole("dialog").closest("body")).toBe(document.body);
  });

  it("puts a dialog inside the target a frame provides, and nowhere else", () => {
    const frame = document.createElement("div");
    document.body.appendChild(frame);
    render(
      <PortalTarget.Provider value={frame}>
        <Modal open onClose={() => {}}>
          <ModalHeader title="Revoke access" />
        </Modal>
      </PortalTarget.Provider>,
    );
    const dialog = screen.getByRole("dialog");
    expect(frame.contains(dialog)).toBe(true);
    frame.remove();
  });
});
