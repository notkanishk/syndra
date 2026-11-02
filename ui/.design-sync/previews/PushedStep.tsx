import { useEffect } from "react";
import { Modal, ModalFooter, ModalHeader, Button, PushedStep, SyndraRoot, WouldCancelQueued, useCancelStep } from "ui";

const catalog = {
  users: [
    { id: "284019337710553311", name: "Kabir Mehta", email: "kabir.mehta@example.edu", title: "Student", team: "Wood Shop", status: "active", avatar: "" },
    { id: "284019337710554187", name: "Priya Nair", email: "priya.nair@example.edu", title: "Makerspace staff", team: "Staff", status: "active", avatar: "" },
  ],
  projects: [
    { id: "382075332397630470", name: "Laser Cutter", kind: "machine", description: "", roles: [
      { key: "operator", label: "Operator", description: "" },
      { key: "supervisor", label: "Supervisor", description: "" },
    ] },
  ],
  applications: [],
};
const seed = [{ key: ["name-catalog"], value: catalog }, { key: ["name-bundles"], value: [] }];
const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
const stage = { margin: -24, height: "100dvh" } as const;
const noop = () => {};

function Host() {
  const step = useCancelStep({ host: "Remove Operator from Kabir Mehta", goAhead: "remove", tone: "danger" });
  useEffect(() => {
    // The write is refused once: it would cancel a grant that is still waiting.
    void step.run(() =>
      Promise.reject(
        new WouldCancelQueued(
          [{
            id: "q-1",
            op_type: "grant",
            user_id: "284019337710553311",
            project_id: "382075332397630470",
            role_keys: ["supervisor"],
            source: "direct",
            initiated_by: "284019337710554187",
            reason: "Covering Saturday open hours.",
            queued_at: ago(40),
          }],
          "DELETE",
          "/api/grants",
        ),
      ),
    ).catch(noop);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- refuse once, on mount
  }, []);
  return (
    <Modal open onClose={noop} size="md" labelledBy="m-host">
      <PushedStep step={step}>
        <ModalHeader titleId="m-host" title="Remove Operator from Kabir Mehta?" lede="Kabir keeps his Member role." />
        <ModalFooter>
          <Button variant="ghost">Keep access</Button>
          <Button variant="dangerConfirm">Remove Operator</Button>
        </ModalFooter>
      </PushedStep>
    </Modal>
  );
}

/** A write that would cancel a change still waiting: the step is pushed inside the host's own sheet. */
export const CancelsAWaitingGrant = () => (
  <div style={stage}>
    <SyndraRoot data={seed}>
      <Host />
    </SyndraRoot>
  </div>
);
