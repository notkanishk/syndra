import { Modal, ModalHeader, ModalFooter, Button, Badge, ConfirmByTyping } from "ui";

const noop = () => {};
/** The card wrapper is transformed, so it is the containing block for the fixed overlay: give it the viewport. */
const stage = { margin: -24, height: "100dvh" } as const;

/** `md` acts on one named subject: header, a short body, the footer's two actions. */
export const RemoveRole = () => (
  <div style={stage}>
  <Modal open onClose={noop} size="md" labelledBy="m-remove">
    <ModalHeader
      titleId="m-remove"
      chip={<Badge tone="warn">Laser cutter</Badge>}
      title="Remove Operator from Aanya Rao?"
      lede="Aanya keeps her Member role. She can no longer start a cut job on her own."
    />
    <ModalFooter note="Syndra records this before it reaches Zitadel.">
      <Button variant="ghost">Keep access</Button>
      <Button variant="dangerConfirm">Remove Operator</Button>
    </ModalFooter>
  </Modal>
  </div>
);

/** `sm` is a single confirmation; rung 3 types the subject's name back. */
export const ConfirmSmall = () => (
  <div style={stage}>
  <Modal open onClose={noop} size="sm" labelledBy="m-delete">
    <ModalHeader
      titleId="m-delete"
      title="Delete the Wood shop bundle?"
      lede="14 people hold it. They lose every role it gives them."
    />
    <div style={{ padding: "0 24px" }}>
      <ConfirmByTyping expected="Wood shop" value="Wood sh" onChange={noop} />
    </div>
    <ModalFooter>
      <Button variant="ghost">Cancel</Button>
      <Button variant="dangerConfirm" disabled reason="Type the bundle name to confirm.">
        Delete bundle
      </Button>
    </ModalFooter>
  </Modal>
  </div>
);

/** While a mutation is in flight the dialog cannot be dismissed. */
export const Busy = () => (
  <div style={stage}>
  <Modal open onClose={noop} size="md" busy labelledBy="m-busy">
    <ModalHeader titleId="m-busy" title="Granting CNC router access" lede="Sending 6 grants to Zitadel." />
    <ModalFooter>
      <Button variant="accent" isPending>Granting…</Button>
    </ModalFooter>
  </Modal>
  </div>
);
