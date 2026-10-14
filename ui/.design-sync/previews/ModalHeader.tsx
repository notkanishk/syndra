import { Modal, ModalHeader, ModalFooter, Button, Badge } from "ui";

const noop = () => {};
/** The card wrapper is transformed, so it is the containing block for the fixed overlay: give it the viewport. */
const stage = { margin: -24, height: "100dvh" } as const;

export const WithChipAndLede = () => (
  <div style={stage}>
  <Modal open onClose={noop} size="md" labelledBy="h-1">
    <ModalHeader
      titleId="h-1"
      chip={<Badge tone="accent">3D printers</Badge>}
      title="Grant Member to Kabir Mehta"
      lede="Kabir can book the Prusa printers and see the print queue. Supervisors are told by email."
    />
    <ModalFooter>
      <Button variant="ghost">Cancel</Button>
      <Button variant="accent">Grant Member</Button>
    </ModalFooter>
  </Modal>
  </div>
);

export const TitleOnly = () => (
  <div style={stage}>
  <Modal open onClose={noop} size="sm" labelledBy="h-2">
    <ModalHeader titleId="h-2" title="Rename this role" />
    <ModalFooter>
      <Button variant="ghost">Cancel</Button>
      <Button variant="accent">Save name</Button>
    </ModalFooter>
  </Modal>
  </div>
);
