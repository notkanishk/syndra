"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserAvatar, UserName } from "@/components/names";
import {
  useAttributeDrift,
  useBulkAttributeDrift,
  useBulkMarkExternalDrift,
  useDriftItems,
  useMarkExternalDrift,
  useReconcileNow,
  useRevokeDrift,
  type BulkDriftResult,
  type DriftTriageItem,
} from "@/lib/queries/useDrift";
import { useReconciliationDiff } from "@/lib/queries/useGrants";
import { Relative } from "@/components/ui/Time";
import { formatLongDate, formatRelative } from "@/lib/format";

type Tab = "triage" | "reconciliation";
type Resolution = "attribute" | "revoke" | "external";

/** Explicit pagination. The queue must have a visible end. */
const PAGE = 12;

/**
 * S6 · Review › Unexplained access — the highest-stakes screen in the product.
 *
 * Every row has to answer two questions in about two seconds: what is this, and
 * what happens if I revoke it. So the evidence sits ON the row rather than
 * behind a click, and the three resolutions are always in the same order —
 * Adopt, Revoke, Owned elsewhere — so a scanning eye learns one sequence.
 *
 * Ordering is by risk then age (computed server-side): a safety-gated role
 * found yesterday outranks a wiki role found last week. The row LAYOUT never
 * changes with risk; only the order and a left border do. A queue whose rows
 * rearrange their own shape is a queue nobody can scan.
 */
export function UnexplainedAccess() {
  const params = useSearchParams();
  const router = useRouter();
  const tab: Tab = params.get("tab") === "reconciliation" ? "reconciliation" : "triage";

  const drift = useDriftItems();
  const reconcile = useReconcileNow();
  const bulkAdopt = useBulkAttributeDrift();
  const bulkExternal = useBulkMarkExternalDrift();

  const [pending, setPending] = useState<{ item: DriftTriageItem; resolution: Resolution } | null>(
    null,
  );
  const [expanded, setExpanded] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [limit, setLimit] = useState(PAGE);

  const items = useMemo(() => drift.data ?? [], [drift.data]);
  const visible = items.slice(0, limit);
  const oldest = items.reduce<string | null>(
    (acc, item) => (!acc || item.detected_at < acc ? item.detected_at : acc),
    null,
  );

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  /**
   * Report what the batch actually did, not what was asked of it.
   *
   * A bulk resolution can partially fail — a row somebody else triaged a second
   * earlier, a write that did not land. Announcing the selected count regardless
   * tells an operator that twelve items are handled when eleven are, and the
   * twelfth is unexplained access nobody is going back to. The rows that failed
   * stay selected, so the next click retries exactly those.
   */
  function reportBulk(result: BulkDriftResult, succeeded: number, verb: string) {
    setSelected(new Set(result.failed_ids));

    if (result.failed === 0) {
      toast.success(`${succeeded} ${verb}.`);
      return;
    }
    if (succeeded === 0) {
      toast.error(`Nothing was ${verb} — all ${result.failed} failed. They are still selected.`);
      return;
    }
    toast.warning(
      `${succeeded} ${verb}. ${result.failed} failed and ${
        result.failed === 1 ? "is" : "are"
      } still selected — somebody may have resolved ${
        result.failed === 1 ? "it" : "them"
      } first.`,
    );
  }

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Unexplained access"
        meta={
          items.length > 0
            ? `${items.length} ${items.length === 1 ? "item" : "items"}${
                oldest ? ` · oldest found ${formatRelative(oldest)}` : ""
              }`
            : "Access that exists in the identity provider which MkAuth cannot explain."
        }
        actions={
          <Button
            isPending={reconcile.isPending}
            onClick={async () => {
              try {
                await reconcile.mutateAsync();
                toast.success("Scan finished.");
              } catch (error) {
                toast.error(error instanceof Error ? error.message : "The scan didn't run.");
              }
            }}
          >
            Compare again
          </Button>
        }
      />

      <div className="flex gap-2">
        {(["triage", "reconciliation"] as const).map((entry) => (
          <button
            key={entry}
            type="button"
            onClick={() => router.replace(entry === "triage" ? "?" : "?tab=reconciliation")}
            aria-current={tab === entry ? "page" : undefined}
            className={`rounded-pill px-4 py-2 text-[14.5px] transition-colors duration-150 ${
              tab === entry ? "bg-tint-3 font-semibold text-ink" : "text-muted hover:text-ink"
            }`}
          >
            {entry === "triage" ? (
              <>
                Triage queue{" "}
                <span className="font-semibold text-danger-text">{items.length}</span>
              </>
            ) : (
              "Reconciliation"
            )}
          </button>
        ))}
      </div>

      {tab === "triage" ? (
        <>
          {selected.size > 0 && (
            <BulkBar
              count={selected.size}
              busy={bulkAdopt.isPending || bulkExternal.isPending}
              onAdopt={async () => {
                try {
                  const result = await bulkAdopt.mutateAsync({
                    ids: Array.from(selected),
                    source: "external_backfill",
                  });
                  reportBulk(result, result.attributed ?? 0, "adopted in MkAuth");
                } catch (error) {
                  toast.error(error instanceof Error ? error.message : "That didn't go through.");
                }
              }}
              onMarkExternal={async () => {
                try {
                  const result = await bulkExternal.mutateAsync({
                    ids: Array.from(selected),
                    reason: "Marked in bulk from triage",
                  });
                  reportBulk(result, result.marked ?? 0, "marked as owned elsewhere");
                } catch (error) {
                  toast.error(error instanceof Error ? error.message : "That didn't go through.");
                }
              }}
            />
          )}

          <Card>
            <CardColumns>
              <span className="w-[18px]" />
              <span className="w-[186px]">Who</span>
              <span className="w-[250px]">What they can get into</span>
              <span className="flex-1">Why MkAuth can&rsquo;t explain it</span>
              <span className="w-[96px]">Found</span>
              <span className="w-[300px] text-right">Resolve</span>
            </CardColumns>

            <ListStates
              isLoading={drift.isLoading}
              error={drift.error}
              isEmpty={items.length === 0}
              onRetry={() => drift.refetch()}
              errorTitle="Couldn't load the triage queue."
              skeleton={<RowSkeleton rows={5} label="Loading unexplained access" />}
              empty={
                <EmptyState
                  title="Everything is explained."
                  guidance="Every grant in the identity provider traces back to something MkAuth did."
                />
              }
            >
              {visible.map((item, index) => (
                <TriageRow
                  key={item.id}
                  item={item}
                  // Only the leading row carries the ranking border. If every
                  // safety-gated row were marked, the marking would stop
                  // meaning "start here".
                  leading={index === 0 && (item.role_group ?? "").toLowerCase().includes("safety")}
                  checked={selected.has(item.id)}
                  onToggle={() => toggle(item.id)}
                  expanded={expanded === item.id}
                  onExpand={() => setExpanded((cur) => (cur === item.id ? null : item.id))}
                  onResolve={(resolution) => setPending({ item, resolution })}
                />
              ))}

              {items.length > visible.length && (
                <div className="row-divider flex items-center gap-4 px-5 py-3.5">
                  <span className="text-[13.5px] text-faint">
                    {items.length - visible.length} more
                  </span>
                  <button
                    type="button"
                    onClick={() => setLimit(items.length)}
                    className="rounded-pill border border-line-strong px-4 py-1.5 text-[13.5px] font-semibold transition-colors hover:bg-[var(--hover)]"
                  >
                    Show all {items.length}
                  </button>
                </div>
              )}
            </ListStates>
          </Card>

          {/*
            The absence is stated on the screen, not only in a spec. An operator
            who cannot find bulk revoke should learn it was a decision rather
            than assume the feature is broken.
          */}
          <div className="danger-note flex items-start gap-3.5 px-[18px] py-4">
            <span
              aria-hidden
              className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill bg-danger-soft text-[12px] font-bold text-danger-text"
            >
              !
            </span>
            <p className="max-w-[92ch] text-[14px] leading-[1.55] text-muted">
              <strong className="font-semibold text-ink">
                Bulk adopt and bulk mark-as-external exist; bulk revoke does not.
              </strong>{" "}
              Adopting is reversible bookkeeping. Revoking removes real access from real machines,
              and reading twelve consequences at once is not something anyone actually does — so
              revoke stays one row, one dialog, one decision.
            </p>
          </div>
        </>
      ) : (
        <Reconciliation />
      )}

      <ResolutionDialog pending={pending} onClose={() => setPending(null)} />
    </div>
  );
}

function BulkBar({
  count,
  busy,
  onAdopt,
  onMarkExternal,
}: {
  count: number;
  busy: boolean;
  onAdopt: () => void;
  onMarkExternal: () => void;
}) {
  return (
    <div className="card flex flex-wrap items-center gap-3 px-5 py-3.5">
      <span className="text-[14.5px] font-semibold">{count} selected</span>
      <Button variant="accent" size="sm" isPending={busy} onClick={onAdopt}>
        Adopt {count} in MkAuth
      </Button>
      <Button size="sm" isPending={busy} onClick={onMarkExternal}>
        Mark {count} as owned elsewhere
      </Button>
      <span className="text-[13px] text-faint">
        Bulk revoke is deliberately not offered — see below.
      </span>
    </div>
  );
}

function TriageRow({
  item,
  leading,
  checked,
  onToggle,
  expanded,
  onExpand,
  onResolve,
}: {
  item: DriftTriageItem;
  leading: boolean;
  checked: boolean;
  onToggle: () => void;
  expanded: boolean;
  onExpand: () => void;
  onResolve: (resolution: Resolution) => void;
}) {
  const role = item.role_keys[0] ?? "";
  // A machine account is not a person: an integration that provisions itself
  // on every deploy will re-create this tomorrow whatever MkAuth records, so
  // adopting is the wrong verb and is neutralised rather than hidden.
  const adoptPointless = item.user_is_service_account;

  return (
    <div className={leading ? "border-l-[3px] border-danger" : "border-l-[3px] border-transparent"}>
      <div className="row-divider flex flex-wrap items-center gap-[18px] px-5 py-3.5">
        <input
          type="checkbox"
          checked={checked}
          onChange={onToggle}
          aria-label={`Select this item for bulk resolution`}
          className="h-[18px] w-[18px] flex-none rounded-[5px] accent-[var(--accent)]"
        />

        <button
          type="button"
          onClick={onExpand}
          aria-expanded={expanded}
          className="flex w-[186px] min-w-0 items-center gap-2.5 text-left"
        >
          <UserAvatar id={item.user_id} size="row" />
          <span className="min-w-0">
            <span className="block truncate text-[14.5px] font-semibold">
              <UserName id={item.user_id} />
            </span>
            <span className="block truncate text-[12.5px] text-faint">
              {describeHolder(item)}
            </span>
          </span>
        </button>

        <div className="w-[250px] min-w-0">
          <div className="truncate text-[14px]">
            <ProjectName id={item.project_id} /> / <Mono>{role}</Mono>
          </div>
          <RiskPill item={item} />
        </div>

        <p className="min-w-[220px] flex-1 text-[13.5px] leading-[1.5] text-muted">
          {explainDrift(item)}
        </p>

        <div className="w-[96px] text-[13px] text-faint">
          <Relative iso={item.detected_at} />
        </div>

        {/*
          Fixed order, every row: Adopt · Revoke · Owned elsewhere. Revoke is a
          red OUTLINE here — the solid fill exists only on the confirming
          button inside its dialog.
        */}
        <div className="flex w-[300px] shrink-0 flex-wrap justify-end gap-2">
          <Button
            variant={adoptPointless ? "ghost" : "accent"}
            size="sm"
            disabled={adoptPointless}
            onClick={() => onResolve("attribute")}
          >
            Adopt
          </Button>
          <Button variant="danger" size="sm" onClick={() => onResolve("revoke")}>
            Revoke
          </Button>
          <Button size="sm" onClick={() => onResolve("external")}>
            Owned elsewhere
          </Button>
        </div>
      </div>

      {expanded && <ExpandedEvidence item={item} />}
    </div>
  );
}

/**
 * The risk pill says what it means in words as well as colour: amber and red
 * are load-bearing here, and colour is never allowed to be the only signal.
 */
function RiskPill({ item }: { item: DriftTriageItem }) {
  if (!item.role_in_catalogue) {
    return (
      <span className="mt-1 inline-block rounded-pill bg-tint-2 px-2.5 py-0.5 text-[12px] font-semibold text-muted">
        Role not in catalogue
      </span>
    );
  }
  if ((item.role_group ?? "").toLowerCase().includes("safety")) {
    return (
      <span className="mt-1 inline-block rounded-pill bg-danger-soft px-2.5 py-0.5 text-[12px] font-semibold text-danger-text">
        {item.role_group}
      </span>
    );
  }
  if (item.role_group) {
    return (
      <span className="mt-1 inline-block rounded-pill bg-tint-2 px-2.5 py-0.5 text-[12px] text-muted">
        {item.role_group}
      </span>
    );
  }
  return null;
}

/** Three columns: what a revoke costs, what an adopt records, and the evidence. */
function ExpandedEvidence({ item }: { item: DriftTriageItem }) {
  return (
    <div className="row-divider grid gap-5 bg-surface-0 px-5 py-4 md:grid-cols-3">
      <div>
        <div className="type-label mb-1.5">If you revoke</div>
        <p className="text-[13.5px] leading-[1.55] text-muted">
          The grant is removed in the identity provider. Whoever holds it loses this role at the
          next cache compile — usually within a minute.
        </p>
      </div>
      <div>
        <div className="type-label mb-1.5">If you adopt</div>
        <p className="text-[13.5px] leading-[1.55] text-muted">
          MkAuth records the grant, you become the granter of record, and it stops appearing here.
          Nothing changes upstream.
        </p>
      </div>
      <div>
        <div className="type-label mb-1.5">Evidence</div>
        <dl className="type-mono grid grid-cols-[84px_1fr] gap-x-3 gap-y-1 text-[12.5px] text-muted">
          <dt className="text-faint">grant_id</dt>
          <dd className="truncate">{item.zitadel_grant_id || "—"}</dd>
          <dt className="text-faint">created</dt>
          <dd>{item.upstream_created_at ? formatLongDate(item.upstream_created_at) : "unknown"}</dd>
          <dt className="text-faint">actor</dt>
          <dd className="truncate">{item.upstream_actor || "unknown"}</dd>
          <dt className="text-faint">last_seen</dt>
          <dd>{item.last_seen_at ? formatLongDate(item.last_seen_at) : "—"}</dd>
        </dl>
      </div>
    </div>
  );
}

/**
 * The three resolutions, each with its consequence stated. Only Revoke takes a
 * solid destructive fill, and only inside this dialog.
 */
function ResolutionDialog({
  pending,
  onClose,
}: {
  pending: { item: DriftTriageItem; resolution: Resolution } | null;
  onClose: () => void;
}) {
  const attribute = useAttributeDrift();
  const revoke = useRevokeDrift();
  const external = useMarkExternalDrift();

  if (!pending) return null;
  const { item, resolution } = pending;
  const busy = attribute.isPending || revoke.isPending || external.isPending;
  const role = item.role_keys[0] ?? "";

  const copy = {
    attribute: {
      title: "Adopt this access in MkAuth?",
      lede: "MkAuth takes ownership of it. The access stays exactly as it is; from now on MkAuth explains it and manages its lifecycle.",
      confirm: "Adopt in MkAuth",
      variant: "accent" as const,
    },
    external: {
      title: "Is this owned by another system?",
      lede: "The access stays and stops being flagged. Use this when a known integration legitimately manages it — not to quiet something you haven't identified.",
      confirm: "Owned elsewhere",
      variant: "accent" as const,
    },
    revoke: {
      title: "Take this access away?",
      lede: "This removes the grant in the identity provider AND records the decision in MkAuth, so the sweep won't surface it again.",
      confirm: "Revoke access",
      variant: "dangerConfirm" as const,
    },
  }[resolution];

  return (
    <Modal open onClose={onClose} busy={busy} size="sm" labelledBy="triage-title">
      <ModalHeader title={copy.title} titleId="triage-title" lede={copy.lede} />
      <div className="flex flex-col gap-3 px-6">
        <div className="rounded-inner bg-tint-1 px-4 py-3.5 text-[14.5px]">
          <UserName id={item.user_id} /> · <ProjectName id={item.project_id} /> / <Mono>{role}</Mono>
        </div>

        {resolution === "revoke" && (
          <>
            <div className="danger-note px-4 py-3.5 text-[14.5px] leading-[1.5]">
              <strong className="font-semibold text-danger-text">
                They will lose this role.
              </strong>
              <br />
              <span className="text-[14px] text-muted">
                Access stops at the next cache compile — usually within a minute.
              </span>
            </div>
            <p className="text-[13.5px] leading-[1.55] text-muted">
              If an integration re-creates the grant upstream, it reappears here tomorrow with a
              new evidence line.
            </p>
            {item.other_items_for_user > 0 && (
              <p className="text-[13.5px] leading-[1.55] text-muted">
                Before you decide: this person has{" "}
                <strong className="font-semibold text-ink">
                  {item.other_items_for_user} more{" "}
                  {item.other_items_for_user === 1 ? "item" : "items"}
                </strong>{" "}
                in this queue. One stray grant is a mistake; several is an offboarding that never
                ran.
              </p>
            )}
          </>
        )}
      </div>
      <ModalFooter>
        <Button
          variant={copy.variant}
          isPending={busy}
          onClick={async () => {
            try {
              if (resolution === "attribute") {
                await attribute.mutateAsync({ id: item.id, body: { source: "external_backfill" } });
              } else if (resolution === "revoke") {
                await revoke.mutateAsync({ id: item.id });
              } else {
                await external.mutateAsync({ id: item.id, body: {} });
              }
              toast.success("Resolved.");
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "That didn't go through.");
            }
          }}
        >
          {copy.confirm}
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}

/**
 * A full sentence naming the upstream actor and date where the detector knew
 * them. Where it did not — the reconciliation sweep compares grant sets and
 * genuinely cannot know who made a change — the row says so rather than
 * naming a plausible culprit.
 */
function explainDrift(item: DriftTriageItem): string {
  if (item.drift_type === "mkauth_only") {
    return "MkAuth expects this grant but the identity provider doesn't have it — usually a queued write that never landed.";
  }
  const when = item.upstream_created_at ? ` on ${formatLongDate(item.upstream_created_at)}` : "";
  const who = item.upstream_actor ? ` by ${item.upstream_actor}` : "";
  if (when || who) {
    return `Created in the identity provider${when}${who}. No direct grant, no bundle and no rule produces it.`;
  }
  return "Found in the identity provider by the reconciliation sweep, which compares grant lists and can't see who made the change. No direct grant, no bundle and no rule produces it.";
}

function describeHolder(item: DriftTriageItem): string {
  if (item.user_is_service_account) return "Service account";
  const status = (item.user_status ?? "").toLowerCase();
  if (status === "departed" || status === "alumni" || status === "inactive") return "No longer active";
  return item.user_status || "Member";
}

/**
 * Reconciliation — the MkAuth ↔ provider diff, relocated here from the retired
 * /grants route. Two directions of drift, named differently: extra upstream
 * grants go to triage, missing downstream writes get re-pushed.
 */
function Reconciliation() {
  const diff = useReconciliationDiff();
  const data = diff.data;

  const empty =
    !data ||
    (data.only_in_mkauth.length === 0 &&
      data.only_in_zitadel.length === 0 &&
      data.drift.length === 0);

  return (
    <div className="flex flex-col gap-[18px]">
      {data?.truncated && (
        <div className="warn-note px-5 py-3.5 text-[14px] text-warn-text">
          This comparison stopped early at the safety cap, so it is incomplete. Treat an empty
          section as &ldquo;nothing found so far&rdquo;, not as &ldquo;nothing exists&rdquo;.
        </div>
      )}

      <Card>
        <CardHeader
          title="MkAuth ↔ identity provider"
          note={
            data?.generated_at ? (
              <>
                compared <Relative iso={data.generated_at} />
              </>
            ) : undefined
          }
        />
        <ListStates
          isLoading={diff.isLoading}
          error={diff.error}
          isEmpty={empty}
          onRetry={() => diff.refetch()}
          errorTitle="Couldn't compare with the identity provider."
          skeleton={<RowSkeleton rows={4} avatar={false} label="Comparing" />}
          empty={
            <EmptyState
              title="The two sides agree."
              guidance="Every grant MkAuth expects exists upstream, and nothing upstream is unaccounted for."
            />
          }
        >
          <DiffSection
            label="Extra upstream"
            hint="These exist in the identity provider with nothing in MkAuth explaining them."
            tone="danger"
            action={{ label: "See in triage →", href: "?" }}
            rows={(data?.only_in_zitadel ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.role_keys,
            }))}
          />
          <DiffSection
            label="Missing downstream"
            hint="MkAuth expects these and the identity provider doesn't have them — a write that never landed."
            tone="warn"
            action={{ label: "Re-push →", href: "/governance/pending" }}
            rows={(data?.only_in_mkauth ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.role_keys,
            }))}
          />
          <DiffSection
            label="Different on both sides"
            hint="Same person and project, different role sets."
            tone="warn"
            rows={(data?.drift ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.only_in_zitadel.length > 0 ? row.only_in_zitadel : row.only_in_mkauth,
            }))}
          />

          {/*
            "Nothing wrong with the other four projects" is itself the answer
            somebody came here for, so agreement is rendered rather than
            filtered out — just at reduced contrast.
          */}
          {!empty && (
            <div className="row-divider px-5 py-3 text-[13.5px] text-faint opacity-[.55]">
              Everything not listed above is in agreement.
            </div>
          )}
        </ListStates>
      </Card>
    </div>
  );
}

function DiffSection({
  label,
  hint,
  tone,
  action,
  rows,
}: {
  label: string;
  hint: string;
  tone: "danger" | "warn";
  action?: { label: string; href: string };
  rows: Array<{ userId: string; projectId: string; roles: string[] }>;
}) {
  if (rows.length === 0) return null;
  return (
    <>
      <div className="row-divider flex flex-wrap items-baseline gap-3 px-5 pb-1.5 pt-3">
        <span className="type-label">{label}</span>
        <span
          className={`text-[13.5px] font-semibold ${
            tone === "danger" ? "text-danger-text" : "text-warn-text"
          }`}
        >
          {rows.length}
        </span>
        <span className="flex-1" />
        {action && (
          <ButtonLink href={action.href} size="sm" variant="ghost">
            {action.label}
          </ButtonLink>
        )}
        <p className="w-full max-w-[80ch] text-[13px] text-faint">{hint}</p>
      </div>
      {rows.map((row, index) => (
        <div
          key={`${row.userId}-${row.projectId}-${index}`}
          className="flex items-center gap-4 px-5 py-2.5 text-[14px]"
        >
          <span className="w-[200px] shrink-0 truncate font-semibold">
            <UserName id={row.userId} />
          </span>
          <span className="w-[180px] shrink-0 truncate text-muted">
            <ProjectName id={row.projectId} />
          </span>
          <span className="min-w-0 flex-1 truncate">
            {row.roles.map((key) => (
              <Mono key={key} className="mr-2 text-muted">
                {key}
              </Mono>
            ))}
          </span>
        </div>
      ))}
    </>
  );
}
