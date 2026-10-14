import { PlanReview } from "ui";

const wrap = { maxWidth: 760, background: "var(--surface-2)", padding: "16px 0", borderRadius: 16 } as const;

const row = (user_id: string, name: string, email: string, effect: any, detail: string, consequence?: string) => ({
  user_id, name, email, effect, detail, consequence,
});

const grant = {
  op: "assign_role" as const,
  plan_id: "plan_01JB2X",
  applied: false,
  outcomes: [
    row("u1", "Aanya Rao", "aanya.rao@example.edu", "apply", "Gets Operator on Laser cutter"),
    row("u2", "Kabir Mehta", "kabir.mehta@example.edu", "apply", "Gets Operator on Laser cutter", "Replaces his Member role there."),
    row("u3", "Ishita Sen", "ishita.sen@example.edu", "no_change", "Already holds Operator on Laser cutter"),
    row("u4", "Rohan Iyer", "rohan.iyer@example.edu", "blocked", "Has not finished laser safety training"),
  ],
  summary: { total: 4, apply: 2, no_change: 1, blocked: 1, failed: 0, succeeded: 0, queued: 0 },
};

const result = {
  op: "remove_role" as const,
  applied: true,
  outcomes: [
    row("u5", "Meera Pillai", "meera.pillai@example.edu", "applied", "Removed Member from CNC router"),
    row("u6", "Arjun Nair", "arjun.nair@example.edu", "queued", "Removal of Member from CNC router", "Still has access until it is sent."),
    row("u7", "Diya Kapoor", "diya.kapoor@example.edu", "failed", "Zitadel did not answer in time"),
  ],
  summary: { total: 3, apply: 0, no_change: 0, blocked: 0, failed: 1, succeeded: 1, queued: 1 },
};

const empty = {
  op: "publish_bundle_version" as const,
  plan_id: "plan_01JB3K",
  applied: false,
  outcomes: [],
  summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
};

export const Rehearsal = () => <div style={wrap}><PlanReview plan={grant} /></div>;
export const AfterApply = () => <div style={wrap}><PlanReview plan={result} /></div>;
export const ReachesNobody = () => <div style={wrap}><PlanReview plan={empty} /></div>;
export const ShortPayload = () => (
  <div style={wrap}><PlanReview plan={{ ...grant, outcomes: undefined as any }} noun={["request", "requests"]} /></div>
);
