import { Modal, ModalHeader, ModalFooter, Button } from "ui";

const noop = () => {};
/** The card wrapper is transformed, so it is the containing block for the fixed overlay: give it the viewport. */
const stage = { margin: -24, height: "100dvh" } as const;

export const WithNote = () => (
  <div style={stage}>
  <Modal open onClose={noop} size="md" labelledBy="f-1">
    <ModalHeader
      titleId="f-1"
      title="Extend Rohan Iyer's CNC router access"
      lede="His Operator role ends in 3 days. Extending keeps it until the end of the semester."
    />
    <ModalFooter note="No change for Rohan's other roles — Syndra leaves those as they are.">
      <Button variant="ghost">Cancel</Button>
      <Button variant="accent">Extend to 15 Dec</Button>
    </ModalFooter>
  </Modal>
  </div>
);

export const DestructivePair = () => (
  <div style={stage}>
  <Modal open onClose={noop} size="sm" labelledBy="f-2">
    <ModalHeader titleId="f-2" title="Remove 3 people from Electronics bench?" />
    <ModalFooter>
      <Button variant="ghost">Keep access</Button>
      <Button variant="dangerConfirm">Remove 3 people</Button>
    </ModalFooter>
  </Modal>
  </div>
);
