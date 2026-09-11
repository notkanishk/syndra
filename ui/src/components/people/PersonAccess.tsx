"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import {
  AccessSourceList,
  NotSentYet,
  nothingSentYet,
  orderedSources,
  sourceQualifier,
  type RoleReason,
} from "@/components/access/AccessSource";
import { ResolvedProjectName } from "@/components/names";
import { UserName } from "@/components/names/UserName";
import { GrantDirectAccess } from "@/components/people/GrantDirectAccess";
import { ManageBundles } from "@/components/people/ManageBundles";
import { PersonActivity } from "@/components/people/PersonActivity";
import { PersonRequests } from "@/components/people/PersonRequests";
import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import { ErrorState, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Chip, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { Tabs } from "@/components/ui/Tabs";
import { Term } from "@/components/ui/Term";
import { Withheld } from "@/components/ui/Withheld";
import { peopleHref } from "@/lib/people-filters";
import { useCrumb } from "@/lib/page-crumb";
import { useUpstreamUserGrants } from "@/lib/queries/useUpstream";
import { useUserAccess, useUserGrants } from "@/lib/queries/useUsers";
import { daysUntil, formatClock, formatShortDate, humanizeKey } from "@/lib/format";
import { useIsAdvanced, useUiView } from "@/lib/ui-view";

type Tab = "access" | "requests" | "activity";

const TAB_LABELS: Record<Tab, string> = {
  access: "Access",
  requests: "Requests",
  activity: "Activity",
};

/**
 * Which tabs this viewer can actually use.
 *
 * A member reaching their own record gets Access and Requests — both are backed
 * by endpoints that accept self-reads. Activity is not: it reads the audit log,
 * which is operator-only, so rendering the tab for a member would put a control
 * on screen whose only possible outcome is an error.
 */
function tabsFor(isOperator: boolean): Tab[] {
  return isOperator ? ["access", "requests", "activity"] : ["access", "requests"];
}

/**
 * Whether Zitadel could be asked, and whether it answered.
 *
 * `unknown` is not `false`. A read that has not returned, and one this viewer
 * is not allowed to make, must never render as "the access is not there" — on
 * this screen that reading costs somebody their afternoon.
 */
type Confirmation = "read" | "unreachable" | "unknown";

interface AccessRole {
  role_key: string;
  reasons: RoleReason[];
}

/**
 * Grouped by project; Granted above Automatic inside each group, so the things
 * a human decided read first. Every row carries its source, and the overflow
 * on each row opens the removal that belongs to THAT source — there is never a
 * generic "revoke role".
 */
export function PersonAccess({ userId, isOperator }: { userId: string; isOperator: boolean }) {
  const access = useUserAccess(userId);
  const grants = useUserGrants(userId);
  const advanced = useIsAdvanced();
  const { revealInAdvanced } = useUiView();

  const [tab, setTab] = useState<Tab>("access");
  const [bundlesOpen, setBundlesOpen] = useState(false);
  const [grantOpen, setGrantOpen] = useState(false);
  const [removal, setRemoval] = useState<Removal | null>(null);

  const user = access.data?.user;
  useCrumb(user?.name);

  const multiSource = useMemo(() => findMultiSource(access.data?.projects ?? []), [access.data]);

  // Zitadel's own grants, read through the observation store. Keyed by
  // project, because that is Zitadel's own shape: one grant per (user,
  // project) carries every role they hold there, so a grant id belongs on the
  // project, not repeated onto each role row as though each had its own.
  //
  // Zitadel is the truth about who has access; Syndra's tables are the record
  // of what was decided. Gating the read on a view toggle meant Basic checked
  // nothing at all — the page stated access as fact and never asked the
  // system that would know. The route is self-or-operator, so a member reads
  // their OWN observed grants through this same pipe — the row-level
  // confirmation states below are exactly as true for them as for an
  // operator looking at somebody else.
  const upstreamGrants = useUpstreamUserGrants(userId);
  const zitadelGrantByProject = useMemo(
    () => new Map((upstreamGrants.data?.items ?? []).map((grant) => [grant.projectId, grant.id])),
    [upstreamGrants.data],
  );
  // The role keys, which the map above threw away. Per-PROJECT presence cannot
  // answer a question asked per role: a grant exists for the project while the
  // one role this row is about is missing from it, and the page said "Granted".
  const zitadelRolesByProject = useMemo(
    () =>
      new Map(
        (upstreamGrants.data?.items ?? []).map((grant) => [
          grant.projectId,
          new Set(grant.roleKeys ?? []),
        ]),
      ),
    [upstreamGrants.data],
  );
  // Three states, never two. "Zitadel does not have it" and "Syndra could not
  // ask" are different facts, and collapsing them would let an outage render as
  // an absence — the worst reading available, on the screen that decides
  // whether somebody has access.
  //
  // The endpoint now observes rather than listing Zitadel live (see
  // internal/observe), so a failed read no longer comes back as an HTTP
  // error — it comes back 200, with `complete: false` and whatever the store
  // last held. That is exactly the case this screen must not read as "read":
  // an incomplete observation is Syndra's own store, possibly stale, and a
  // role missing from it is not the same fact as Zitadel saying so just now.
  const confirmation: Confirmation =
    upstreamGrants.error || upstreamGrants.data?.complete === false
      ? "unreachable"
      : upstreamGrants.isLoading
        ? "unknown"
        : "read";
  // When the observation store's read happened — the basis for every "In
  // Zitadel" claim below. A positive mark with no read time attached is an
  // assertion with no evidence behind it.
  const readAt = upstreamGrants.data?.observedAt;

  if (access.isLoading) {
    return (
      <Card>
        <RowSkeleton rows={5} label="Loading this person's access" />
      </Card>
    );
  }
  if (access.error) {
    return (
      <ErrorState
        title="Couldn't load this person's access."
        error={access.error}
        onRetry={() => access.refetch()}
      />
    );
  }
  if (!access.data || !user) return null;

  const grantsByRole = new Map(
    (grants.data ?? []).map((grant) => [`${grant.project_id}::${grant.role_key}`, grant]),
  );
  // Whether that map is an ANSWER or just an empty default. This screen gates
  // its render on the access query alone, so `grants` is still in flight on
  // every early render and stays empty for ever if it fails — and an empty map
  // is indistinguishable from "this person has no direct grants" unless the
  // question is asked separately.
  const grantsResolved = !grants.isLoading && !grants.error;
  // Only the ones applying now. A lifted or lapsed hold belongs to the history
  // the Review queue keeps, not to what this person can reach.
  const inForce = (access.data.allowances ?? []).filter((a) => a.in_force);

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Stacked below `sm`, side by side above it. At phone width the avatar
          sitting left of the text column left email/id/lede/actions squeezed
          into a narrow remainder, with the two action buttons forced onto
          their own stacked rows inside it — full width for the text and
          actions here is what gives them room to wrap normally instead. */}
      <div className="flex flex-col items-start gap-4 sm:flex-row sm:gap-[22px]">
        <Avatar name={user.name} size="header" />
        <PageHeader
          className="w-full flex-1"
          title={user.name}
          lede="Everything this person can use, and where each piece of access came from. Change it here; Syndra keeps the record of why."
          meta={
            <MetaRow>
              {[
                user.email,
                user.title || null,
                user.team || null,
                // The id stays reachable — this is the one page where an
                // operator genuinely needs it — but it sits last, after every
                // human-readable fact, and never stands in for a name.
                <Mono key="id">
                  {user.id}
                </Mono>,
                // Operator-only for the same reason the Activity tab is: /audit
                // is operator-gated, so a member following this link would land
                // on a page they cannot read.
                isOperator ? (
                  <Link
                    key="trail"
                    href={`/audit?user=${encodeURIComponent(user.id)}`}
                    className="inline-flex min-h-11 items-center font-semibold text-accent-text desktop:min-h-6"
                  >
                    Full audit trail
                  </Link>
                ) : null,
              ]}
            </MetaRow>
          }
          actions={
            isOperator ? (
              <>
                <Button onClick={() => setBundlesOpen(true)}>Manage bundles</Button>
                <Button variant="accent" onClick={() => setGrantOpen(true)}>
                  Grant direct access
                </Button>
              </>
            ) : null
          }
        />
      </div>

      {/* Pill tabs, not underlines. Activity is operator-only: this same route
          serves a member looking at their own record, and the audit endpoint
          behind that tab is operator-gated, so offering it to a member would
          be offering a control that can only fail. */}
      <Tabs
        label="Views of this person's access"
        value={tab}
        onSelect={setTab}
        options={tabsFor(isOperator).map((entry) => ({
          value: entry,
          label: TAB_LABELS[entry],
        }))}
      />

      {tab === "requests" ? (
        <PersonRequests userId={userId} name={user.name} isOperator={isOperator} />
      ) : tab === "activity" && isOperator ? (
        <PersonActivity userId={userId} name={user.name} />
      ) : (
        <>
          {/* Bundle chips state membership and nothing else — no inline ✕.
              Removal lives behind Manage bundles, which shows the impact. */}
          <div className="panel flex flex-wrap items-center gap-3 px-5 py-4">
            <span className="type-label">Bundles</span>
            {access.data.bundles.length === 0 ? (
              <span className="text-[14px] text-faint">None assigned</span>
            ) : (
              // A chip that names a bundle should reach the people in it —
            // "who else has this?" is the next question every single time,
            // and it used to dead-end here.
            access.data.bundles.map((bundle) => (
              // Linking with the version narrows to the people on the SAME
              // version, which is the cohort question: "who else is still on
              // v2 with them".
              <Link
                key={bundle.id}
                href={peopleHref({
                  bundle: bundle.name,
                  version: bundle.pinned_version ? String(bundle.pinned_version) : "",
                })}
              >
                {/* `gap`, not a space. Chip is `inline-flex`, so the name and
                    the version are two flex items and the leading `{" "}`
                    inside the second one was collapsed away by flex layout —
                    which is why this rendered as "Ops Adminv1". A space
                    character cannot survive that boundary; the gap can. */}
                <Chip className="gap-[5px]">
                  {bundle.name}
                  {bundle.pinned_version ? (
                    <span className="text-faint">
                      v{bundle.pinned_version}
                      {bundle.latest_version && bundle.latest_version > bundle.pinned_version
                        ? ` · v${bundle.latest_version} available`
                        : ""}
                    </span>
                  ) : null}
                </Chip>
              </Link>
            ))
            )}
            <span className="flex-1" />
            {isOperator && (
              <button
                type="button"
                onClick={() => setBundlesOpen(true)}
                className="inline-flex min-h-11 items-center text-[13.5px] font-semibold text-accent-text desktop:min-h-6"
              >
                Manage bundles →
              </button>
            )}
          </div>

          {multiSource && (
            <div className="accent-note flex items-start gap-3 px-5 py-4">
              <span
                aria-hidden
                className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill bg-accent-soft text-[12px] font-bold text-accent-text"
              >
                i
              </span>
              <p className="text-[14.5px] leading-[1.55] text-ink/[.78]">
                <strong className="font-semibold text-ink">
                  {multiSource.projectName} / {multiSource.roleKey} is held twice
                </strong>{" "}
                — {multiSource.explanation}. Removing one would not remove this role.
              </p>
            </div>
          )}

          {/*
            The holds band, above the roles rather than below them. A
            role-holder list reads as full access, and a hold is precisely the
            case where it is not: the person holds the role and the entitlement
            it maps to is being withheld, by somebody, for a reason, until a
            date. §6's whole promise is that "why does this person have access
            to X" has one answer — this is the half that says they do not.

            The same component the member reads on their own page, in its
            operator voice. One object, and the two of them talking about it on
            the phone need it to be one object on screen too.
          */}
          {inForce.length > 0 && (
            <Card>
              <div className="px-5 py-4">
                <Withheld
                  audience={isOperator ? "operator" : "member"}
                  items={inForce.map((a) => ({
                    field: a.field,
                    value: a.value,
                    reason: a.reason,
                    target: a.target,
                    actorId: a.actor_id,
                    reviewDue: a.review_due,
                  }))}
                />
              </div>
            </Card>
          )}

          {access.data.projects.map((project) => (
            <Card key={project.project_id}>
              <div className="flex flex-wrap items-center gap-3 px-5 py-4">
                <ResolvedProjectName
                  className="type-card-title"
                  name={project.project_name}
                  resolved={project.project_name_resolved}
                  id={project.project_id}
                />
                <span className="text-[13.5px] text-faint">
                  {project.effective_role_keys.length}{" "}
                  {project.effective_role_keys.length === 1 ? "role" : "roles"}
                </span>
                {advanced && isOperator && (
                  <ZitadelGrantId
                    id={zitadelGrantByProject.get(project.project_id)}
                    readAt={readAt}
                    loading={upstreamGrants.isLoading}
                    unreachable={Boolean(upstreamGrants.error)}
                    // Absent because nothing has been sent yet is not absent
                    // because somebody changed Zitadel behind Syndra's back,
                    // and only the second is drift. Sending an operator to
                    // Drift for the first is a wrong turn that ends in an
                    // empty screen — worse than saying nothing, because they
                    // conclude the drift report is broken.
                    unsent={[...project.source_roles, ...project.derived_roles].every((role) =>
                      nothingSentYet(role.reasons),
                    )}
                  />
                )}
              </div>

              <RoleGroup
                confirmation={confirmation}
                inZitadel={zitadelRolesByProject.get(project.project_id)}
                readAt={readAt}
                label="Given"
                roles={project.source_roles}
                projectId={project.project_id}
                projectName={project.project_name}
                grantsByRole={grantsByRole}
                grantsResolved={grantsResolved}
                advanced={advanced}
                isOperator={isOperator}
                onRemove={setRemoval}
                onReveal={revealInAdvanced}
              />
              <RoleGroup
                confirmation={confirmation}
                inZitadel={zitadelRolesByProject.get(project.project_id)}
                readAt={readAt}
                label="Automatic"
                roles={project.derived_roles}
                projectId={project.project_id}
                projectName={project.project_name}
                grantsByRole={grantsByRole}
                grantsResolved={grantsResolved}
                advanced={advanced}
                isOperator={isOperator}
                onRemove={setRemoval}
                onReveal={revealInAdvanced}
              />
            </Card>
          ))}

          {/* Advisory notes. Never an error — cleanup_hints are opinions. */}
          {access.data.cleanup_hints.map((hint) => (
            <div key={hint} className="flex items-start gap-3 px-1 py-0.5">
              <span
                aria-hidden
                className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill border border-line-strong text-[12px] text-muted"
              >
                ?
              </span>
              <p className="max-w-[840px] text-[14px] leading-[1.55] text-faint">
                Advisory · {hint}
              </p>
            </div>
          ))}
        </>
      )}

      {isOperator && (
        <>
          <ManageBundles
            userId={userId}
            userName={user.name}
            assigned={access.data.bundles}
            open={bundlesOpen}
            onClose={() => setBundlesOpen(false)}
          />
          <GrantDirectAccess
            userId={userId}
            userName={user.name}
            open={grantOpen}
            onClose={() => setGrantOpen(false)}
          />
          {/*
            `userId` and `userName` are load-bearing, not decoration.
            RemovalDialog resolves the person as `removal.userId ?? userId`, and
            this page passed NEITHER — so `person` was undefined, every removal
            button was disabled, and the bundle dialog rendered its fallback
            title ("...from this person"). The only way into the removal flow
            for a role, on the page an operator actually uses, could not be
            pressed. The role page has always passed it, which is why the flow
            looked fine there.
          */}
          <RemovalDialog
            removal={removal}
            userId={userId}
            userName={user?.name}
            onClose={() => setRemoval(null)}
          />
        </>
      )}
    </div>
  );
}

/**
 * What Zitadel showed for this project — advanced only: in Basic, this line is noise around the
 * thing that matters.
 *
 * The truth is Zitadel, so the primary line states what it showed positively ("In Zitadel · read
 * 04:53"), not a raw grant id — an id on its own answers "what is it called", never "is it
 * there". The id is still the handle an operator needs for Zitadel's own console or a ticket, so
 * it stays, as a muted suffix behind the positive statement rather than the statement itself.
 *
 * Four states, and none of them guesses. An absent grant is stated as absent rather than shown
 * as a dash: Syndra listing roles for a project Zitadel has no grant for is a real condition,
 * and naming it is not the same as interpreting it — Reconciliation is where that gets triaged.
 */
function ZitadelGrantId({
  id,
  readAt,
  loading,
  unreachable,
  unsent,
}: {
  id: string | undefined;
  /** When this observation was taken — the basis the positive statement below cites. */
  readAt: string | undefined;
  loading: boolean;
  unreachable: boolean;
  /** Every role here is still queued, so Zitadel has not been told yet. */
  unsent: boolean;
}) {
  if (loading) return null;
  if (unreachable) {
    return (
      // The reason is the sentence, not a tooltip. This was the only
      // explanation anywhere for why the id is missing, and it was reachable
      // by hover alone — so on a phone the row said "unavailable" and nothing
      // else, which reads as a defect in Syndra rather than a read that failed.
      <span className="text-[13px] text-faint">
        Zitadel grant · unavailable — it could not be read just now
      </span>
    );
  }
  if (!id) {
    return (
      <span className="text-[13px] text-faint">
        {unsent
          ? "Not in Zitadel yet — these changes have not been sent"
          : "Not in Zitadel — see Drift › Side by side"}
      </span>
    );
  }
  return (
    <span className="text-[13px] font-semibold text-healthy">
      In Zitadel{readAt ? ` · read ${formatClock(readAt)}` : ""}{" "}
      <Mono className="font-normal text-faint">{id}</Mono>
    </span>
  );
}

/**
 * The word "Zitadel", introduced with its glossary popover for a member
 * reading the confirmation states on their OWN page for the first time — an
 * operator sees this word across a dozen other screens and does not need it
 * explained again here.
 */
function ZitadelWord({ isOperator }: { isOperator: boolean }) {
  return isOperator ? <>Zitadel</> : <Term name="zitadel">Zitadel</Term>;
}

function RoleGroup({
  label,
  confirmation,
  inZitadel,
  readAt,
  roles,
  projectId,
  projectName,
  grantsByRole,
  grantsResolved,
  advanced,
  isOperator,
  onRemove,
  onReveal,
}: {
  // "Given" (source_roles), not "Granted": the per-row state below is what
  // carries the truth about whether it landed. A hardcoded "Granted" sitting
  // above a row that says "Waiting to be sent" contradicted it directly.
  label: "Given" | "Automatic";
  /** Whether Zitadel could be asked at all, and what it said. */
  confirmation: Confirmation;
  /** The roles Zitadel reports for this project. Absent when it was not asked. */
  inZitadel: Set<string> | undefined;
  /** When that observation was taken — the basis "In Zitadel" cites. */
  readAt: string | undefined;
  roles: AccessRole[];
  projectId: string;
  projectName: string;
  grantsByRole: Map<string, { id: string; expires_at?: string | null; granted_by: string }>;
  /** Whether `grantsByRole` is an answer rather than an empty default. */
  grantsResolved: boolean;
  advanced: boolean;
  isOperator: boolean;
  onRemove: (removal: Removal) => void;
  onReveal: (panelId: string) => void;
}) {
  if (roles.length === 0) return null;

  return (
    <>
      <div className="row-divider px-5 pb-2 pt-1.5">
        <span className="type-label">{label}</span>
      </div>
      {roles.map((role) => {
        const grant = grantsByRole.get(`${projectId}::${role.role_key}`);
        const sources = orderedSources(role.reasons);
        const strongest = sources[0];
        const expires = grant?.expires_at ?? null;
        const remaining = daysUntil(expires);
        // Nothing that gives this role has left the queue, so the row is a
        // record of a decision and not a description of their access. It takes
        // the status slot outright: an expiry date on access that does not
        // exist yet is answering a question nobody can ask.
        const waiting = nothingSentYet(role.reasons);
        // What Zitadel says, which is the only thing that decides whether this
        // person can actually open the door. Syndra's tables say what was
        // DECIDED; they were being rendered as though they said what is true.
        const standing = confirmation === "read" ? Boolean(inZitadel?.has(role.role_key)) : null;

        return (
          <div
            key={role.role_key}
            id={`role-${projectId}-${role.role_key}`}
            className="flex flex-wrap items-center gap-[18px] px-5 py-3"
          >
            <div className="w-[230px] shrink-0 text-[15px] font-semibold">
              {humanizeKey(role.role_key)}
              {/* The raw key is an operator's handle for this role — a member
                  reading their own access has no use for it, and MemberAccess's
                  whole premise is that this screen carries no role keys. */}
              {isOperator && (
                <>
                  {" "}
                  <Mono className="font-normal text-faint">{role.role_key}</Mono>
                </>
              )}
            </div>

            <div className="min-w-0 flex-1">
              <AccessSourceList reasons={role.reasons} />
            </div>

            <span className="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-0.5 text-[13.5px]">
              {waiting ? (
                <NotSentYet />
              ) : standing === false ? (
                // Recorded here, absent there, and nothing queued to carry it.
                // This is the state the whole page exists to make visible: the
                // one time it happened, every element said "Granted" and only a
                // note at the top of the project said otherwise.
                <span className="font-semibold text-danger-text">
                  Not in <ZitadelWord isOperator={isOperator} />
                </span>
              ) : standing === null && confirmation === "unreachable" ? (
                <span className="text-warn-text">
                  Could not check <ZitadelWord isOperator={isOperator} />
                </span>
              ) : standing === null ? (
                // confirmation === "unknown": still loading, or the read has
                // not settled yet. Never nothing here — an unlabeled blank
                // reads as the row having no status at all, which is one more
                // wrong reading this page exists to rule out.
                <span className="text-faint">
                  Checking <ZitadelWord isOperator={isOperator} />…
                </span>
              ) : (
                <>
                  {/* The positive mark. The truth is Zitadel, so a role it
                      confirmed says so outright — not only silence where the
                      negative states would otherwise complain. */}
                  <span className="font-semibold text-healthy">
                    In <ZitadelWord isOperator={isOperator} />
                    {readAt ? ` · read ${formatClock(readAt)}` : ""}
                  </span>
                  {expires ? (
                    <span className="font-semibold text-warn-text">
                      Expires {formatShortDate(expires)}
                      {remaining !== null && remaining >= 0 ? ` · ${remaining} days` : ""}
                    </span>
                  ) : sources.length > 1 ? (
                    <span className="text-faint">Held {sources.length} ways</span>
                  ) : strongest?.kind === "mapping" ? (
                    <span className="text-faint">Nobody clicked this</span>
                  ) : (
                    <span className="text-faint">No expiry</span>
                  )}
                </>
              )}
            </span>

            {isOperator && strongest && (
              <button
                type="button"
                aria-label={`Actions for ${role.role_key}`}
                onClick={() =>
                  onRemove({
                    projectId,
                    projectName,
                    roleKey: role.role_key,
                    sources,
                    grantId: grant?.id,
                    // The grant list is a SECOND query, and this screen gates
                    // its render on the access query only. So `grant` is
                    // undefined on every early render and for ever if that
                    // fetch fails — and without this the dialog explained its
                    // own missing read as a fact about the operator's data.
                    grantsResolved,
                  })
                }
                // 44px of target around a 30px ring: this is the only way into the
                // removal flow for a role, so it is a destructive control and a
                // mis-tap costs most here. The ring stays 30px; the box around it
                // does not.
                className="flex h-11 w-11 shrink-0 items-center justify-center text-[15px] leading-none text-muted motion-tint hover:text-ink desktop:h-[30px] desktop:w-[30px]"
              >
                <span
                  aria-hidden
                  className="flex h-[30px] w-[30px] items-center justify-center rounded-pill border border-line-strong"
                >
                  ⋯
                </span>
              </button>
            )}

            {/* Advanced reveals lineage in place, on the same URL. */}
            {advanced && (
              <div className="w-full">
                <dl className="mt-1 grid grid-cols-[118px_1fr] gap-x-4 gap-y-1.5 rounded-block bg-tint-1 px-4 py-3 text-[13.5px]">
                  <dt className="text-faint">Grant id</dt>
                  <dd>
                    {grant ? (
                      <Mono className="text-muted">{grant.id}</Mono>
                    ) : (
                      <Mono className="text-muted">derived — no row in direct_grants</Mono>
                    )}
                  </dd>
                  {strongest?.kind === "mapping" && (
                    <>
                      <dt className="text-faint">Rule input</dt>
                      <dd>{sourceQualifier(strongest) ?? "—"}</dd>
                    </>
                  )}
                  {strongest?.kind === "bundle" && (
                    <>
                      <dt className="text-faint">Bundle</dt>
                      <dd>{strongest.bundle_name ?? strongest.bundle_id ?? "—"}</dd>
                    </>
                  )}
                  {grant?.granted_by && (
                    <>
                      <dt className="text-faint">Granted by</dt>
                      <dd>
                        <UserName id={grant.granted_by} />
                      </dd>
                    </>
                  )}
                </dl>
              </div>
            )}

            {/* No dead ends: Basic names the cause and offers one scoped jump. */}
            {!advanced && strongest?.kind === "mapping" && (
              <div className="w-full">
                <Button
                  variant="accentSoft"
                  size="sm"
                  className="mt-1"
                  onClick={() => onReveal(`role-${projectId}-${role.role_key}`)}
                >
                  This came from an automatic rule — Open automation details →
                </Button>
              </div>
            )}
          </div>
        );
      })}
    </>
  );
}

/**
 * The multi-source notice: a role held more than once, stated plainly above
 * the groups. Without it, removing a bundle looks like it will remove the role.
 */
function findMultiSource(
  projects: Array<{ project_name: string; source_roles: AccessRole[]; derived_roles: AccessRole[] }>,
) {
  for (const project of projects) {
    for (const role of [...project.source_roles, ...project.derived_roles]) {
      const sources = orderedSources(role.reasons);
      if (sources.length > 1) {
        const parts = sources.map((source) => {
          const qualifier = sourceQualifier(source);
          if (source.kind === "bundle") return `the ${qualifier ?? "assigned"} bundle`;
          if (source.kind === "mapping") return `an automatic rule${qualifier ? ` from ${qualifier}` : ""}`;
          return "direct access";
        });
        return {
          projectName: project.project_name,
          roleKey: role.role_key,
          explanation: `through ${parts.join(" and ")}`,
        };
      }
    }
  }
  return null;
}
