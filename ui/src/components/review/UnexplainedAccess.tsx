"use client";

import { Term } from "@/components/ui/Term";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { FilterPills, Select } from "@/components/ui/Select";
import { Tabs } from "@/components/ui/Tabs";
import { BULK_MAX_USERS } from "@/lib/queries/useBulkGrants";
import { useProjects } from "@/lib/queries/useProjects";
import { ProjectName, UserAvatar, UserName } from "@/components/names";
import {
  useAttributeDrift,
  useBulkAttributeDrift,
  useBulkMarkExternalDrift,
  useDriftItems,
  useDriftOrigin,
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
  SelectAllRow,
  SelectModeToggle,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { useRowSelection, type RowSelection } from "@/lib/useRowSelection";
import { useReconciliationDiff } from "@/lib/queries/useGrants";
import { Relative } from "@/components/ui/Time";
import { formatLongDate, formatRelative } from "@/lib/format";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";

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
  const [scanOutcome, setScanOutcome] = useState<Outcome | null>(null);

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
  // A triage row is read before it is resolved — the whole column headed "Why
  // Syndra can't explain it" is there to be read — so the checkboxes arrive
  // behind a named control rather than in front of every row by default.
  const [selecting, setSelecting] = useState(false);
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
        lede={
          <>
            <Term name="drift">Drift</Term> — access that exists in Zitadel that nothing in Syndra
            gave. Until you resolve a row, the person keeps that access.
          </>
        }
        meta={
          items.length > 0
            ? `${items.length} ${items.length === 1 ? "item" : "items"}${
                filtered ? " matching these filters" : ""
              }${oldest ? ` · oldest found ${formatRelative(oldest)}` : ""}`
            : undefined
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
                { value: "webhook", label: "Caught as it happened" },
                { value: "reconciliation_sweep", label: "Found by the scheduled check" },
              ]}
            />
            <Button
              isPending={reconcile.isPending}
              onClick={async () => {
                setScanOutcome(null);
                try {
                  await reconcile.mutateAsync();
                  // A scan is a read, so it applies nothing — saying "applied"
                  // about a comparison would be the queue claiming it had
                  // acted on what it found.
                  setScanOutcome({
                    kind: "no_change",
                    message: "Checked again",
                    detail: "The list below is what the check found just now.",
                  });
                } catch (error) {
                  setScanOutcome(outcomeFromError(error));
                }
              }}
            >
              Check Zitadel again
            </Button>
          </>
        }
      />

      {/* Under the control that ran it, above the queue it describes. */}
      {scanOutcome && <ActionOutcome outcome={scanOutcome} />}

      <div className="flex flex-wrap items-center gap-2">
        <Tabs
          label="Views of unexplained access"
          value={tab}
          onSelect={(next) => router.replace(next === "triage" ? "?" : "?tab=reconciliation")}
          options={[
            {
              value: "triage" as const,
              label: (
                <>
                  Needs a decision{" "}
                  <span className="font-semibold text-danger-text">{items.length}</span>
                </>
              ),
            },
            { value: "reconciliation" as const, label: "Side by side" },
          ]}
        />
        {tab === "triage" && items.length > 0 && (
          <>
            <span className="flex-1" />
            <SelectModeToggle active={selecting} onToggle={() => setSelecting((on) => !on)} />
          </>
        )}
      </div>

      {tab === "triage" ? (
        <>
          <p className="max-w-[92ch] text-[13.5px] leading-[1.5] text-muted">
            Adopt — keep it and let Syndra track it. Revoke — end it. Owned elsewhere — another
            system manages it; Syndra stops asking.
          </p>
          <Card>
            <CardColumns>
              {selecting && <span className="w-11 shrink-0 desktop:w-[18px]" />}
              <span className="w-[186px]">Who</span>
              <span className="w-[250px]">What they can get into</span>
              <span className="flex-1">Why Syndra can&rsquo;t explain it</span>
              <span className="w-[96px]">Found</span>
              <span className="w-[300px] text-right">Resolve</span>
            </CardColumns>

            {selecting && items.length > 0 && (
              <SelectAllRow
                inScope={items.length}
                noun={["item", "items"]}
                allSelected={selection.allSelected}
                {...selection.headerCheckboxProps}
              />
            )}

            <div data-selection-scope {...selection.containerProps}>
            <ListStates
              isLoading={drift.isLoading}
              error={drift.error}
              isEmpty={items.length === 0}
              onRetry={() => drift.refetch()}
              errorTitle="Couldn't load unexplained access."
              skeleton={<RowSkeleton rows={5} label="Loading unexplained access" />}
              empty={
                // "Nothing here" and "nothing here that matches" are different
                // answers, and on this queue the difference is whether there is
                // unexplained access somewhere else that nobody is looking at.
                filtered ? (
                  <EmptyState
                    title="Nothing unexplained matches those filters."
                    guidance="There may still be items under another project, or found the other way."
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
                    guidance="Every piece of access in Zitadel traces back to something Syndra did."
                    resolved
                  />
                )
              }
            >
              {visible.map((item, index) => (
                <TriageRow
                  selecting={selecting}
                  key={item.id}
                  item={item}
                  // Only the leading row carries the ranking border. If every
                  // safety-gated row were marked, the marking would stop
                  // meaning "start here".
                  leading={index === 0 && (item.role_group ?? "").toLowerCase().includes("safety")}
                  selection={selection}
                  // Selecting is what this does, so it turns the mode on rather
                  // than filling a selection nothing on screen is showing.
                  onSelectSimilar={() => {
                    setSelecting(true);
                    selectSimilar(item);
                  }}
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
                  {/* `Button`, not a hand-rolled pill. This was a `<button>`
                      carrying a copy of the outline variant's classes, and the
                      copy had already drifted: it pressed with `motion-tint`
                      instead of `motion-press press-scale`, and sat a pixel
                      short of every other control on the page. */}
                  <Button onClick={() => setLimit(items.length)}>
                    Show all {items.length}
                  </Button>
                </div>
              )}
            </ListStates>
            </div>
          </Card>

          <SelectionBar
            count={selecting ? selection.count : 0}
            noun={["item", "items"]}
            composition={composition}
            // `boundBulkIDs` refuses past this on both drift bulk routes. Same
            // server constant the grant and request endpoints use.
            ceiling={BULK_MAX_USERS}
            onTakeCeiling={() =>
              selection.selectOnly(items.slice(0, BULK_MAX_USERS).map((item) => item.id))
            }
            onClear={selection.clear}
          >
            {/* Revoke is missing from this bar on purpose, and its absence is
                stated where somebody would go looking for it. An operator who
                selects nine rows to revoke and finds two buttons that are not
                Revoke concludes the bar is broken, or hunts. Neither is a
                thing to leave anybody doing on this queue. */}
            <span className="text-[13px] text-faint">Revoking is one row at a time.</span>
            {/* Both open a plan; neither resolves anything on tap. */}
            <SelectionAction onClick={() => setBulkOp("adopt")}>
              Preview adopting these
            </SelectionAction>
            <SelectionAction onClick={() => setBulkOp("external")}>
              Preview marking these owned elsewhere
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
                You can adopt several rows at once, or mark several as owned elsewhere. Revoking
                is always one row at a time.
              </strong>{" "}
              Revoking ends real access on real machines, and each revocation deserves its own
              look.
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
  selecting,
  selection,
  onSelectSimilar,
  expanded,
  onExpand,
  onResolve,
}: {
  item: DriftTriageItem;
  leading: boolean;
  selecting: boolean;
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
        className={`row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-[18px] ${
          selection.isSelected(item.id) ? "bg-accent-soft/30" : ""
        }`}
        {...selection.rowProps(item.id)}
      >
        {selecting && (
          <RowCheckbox
            label={`Select the ${role} row`}
            {...selection.checkboxProps(item.id)}
          />
        )}

        <button
          type="button"
          onClick={onExpand}
          aria-expanded={expanded}
          className="flex min-h-[44px] w-full min-w-0 items-center gap-2.5 text-left tablet:w-[186px] desktop:min-h-0"
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

        <div className="w-full min-w-0 tablet:w-[250px]">
          <div className="truncate text-[14px]">
            <ProjectName id={item.project_id} /> / <Mono>{role}</Mono>
          </div>
          <RiskPill item={item} />
        </div>

        <div className="w-full tablet:min-w-[220px] tablet:flex-1">
          <p className="text-[13.5px] leading-[1.5] text-muted">{explainDrift(item)}</p>
          <WhoMadeIt item={item} />
        </div>

        <div className="text-[13px] text-faint tablet:w-[96px]">
          <span className="tablet:hidden">Found </span>
          <Relative iso={item.detected_at} />
        </div>

        {/*
          Fixed order, every row: Adopt · Revoke · Owned elsewhere. Revoke is a
          red OUTLINE here — the solid fill exists only on the confirming
          button inside its dialog.
        */}
        <div className="flex w-full flex-wrap gap-3 tablet:w-[300px] tablet:shrink-0 tablet:justify-end tablet:gap-2">
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
            className="inline-flex min-h-[44px] items-center text-[12.5px] font-semibold text-muted underline-offset-2 motion-tint hover:text-accent-text hover:underline desktop:min-h-6"
          >
            Select similar rows
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
  // Only a target whose access Syndra catalogues can have a role missing from
  // it. On one that has none, "not in catalogue" would be true of every row and
  // informative about none.
  if (item.role_catalogue_applies && !item.role_in_catalogue) {
    return <Badge className="mt-1">Role unknown to Syndra</Badge>;
  }
  if ((item.role_group ?? "").toLowerCase().includes("safety")) {
    return (
      <Badge tone="dangerSoft" className="mt-1">
        {item.role_group}
      </Badge>
    );
  }
  if (item.role_group) {
    // The same pill as the safety one in every respect but its tone. It used to
    // be a separate inline copy that had lost its `font-semibold`, so a role
    // group rendered bold or not depending on whether it was a safety group —
    // a weight difference that read as meaning and carried none.
    return <Badge className="mt-1">{item.role_group}</Badge>;
  }
  return null;
}

/** Three columns: what a revoke costs, what an adopt records, and the evidence. */
function ExpandedEvidence({ item }: { item: DriftTriageItem }) {
  return (
    <div className="row-divider grid gap-5 bg-surface-0 px-5 py-4 tablet:grid-cols-3">
      <div>
        <div className="type-label mb-1.5">If you revoke</div>
        <p className="text-[13.5px] leading-[1.55] text-muted">
          The access is removed from Zitadel as you confirm. An app they are already signed in to keeps what it was told until they sign in again.
        </p>
      </div>
      <div>
        <div className="type-label mb-1.5">If you adopt</div>
        <p className="text-[13.5px] leading-[1.55] text-muted">
          Syndra records this as access you gave, and the row stops appearing here. The
          person&rsquo;s access does not change.
        </p>
      </div>
      <div>
        <div className="type-label mb-1.5">Evidence</div>
        <dl className="type-mono grid grid-cols-[84px_1fr] gap-x-3 gap-y-1 text-[12.5px] text-muted">
          <dt className="text-faint">Zitadel id</dt>
          <dd className="truncate">{item.zitadel_grant_id || "—"}</dd>
          <dt className="text-faint">Created</dt>
          <dd>{item.upstream_created_at ? formatLongDate(item.upstream_created_at) : "unknown"}</dd>
          <dt className="text-faint">Made by</dt>
          <dd className="truncate">{item.upstream_actor || "unknown"}</dd>
          <dt className="text-faint">Last seen</dt>
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
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const revoke = useRevokeDrift();
  const external = useMarkExternalDrift();

  if (!pending) return null;
  const { item, resolution } = pending;
  const busy = attribute.isPending || revoke.isPending || external.isPending;
  const role = item.role_keys[0] ?? "";

  const copy = {
    attribute: {
      title: "Adopt this access in Syndra?",
      lede: "Syndra takes ownership of it. The access stays exactly as it is; from now on Syndra explains it and can extend, expire or revoke it like any other access.",
      confirm: "Adopt in Syndra",
      variant: "accent" as const,
    },
    external: {
      title: "Is this owned by another system?",
      lede: "The access stays and stops being listed here. Use this when a known system legitimately manages it, not to quiet something you have not identified.",
      confirm: "Mark as owned elsewhere",
      variant: "accent" as const,
    },
    revoke: {
      title: "Revoke this access?",
      lede: "This revokes the access in Zitadel (ends it there) and records your decision in Syndra, so the next check will not list it again.",
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
                The grant is removed from Zitadel as you confirm.
              </span>
            </div>
            <p className="text-[13.5px] leading-[1.55] text-muted">
              If another system adds this access back, it shows up here again after the next
              check.
            </p>
            {item.other_items_for_user > 0 && (
              <p className="text-[13.5px] leading-[1.55] text-muted">
                Before you decide: this person has{" "}
                <strong className="font-semibold text-ink">
                  {item.other_items_for_user} more{" "}
                  {item.other_items_for_user === 1 ? "item" : "items"}
                </strong>{" "}
                in this list. One stray role is a mistake; several is a leaver whose access was
                never ended.
              </p>
            )}
          </>
        )}
      </div>
      {/* The dialog states what it did, and for a revocation that is that
          nothing has reached the target yet — the access is still there until
          the drain runs. */}
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

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
              setOutcome(
                resolution === "revoke"
                  ? {
                      kind: "queued",
                      message: "Revocation recorded",
                      detail:
                        "Waiting to be sent to Zitadel; revocations send on their own, every few minutes. Until then the person still has the access.",
                    }
                  : { kind: "applied", message: "Resolved. It will not be listed again." },
              );
            } catch (error) {
              setOutcome(outcomeFromError(error));
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
/**
 * Who made it, asked one row at a time.
 *
 * A sweep-detected row cannot name an actor: the sweep compares grant SETS, and
 * a set difference has no author. The add-on targets solve the same problem
 * with a recorded merge base — remember what you last saw, infer who moved.
 * Zitadel needs no inference; it is event-sourced, and the event that created
 * the grant carries its editor. Reading it is strictly better than deducing it,
 * which is why this side has no merge base and this button instead.
 *
 * Behind a click rather than on load. The queue routinely holds dozens of rows
 * and this is one API call each; asking for all of them to render a page would
 * be a burst against the identity provider to answer a question nobody asked.
 *
 * Rendered only where it can help: a row Syndra already has an actor for says
 * so through `explainDrift`, and a row naming no grant has no event to read.
 */
function WhoMadeIt({ item }: { item: DriftTriageItem }) {
  const [asked, setAsked] = useState(false);
  const origin = useDriftOrigin(item.id, asked);

  if (item.upstream_actor || !item.zitadel_grant_id) return null;

  if (!asked) {
    return (
      <button
        type="button"
        onClick={() => setAsked(true)}
        className="mt-1 inline-flex min-h-[44px] items-center text-[12.5px] font-semibold text-muted underline-offset-2 motion-tint hover:text-accent-text hover:underline desktop:min-h-6"
      >
        Who made it?
      </button>
    );
  }
  if (origin.isLoading) {
    return <p className="mt-1 text-[12.5px] text-faint">Asking Zitadel…</p>;
  }

  const data = origin.data;
  // Three different answers, and none of them may render as another: the
  // lookup failed, the log does not go back that far, or the event names
  // nobody. Collapsing any pair would put a claim on the row that nothing
  // supports.
  if (origin.isError || !data || !data.readable) {
    return (
      <p className="mt-1 text-[12.5px] text-warn-text">
        Zitadel could not be asked. Somebody may still have made this; it is just not known who.
        {data?.detail ? ` ${data.detail}` : ""}
      </p>
    );
  }
  if (!data.recorded) {
    return (
      <p className="mt-1 text-[12.5px] text-faint">
        Zitadel&rsquo;s event log does not go back to when this was made.
      </p>
    );
  }
  if (!data.attributed) {
    return (
      <p className="mt-1 text-[12.5px] text-faint">
        Zitadel recorded the change and not who made it.
      </p>
    );
  }
  return (
    <p className="mt-1 text-[12.5px] text-muted">
      Made by{" "}
      <span className="font-semibold text-ink">
        {data.actor_name || (data.actor_id ? <UserName id={data.actor_id} /> : data.service)}
      </span>
      {data.actor_name && data.service ? (
        <span className="text-faint"> via {data.service}</span>
      ) : null}
      {data.at ? (
        <>
          {" "}
          <Relative iso={data.at} />
        </>
      ) : null}
      .
    </p>
  );
}

function explainDrift(item: DriftTriageItem): string {
  if (item.drift_type === "syndra_only") {
    // The row used to say "usually a queued write that never landed" for every
    // one of these, which is the wrong half of the story whenever the target was
    // seen HOLDING it: that is not a write that never happened, it is one
    // somebody undid. Told with its history, the row is recognisably the same
    // entitlement Syndra applied rather than a stranger.
    const p = item.provenance;
    // The write landing is the stronger evidence and, for a grant applied and
    // removed between two sweeps, the only evidence: nothing ever read it, so
    // the observation below does not exist. Told first for that reason.
    if (p?.applied_at) {
      const who = p.granted_by ? ` by ${p.granted_by}` : "";
      const why = p.reason ? ` — ${p.reason}` : "";
      const held = p.last_observed_at
        ? ` Zitadel was still holding it on ${formatLongDate(p.last_observed_at)}.`
        : "";
      const removedBy = item.upstream_actor ? ` Removed by ${item.upstream_actor}.` : "";
      return `Granted${who}${why}. Syndra applied it on ${formatLongDate(
        p.applied_at,
      )} and Zitadel accepted it.${held} It is not there now, so somebody removed it.${removedBy}`;
    }
    if (p?.last_observed_at) {
      const who = p.granted_by ? ` by ${p.granted_by}` : "";
      const when = p.granted_at ? ` on ${formatLongDate(p.granted_at)}` : "";
      const why = p.reason ? ` — ${p.reason}` : "";
      const removedBy = item.upstream_actor ? ` Removed by ${item.upstream_actor}.` : "";
      return `Granted${who}${when}${why}. Zitadel was still holding it on ${formatLongDate(
        p.last_observed_at,
      )} and does not now, so somebody removed it there.${removedBy}`;
    }
    if (p?.granted_at) {
      return `Granted${p.granted_by ? ` by ${p.granted_by}` : ""} on ${formatLongDate(
        p.granted_at,
      )}. Zitadel has never been seen holding it, so the change most likely never went through. Check Pending changes.`;
    }
    return "Syndra expects this access but Zitadel does not have it, and there is no record of it ever being held there.";
  }
  const when = item.upstream_created_at ? ` on ${formatLongDate(item.upstream_created_at)}` : "";
  const who = item.upstream_actor ? ` by ${item.upstream_actor}` : "";
  if (when || who) {
    return `Created in Zitadel${when}${who}. Nothing in Syndra — no direct access, no bundle, no automatic rule — gives it.`;
  }
  return "Found by the scheduled check, which compares lists and cannot see who made the change. Nothing in Syndra — no direct access, no bundle, no automatic rule — gives it.";
}

function describeHolder(item: DriftTriageItem): string {
  if (item.user_is_service_account) return "Service account";
  const status = (item.user_status ?? "").toLowerCase();
  if (status === "departed" || status === "alumni" || status === "inactive") return "No longer active";
  return "Member";
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
          This comparison stopped early because the list was too long, so it is incomplete. Treat an empty
          section as &ldquo;nothing found so far&rdquo;, not as &ldquo;nothing exists&rdquo;.
        </div>
      )}

      <Card>
        <CardHeader
          title="Syndra compared with Zitadel"
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
          errorTitle="Couldn't compare with Zitadel."
          skeleton={<RowSkeleton rows={4} avatar={false} label="Comparing" />}
          empty={
            <EmptyState
              title="The two sides agree."
              guidance="Everything Syndra gave exists in Zitadel, and nothing there is unaccounted for."
              resolved
            />
          }
        >
          <DiffSection
            label="In Zitadel but not in Syndra"
            hint="These exist in Zitadel with nothing in Syndra explaining them."
            tone="danger"
            action={{ label: "Open Needs a decision", href: "?" }}
            rows={(data?.only_in_zitadel ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.role_keys,
            }))}
          />
          <DiffSection
            label="In Syndra but not in Zitadel"
            hint="Syndra gave these and Zitadel does not have them — the change never went through."
            tone="warn"
            action={{ label: "Open Pending changes", href: "/governance/pending" }}
            rows={(data?.only_in_syndra ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.role_keys,
            }))}
          />
          <DiffSection
            label="Different on both sides"
            hint="Same person and project, different roles. Each row says which side has them."
            tone="warn"
            rows={(data?.drift ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.only_in_zitadel.length > 0 ? row.only_in_zitadel : row.only_in_syndra,
              side: row.only_in_zitadel.length > 0 ? "only in Zitadel" : "only in Syndra",
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
  rows: Array<{ userId: string; projectId: string; roles: string[]; side?: string }>;
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
            {row.side && <span className="mr-2 text-[13px] text-faint">{row.side}:</span>}
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
  const externalBody = { ids, reason: "Marked as owned elsewhere in bulk from Unexplained access" };

  return (
    <RehearsalDialog
      title={op === "adopt" ? "Adopt in Syndra" : "Mark as owned elsewhere"}
      lede={composition}
      noun={["item", "items"]}
      onRehearse={(acknowledgeScope) =>
        op === "adopt"
          ? rehearseAdopt.mutateAsync({ ...adoptBody, acknowledge_scope: acknowledgeScope })
          : rehearseExternal.mutateAsync({ ...externalBody, acknowledge_scope: acknowledgeScope })
      }
      onApply={async (planId) => {
        const plan =
          op === "adopt"
            ? await applyAdopt.mutateAsync({ ...adoptBody, plan_id: planId })
            : await applyExternal.mutateAsync({ ...externalBody, plan_id: planId });
        onApplied();
        return plan;
      }}
      onClose={onClose}
    />
  );
}
