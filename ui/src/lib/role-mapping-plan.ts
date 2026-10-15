import { ApiError } from "@/lib/api-client";
import { describeFailure, sentence } from "@/lib/outcome";
import type { BulkEffect, BulkOutcome, BulkPlan } from "@/lib/queries/useBulkGrants";
import {
  applyMappingDelete,
  applyMappingEdit,
  rehearseMappingCreate,
  rehearseMappingDelete,
  rehearseMappingEdit,
  type MappingRehearsal,
  type RoleMapping,
} from "@/lib/queries/useMappings";

/**
 * What a role gives on a system, as one decision (A-03 §2).
 *
 * A role's group, whether it gives a home folder, and that folder's limit are
 * three mapping rows, because each is a field the add-on converges. The
 * operator decides them together, so they are rehearsed together and reviewed
 * as one plan, then applied each under its own approval.
 */

export const FIELD_GROUP = "group";
export const FIELD_HOME = "home";
export const FIELD_HOME_QUOTA = "home_quota";

/** A role's rows on a system, by field. */
export interface RoleRows {
  group?: RoleMapping;
  home?: RoleMapping;
  quota?: RoleMapping;
}

export function rowsOf(mappings: RoleMapping[], projectId: string, roleKey: string): RoleRows {
  const mine = mappings.filter((m) => m.project_id === projectId && m.role_key === roleKey);
  return {
    group: mine.find((m) => m.field === FIELD_GROUP),
    home: mine.find((m) => m.field === FIELD_HOME),
    quota: mine.find((m) => m.field === FIELD_HOME_QUOTA),
  };
}

/** What the operator chose. `quota` is bytes, "0" for no limit, null when no home. */
export interface RoleChoice {
  group: string;
  home: boolean;
  quota: string | null;
}

export type RowOp =
  | { kind: "create"; field: string; value: string }
  | { kind: "edit"; field: string; value: string; row: RoleMapping }
  | { kind: "delete"; field: string; row: RoleMapping };

/** The row changes that take a role from what it has to what was chosen. */
export function opsFor(current: RoleRows, choice: RoleChoice | null): RowOp[] {
  const ops: RowOp[] = [];
  const want = (field: string, row: RoleMapping | undefined, value: string | null) => {
    if (value === null || value === "") {
      if (row) ops.push({ kind: "delete", field, row });
    } else if (!row) {
      ops.push({ kind: "create", field, value });
    } else if (row.value !== value) {
      ops.push({ kind: "edit", field, value, row });
    }
  };
  want(FIELD_GROUP, current.group, choice ? choice.group : null);
  want(FIELD_HOME, current.home, choice?.home ? "true" : null);
  want(FIELD_HOME_QUOTA, current.quota, choice?.home ? choice.quota : null);
  // Removals last and group last among them: a removal that stops part-way
  // then leaves the group row, which the list shows and can remove again.
  // Group first, it left home rows no list shows, still giving home folders.
  const removals = ops.filter((op) => op.kind === "delete").reverse();
  return [...ops.filter((op) => op.kind !== "delete"), ...removals];
}

/** How many roles a system's mappings name: one per role, whatever it gives. */
export function roleCount(mappings: RoleMapping[]): number {
  return new Set(mappings.map((m) => `${m.project_id}\u0000${m.role_key}`)).size;
}

export interface RehearsedPart {
  op: RowOp;
  plan: MappingRehearsal;
}

const RANK: Record<string, number> = { blocked: 3, failed: 3, apply: 2, queued: 1, no_change: 0, succeeded: 0 };

/**
 * One review from several rehearsals: each person once, with every change to
 * them said, and their strongest effect. The approval shown is the first
 * part's; each part keeps its own for the apply.
 */
export function mergePlans(parts: RehearsedPart[]): MappingRehearsal {
  const byUser = new Map<string, BulkOutcome>();
  for (const { plan } of parts) {
    for (const o of plan.outcomes) {
      const seen = byUser.get(o.user_id);
      if (!seen) {
        byUser.set(o.user_id, { ...o });
        continue;
      }
      if ((RANK[o.effect] ?? 0) > (RANK[seen.effect] ?? 0)) seen.effect = o.effect as BulkEffect;
      if (o.detail && !seen.detail.includes(o.detail)) seen.detail = seen.detail ? `${seen.detail}; ${o.detail}` : o.detail;
    }
  }
  const outcomes = Array.from(byUser.values());
  const count = (effect: string) => outcomes.filter((o) => o.effect === effect).length;
  const withPlan = parts.find((p) => p.plan.plan_id);
  return {
    op: parts[0]?.plan.op ?? "edit_mapping",
    plan_id: withPlan?.plan.plan_id,
    applied: false,
    outcomes,
    summary: {
      total: outcomes.length,
      apply: count("apply"),
      no_change: count("no_change"),
      blocked: count("blocked"),
      failed: 0,
      succeeded: 0,
      queued: 0,
    },
    value_checked: parts.every((p) => p.plan.value_checked !== false),
  };
}

/**
 * "Makes 34 home folders in main › home_dir; 2 people already have one there"
 * (A-02 §5), from the home row's own rehearsal.
 */
export function homeCount(parts: RehearsedPart[]): { makes: number; already: number } | null {
  const home = parts.find((p) => p.op.field === FIELD_HOME && p.op.kind === "create");
  if (!home || home.plan.outcomes.length === 0) return null;
  const makes = home.plan.outcomes.filter((o) => o.effect === "apply").length;
  return { makes, already: home.plan.outcomes.length - makes };
}

export async function rehearseOps(
  ops: RowOp[],
  role: { target: string; projectId: string; roleKey: string },
  acknowledgeScope: boolean,
): Promise<RehearsedPart[]> {
  const parts: RehearsedPart[] = [];
  for (const op of ops) {
    const plan =
      op.kind === "create"
        ? await rehearseMappingCreate({ ...role, field: op.field, value: op.value }, acknowledgeScope)
        : op.kind === "edit"
          ? await rehearseMappingEdit(op.row.id, op.value, acknowledgeScope)
          : ((await rehearseMappingDelete(op.row.id, acknowledgeScope)) as MappingRehearsal);
    parts.push({ op, plan });
  }
  return parts;
}

/**
 * Applies each part under its own approval; returns how many people have a
 * change queued. Each part queues one per holder, so it is the largest part's
 * count, never their sum: three parts for 34 people are 34 people waiting.
 *
 * A part that fails after others landed says so. The role is then part-way
 * changed, and only reopening it shows what it gives now.
 */
export async function applyParts(
  parts: RehearsedPart[],
  role: { target: string; projectId: string; roleKey: string },
  create: (input: { target: string; projectId: string; roleKey: string; field: string; value: string; planId?: string }) => Promise<{ queued_convergences: number }>,
): Promise<number> {
  let queued = 0;
  let done = 0;
  for (const { op, plan } of parts) {
    const planId = plan.plan_id ?? "";
    try {
      const result =
        op.kind === "create"
          ? await create({ ...role, field: op.field, value: op.value, planId })
          : op.kind === "edit"
            ? await applyMappingEdit(op.row.id, op.value, planId)
            : await applyMappingDelete(op.row.id, planId);
      queued = Math.max(queued, result.queued_convergences);
      done++;
    } catch (error) {
      if (done === 0) throw error;
      const why = sentence(describeFailure(error));
      throw new ApiError(error instanceof ApiError ? error.status : 500, {
        error: "PARTLY_SAVED",
        message: `${done} of ${parts.length} changes to this role were saved before one failed, so it is part-way changed. Close this and open the role again to see what it gives now. ${why}`,
      });
    }
  }
  return queued;
}

/** The merged plan as a result: queued, never applied. */
export function asQueued(plan: BulkPlan, queued: number): BulkPlan {
  return {
    op: plan.op,
    applied: true,
    outcomes: plan.outcomes.map((o) => ({ ...o, effect: "queued" as const })),
    summary: { ...plan.summary, apply: 0, failed: 0, succeeded: 0, queued },
  };
}

const GB = 1024 ** 3;

/** "20 GB", "no limit". */
export function limitWords(quota: string | undefined | null): string {
  if (!quota || quota === "0") return "no limit";
  const n = Number(quota) / GB;
  return `up to ${Number.isInteger(n) ? n : n.toFixed(1)} GB`;
}

export function gbToBytes(gb: string): string {
  const n = Number(gb);
  return Number.isFinite(n) && n > 0 ? String(Math.round(n * GB)) : "";
}

/** Exact, so reading a limit back and saving it unchanged changes nothing. */
export function bytesToGb(bytes: string | undefined): string {
  if (!bytes || bytes === "0") return "";
  return String(Number(bytes) / GB);
}
