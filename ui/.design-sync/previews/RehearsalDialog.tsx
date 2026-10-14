import { RehearsalDialog, Select } from "ui";

const noop = () => {};
/** The card wrapper is transformed, so it is the containing block for the fixed overlay: give it the viewport. */
const stage = { margin: -24, height: "100dvh" } as const;
const row = (user_id: string, name: string, effect: any, detail: string, consequence?: string) => ({
  user_id, name, email: `${name.toLowerCase().replace(" ", ".")}@example.edu`, effect, detail, consequence,
});

const grant = {
  op: "assign_role" as const,
  plan_id: "plan_01JB2X",
  applied: false,
  outcomes: [
    row("u1", "Aanya Rao", "apply", "Gets Member on 3D printers"),
    row("u2", "Kabir Mehta", "apply", "Gets Member on 3D printers"),
    row("u3", "Ishita Sen", "apply", "Gets Member on 3D printers"),
    row("u4", "Rohan Iyer", "no_change", "Already holds Member on 3D printers"),
  ],
  summary: { total: 4, apply: 3, no_change: 1, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
};

const removal = {
  op: "remove_role" as const,
  plan_id: "plan_01JB4R",
  applied: false,
  outcomes: [
    row("u5", "Meera Pillai", "apply", "Loses Operator on CNC router", "Keeps Member there."),
    row("u6", "Arjun Nair", "apply", "Loses Operator on CNC router"),
    row("u7", "Diya Kapoor", "blocked", "Is the only Supervisor on CNC router"),
  ],
  summary: { total: 3, apply: 2, no_change: 0, blocked: 1, failed: 0, succeeded: 0, queued: 0 },
};

const pending = () => new Promise<never>(() => {});

/** No compose step: the dialog rehearses on open and lands on the plan. */
export const ReviewGrant = () => (
  <div style={stage}>
  <RehearsalDialog
    title="Grant Member on 3D printers to 4 people"
    lede="Pick a role to grant."
    noun={["person", "people"]}
    onRehearse={async () => grant}
    onApply={pending}
    onClose={noop}
  />
  </div>
);

export const ReviewRemovalDestructive = () => (
  <div style={stage}>
  <RehearsalDialog
    title="Remove Operator on CNC router from 3 people"
    lede="Remove a role."
    noun={["person", "people"]}
    destructive
    consequence="Anyone mid-job on the router keeps their session until it ends."
    onRehearse={async () => removal}
    onApply={pending}
    onClose={noop}
  />
  </div>
);

/** Compose step, incomplete: the preview button carries its reason. */
export const ComposeNotReady = () => (
  <div style={stage}>
  <RehearsalDialog
    title="Grant a role to 12 people"
    lede="Choose the role. Syndra shows exactly what would change before anything is sent."
    noun={["person", "people"]}
    compose={
      <Select defaultValue="">
        <option value="" disabled>Choose a role…</option>
        <option>Member · Laser cutter</option>
        <option>Operator · Wood shop</option>
      </Select>
    }
    ready={false}
    notReadyReason="Choose a role first."
    onRehearse={async () => grant}
    onApply={pending}
    onClose={noop}
  />
  </div>
);
