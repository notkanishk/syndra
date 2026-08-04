"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { FilterPills, Select } from "@/components/ui/Select";
import { useProjects } from "@/lib/queries/useProjects";
import { ProjectName, UserAvatar, UserName } from "@/components/names";
import {
  useAttributeDrift,
  useBulkAttributeDrift,
  useBulkMarkExternalDrift,
  useDriftItems,
  useMarkExternalDrift,
  useReconcileNow,
  useRehearseAdoptDrift,
  useRehearseMarkExternalDrift,
  useRevokeDrift,
  type DriftTriageItem,
} from "@/lib/queries/useDrift";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import {
  RowCheckbox,
  SelectAllCheckbox,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { useRowSelection, type RowSelection } from "@/lib/useRowSelection";
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

  /**
   * Two filters, server-side, and deliberately not three.
   *
   * `source` is the one an operator asks for by name: a sweep-found row has no
   * actor to attribute — the sweep compares grant sets and genuinely cannot know
   * who — so "show me only what the sweep found" is "show me the ones I'll have
   * to judge without evidence". `project_id` scopes a queue to the thing that
   * went wrong, which is usually one project.
   *
   * `user_id` the backend also accepts, and this screen does NOT offer: "select
   * everything else for this person" is already on every row, works from the row
   * you are looking at, and doesn't ask anyone to find a name in a list of three
   * hundred. A select there would be a worse version of a control that exists.
   */
  const [source, setSource] = useState("");
  const [projectId, setProjectId] = useState("");
  const drift = useDriftItems({ source: source || undefined, project_id: projectId || undefined });
  const projects = useProjects();
  const filtered = Boolean(source || projectId);
  const reconcile = useReconcileNow();

  const [pending, setPending] = useState<{ item: DriftTriageItem; resolution: Resolution } | null>(
    null,
  );
  const [expanded, setExpanded] = useState<string | null>(null);
  const [limit, setLimit] = useState(PAGE);
  const [bulkOp, setBulkOp] = useState<"adopt" | "external" | null>(null);

  const items = useMemo(() => drift.data ?? [], [drift.data]);
  const visible = items.slice(0, limit);
  // Selection spans the whole queue, not the rendered page: a triage backlog is
  // exactly the case where paging four times before you can act is the tedium
  // worth removing.
  const selection = useRowSelection(useMemo(() => items.map((item) => item.id), [items]));
  const oldest = items.reduce<string | null>(
    (acc, item) => (!acc || item.detected_at < acc ? item.detected_at : acc),
    null,
  );

  const selectedIds = useMemo(() => Array.from(selection.selected), [selection.selected]);

  /**
   * Drift arrives in clusters — one misconfigured rule, one person onboarded by
   * hand, one project nobody told Syndra about. Selecting the cluster is the
   * actual shape of the work, and no amount of shift-clicking finds it as
   * reliably as asking for it.
   */
  const selectSimilar = useCallback(
    (item: DriftTriageItem) => {
      const kin = items.filter(
        (candidate) =>
          candidate.user_id === item.user_id || candidate.project_id === item.project_id,
      );
      selection.selectOnly(kin.map((candidate) => candidate.id));
    },
    [items, selection],
  );

  /**
   * What the selection is made of, not just how much of it there is.
   *
   * Safety-gated access is what the queue's ordering already keys on, so it is
   * what an operator needs to know before resolving twelve rows at once — a
   * batch of twelve wiki roles and a batch containing three laser-cutter roles
   * are not the same decision.
   */
  const composition = useMemo(() => {
    if (selection.count === 0) return "";
    const chosen = items.filter((item) => selection.selected.has(item.id));
    const safety = chosen.filter((item) =>
      (item.role_group ?? "").toLowerCase().includes("safety"),
    ).length;
    const people = new Set(chosen.map((item) => item.user_id)).size;
    const parts = [`${people} ${people === 1 ? "person" : "people"}`];
    if (safety > 0) parts.unshift(`${safety} safety-gated`);
    return parts.join(" · ");
  }, [items, selection]);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Unexplained access"
        meta={
          items.length > 0
            ? `${items.length} ${items.length === 1 ? "item" : "items"}${
                filtered ? " matching these filters" : ""
              }${oldest ? ` · oldest found ${formatRelative(oldest)}` : ""}`
            : "Access that exists in the identity provider which Syndra cannot explain."
        }
        actions={
          <>
            <Select
              value={projectId}
              onChange={(event) => setProjectId(event.target.value)}
              aria-label="Filter by project"
              className="w-[180px]"
            >
              <option value="">All projects</option>
              {(projects.data ?? []).map((entry) => (
                <option key={entry.project.id} value={entry.project.id}>
                  {entry.project.name}
                </option>
              ))}
            </Select>
            <FilterPills
              label="Filter by how it was found"
              value={source}
              onChange={setSource}
              options={[
                { value: "", label: "Any source" },
                { value: "webhook", label: "Caught live" },
                { value: "reconciliation_sweep", label: "Found by sweep" },
              ]}
            />
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
          </>
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
          <Card>
            <CardColumns>
              <span className="w-[18px]">
                <SelectAllCheckbox
                  label={
                    selection.allSelected
                      ? "Clear the selection"
                      : `Select all ${items.length} unexplained items`
                  }
                  {...selection.headerCheckboxProps}
                />
              </span>
              <span className="w-[186px]">Who</span>
              <span className="w-[250px]">What they can get into</span>
              <span className="flex-1">Why Syndra can&rsquo;t explain it</span>
              <span className="w-[96px]">Found</span>
              <span className="w-[300px] text-right">Resolve</span>
            </CardColumns>

            <div data-selection-scope {...selection.containerProps}>
            <ListStates
              isLoading={drift.isLoading}
              error={drift.error}
              isEmpty={items.length === 0}
              onRetry={() => drift.refetch()}
              errorTitle="Couldn't load the triage queue."
              skeleton={<RowSkeleton rows={5} label="Loading unexplained access" />}
              empty={
                // "Nothing here" and "nothing here that matches" are different
                // answers, and on this queue the difference is whether there is
                // unexplained access somewhere else that nobody is looking at.
                filtered ? (
                  <EmptyState
                    title="Nothing unexplained matches those filters."
                    guidance="There may still be items under another project, or from the other detection source."
                    action={{
                      label: "Clear filters",
                      onClick: () => {
                        setSource("");
                        setProjectId("");
                      },
                    }}
                  />
                ) : (
                  <EmptyState
                    title="Everything is explained."
                    guidance="Every grant in the identity provider traces back to something Syndra did."
                  />
                )
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
                  selection={selection}
                  onSelectSimilar={() => selectSimilar(item)}
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
            </div>
          </Card>

          <SelectionBar
            count={selection.count}
            noun={["item", "items"]}
            composition={composition}
            onClear={selection.clear}
          >
            <SelectionAction onClick={() => setBulkOp("adopt")}>Adopt in Syndra</SelectionAction>
            <SelectionAction onClick={() => setBulkOp("external")}>
              Mark as owned elsewhere
            </SelectionAction>
          </SelectionBar>

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

      {bulkOp && (
        <BulkResolutionDialog
          op={bulkOp}
          ids={selectedIds}
          composition={composition}
          onClose={() => setBulkOp(null)}
          onApplied={selection.clear}
        />
      )}
    </div>
  );
}


function TriageRow({
  item,
  leading,
  selection,
  onSelectSimilar,
  expanded,
  onExpand,
  onResolve,
}: {
  item: DriftTriageItem;
  leading: boolean;
  selection: RowSelection;
  onSelectSimilar: () => void;
  expanded: boolean;
  onExpand: () => void;
  onResolve: (resolution: Resolution) => void;
}) {
  const role = item.role_keys[0] ?? "";
  // A machine account is not a person: an integration that provisions itself
  // on every deploy will re-create this tomorrow whatever Syndra records, so
  // adopting is the wrong verb and is neutralised rather than hidden.
  const adoptPointless = item.user_is_service_account;

  return (
    <div className={leading ? "border-l-[3px] border-danger" : "border-l-[3px] border-transparent"}>
      <div
        className={`row-divider flex flex-wrap items-center gap-[18px] px-5 py-3.5 ${
          selection.isSelected(item.id) ? "bg-accent-soft/30" : ""
        }`}
        {...selection.rowProps(item.id)}
      >
        <RowCheckbox
          label={`Select this unexplained grant`}
          {...selection.checkboxProps(item.id)}
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
          {/* Drift arrives in clusters, so the fastest way to select the ones
              that belong together is to say so, not to hunt for them. */}
          <button
            type="button"
            onClick={onSelectSimilar}
            className="text-[12.5px] font-semibold text-muted underline-offset-2 transition-colors hover:text-accent-text hover:underline"
          >
            Select similar
          </button>
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
          Syndra records the grant, you become the granter of record, and it stops appearing here.
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
      title: "Adopt this access in Syndra?",
      lede: "Syndra takes ownership of it. The access stays exactly as it is; from now on Syndra explains it and manages its lifecycle.",
      confirm: "Adopt in Syndra",
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
      lede: "This removes the grant in the identity provider AND records the decision in Syndra, so the sweep won't surface it again.",
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
  if (item.drift_type === "syndra_only") {
    return "Syndra expects this grant but the identity provider doesn't have it — usually a queued write that never landed.";
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
 * Reconciliation — the Syndra ↔ provider diff, relocated here from the retired
 * /grants route. Two directions of drift, named differently: extra upstream
 * grants go to triage, missing downstream writes get re-pushed.
 */
function Reconciliation() {
  const diff = useReconciliationDiff();
  const data = diff.data;

  const empty =
    !data ||
    (data.only_in_syndra.length === 0 &&
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
          title="Syndra ↔ identity provider"
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
              guidance="Every grant Syndra expects exists upstream, and nothing upstream is unaccounted for."
            />
          }
        >
          <DiffSection
            label="Extra upstream"
            hint="These exist in the identity provider with nothing in Syndra explaining them."
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
            hint="Syndra expects these and the identity provider doesn't have them — a write that never landed."
            tone="warn"
            action={{ label: "Re-push →", href: "/governance/pending" }}
            rows={(data?.only_in_syndra ?? []).map((row) => ({
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
              roles: row.only_in_zitadel.length > 0 ? row.only_in_zitadel : row.only_in_syndra,
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

/**
 * Bulk adopt / bulk mark-as-external, rehearsed.
 *
 * The same dialog the People page opens for a bulk grant, because it is the
 * same decision shape: see what would happen to every selected row, then apply
 * it. Adopting writes ledger rows and marking-external suppresses future
 * detection — neither is trivially undone from here, and a triage queue is
 * exactly where an operator is moving fast.
 */
function BulkResolutionDialog({
  op,
  ids,
  composition,
  onClose,
  onApplied,
}: {
  op: "adopt" | "external";
  ids: string[];
  composition: string;
  onClose: () => void;
  onApplied: () => void;
}) {
  const rehearseAdopt = useRehearseAdoptDrift();
  const applyAdopt = useBulkAttributeDrift();
  const rehearseExternal = useRehearseMarkExternalDrift();
  const applyExternal = useBulkMarkExternalDrift();

  const adoptBody = { ids, source: "external_backfill" as const };
  const externalBody = { ids, reason: "Marked in bulk from triage" };

  return (
    <RehearsalDialog
      title={op === "adopt" ? "Adopt in Syndra" : "Mark as owned elsewhere"}
      lede={composition}
      noun={["item", "items"]}
      onRehearse={() =>
        op === "adopt"
          ? rehearseAdopt.mutateAsync(adoptBody)
          : rehearseExternal.mutateAsync(externalBody)
      }
      onApply={async () => {
        const plan =
          op === "adopt"
            ? await applyAdopt.mutateAsync(adoptBody)
            : await applyExternal.mutateAsync(externalBody);
        onApplied();
        return plan;
      }}
      onClose={onClose}
    />
  );
}
