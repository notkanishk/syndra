"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";

import {
  AccessSourceList,
  NotSentYet,
  givenAt,
  nothingSentYet,
  orderedSources,
  sourceQualifier,
  type RoleReason,
} from "@/components/access/AccessSource";
import { ResolvedProjectName } from "@/components/names";
import { UserName } from "@/components/names/UserName";
import { AccountKind } from "@/components/people/AccountKind";
import { GIVE_ROLE_LABEL, GrantDirectAccess } from "@/components/people/GrantDirectAccess";
import { ManageBundles } from "@/components/people/ManageBundles";
import { PersonActivity } from "@/components/people/PersonActivity";
import { PersonRequests } from "@/components/people/PersonRequests";
import { PersonStorage } from "@/components/people/PersonStorage";
import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import { EmptyState, ErrorState, RowSkeleton, SkeletonAfterDelay } from "@/components/states";
import { ReadGate } from "@/components/states/ReadGate";
import { Avatar } from "@/components/ui/Avatar";
import { Chip, Mono } from "@/components/ui/Badge";
import { FoldChevron } from "@/components/ui/FoldChevron";
import { Notice } from "@/components/ui/Notice";
import { StatusSeat } from "@/components/ui/StatusSeat";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { KEPT, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { Crossfade } from "@/lib/motion";
import { ActionForecast, ActionOutcome } from "@/components/ui/ActionOutcome";
import { Card } from "@/components/ui/Card";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { ReadLine, SectionPill } from "@/components/ui/SectionNav";
import { Unfold } from "@/lib/motion";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { Tabs } from "@/components/ui/Tabs";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { Withheld } from "@/components/ui/Withheld";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { driftHref } from "@/lib/drift-filters";
import { peopleHref } from "@/lib/people-filters";
import { useCrumb } from "@/lib/page-crumb";
import { useSeededUser } from "@/lib/queries/seed";
import { useUpstreamUserGrants } from "@/lib/queries/useUpstream";
import { useCreateGrant, useUserAccess, useUserGrants } from "@/lib/queries/useUsers";
import { EXTEND_DAYS, FORECAST, outcomeFromError, outcomeFromExtend, statesNothingChanged, type ActionOutcome as Outcome } from "@/lib/outcome";
import { daysUntil, expiryLabel, formatAgoShort, formatShortDate, humanizeKey } from "@/lib/format";
import { useIsAdvanced, useUiView } from "@/lib/ui-view";
import { useApplications } from "@/lib/queries/useApplications";
import { usePersonSignIns } from "@/lib/queries/useSignIns";
import { useTargets } from "@/lib/queries/useTargets";
import { projectSignInLine } from "@/lib/sign-in-line";
import { sortByName } from "@/lib/sort";

type Tab = "access" | "requests" | "activity";

const PERSON_LEDE =
  "Everything this person can use, and where each piece of access came from. Change it here; Syndra keeps the record of why.";

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
/** A role as the catalog names it, the key made readable only when the catalog does not know it. */
function useRoleName(): (projectId: string, roleKey: string) => string {
  const resolver = useNameResolver();
  return (projectId, roleKey) => resolver.resolveRole(projectId, roleKey).value?.display_name || humanizeKey(roleKey);
}

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
  // A page opened from a row already knows the name (A-19 P1): the header
  // paints from the row, and only the access below it waits on the read.
  const seed = useSeededUser(userId);
  useCrumb(seed?.name);
  // The header is the row's own, so it is not behind the 200ms skeleton delay:
  // it is there on the first frame, and only the access below it waits (S7).
  if (access.isLoading && seed) {
    return (
      <div className="flex flex-col gap-4">
        <div className="flex flex-col items-start gap-4 sm:flex-row sm:gap-[22px]">
          <Avatar name={seed.name} size="header" />
          <PageHeader
            className="w-full flex-1"
            title={seed.name}
            lede={PERSON_LEDE}
            meta={<MetaRow>{[seed.email, seed.title || null, seed.team || null]}</MetaRow>}
          />
        </div>
        <SkeletonAfterDelay>
          <Card>
            <RowSkeleton rows={5} label="Loading this person's access" />
          </Card>
        </SkeletonAfterDelay>
      </div>
    );
  }
  // One hand-over (S7): a skeleton only if the read is slow, then the page arrives.
  return (
    <ReadGate
      loading={access.isLoading}
      skeleton={
        <Card>
          <RowSkeleton rows={5} label="Loading this person's access" />
        </Card>
      }
    >
      <ReadPersonAccess userId={userId} isOperator={isOperator} />
    </ReadGate>
  );
}

function ReadPersonAccess({ userId, isOperator }: { userId: string; isOperator: boolean }) {
  const access = useUserAccess(userId);
  const grants = useUserGrants(userId);
  const advanced = useIsAdvanced();
  const { revealInAdvanced } = useUiView();

  // The tab lives in the URL (?tab=), so a link, a reload and Back keep it.
  const asked = useSearchParams()?.get("tab");
  const fromUrl: Tab = tabsFor(isOperator).find((entry) => entry === asked) ?? "access";
  const [tab, pick] = useState<Tab>(fromUrl);
  const [seen, setSeen] = useState(fromUrl);
  if (seen !== fromUrl) {
    // The address itself changed (a link, Back): follow it. Our own write
    // below picks first, so the address catching up later changes nothing.
    setSeen(fromUrl);
    pick(fromUrl);
  }
  const setTab = useCallback((next: Tab) => {
    pick(next);
    // A replace, not a push: the tab is where you are, not somewhere you went.
    // Next folds a history.replaceState into its own router state.
    const url = new URL(window.location.href);
    if (next === "access") url.searchParams.delete("tab");
    else url.searchParams.set("tab", next);
    window.history.replaceState(window.history.state, "", url);
  }, []);
  const [bundlesOpen, setBundlesOpen] = useState(false);
  const [grantOpen, setGrantOpen] = useState(false);
  const [removal, setRemoval] = useState<Removal | null>(null);
  // Projects folded shut. Open by default: a person's page is read to find
  // something, and a fold that hid the answer would be the page's own fault.
  const [folded, setFolded] = useState<ReadonlySet<string>>(new Set());
  const toggleProject = (id: string) =>
    setFolded((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  const user = access.data?.user;
  useCrumb(user?.name);
  // Operators only, like the reads behind them (A-10 §1, §6).
  const signIns = usePersonSignIns(userId, isOperator);
  const apps = useApplications();
  const systems = useTargets(isOperator);

  const multiSource = useMemo(() => findMultiSource(access.data?.projects ?? []), [access.data]);
  const roleName = useRoleName();

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
  // When the observation store's read happened — the basis every "In Zitadel"
  // claim below rests on. Stated ONCE, on the read line above the cards,
  // because every role on the page was read from this same observation; a
  // clock repeated on each project or row is the same fact printed again and
  // crowds out the part that actually differs.
  const readAt = upstreamGrants.data?.observedAt;

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

  // One section per project, in the order the cards are drawn. Never the two
  // counts added together (ProjectRoleTally): what Syndra decided, or, with
  // none, what it never gave.
  const projectSections = access.data.projects.map((project) => ({
    id: `proj-${project.project_id}`,
    projectId: project.project_id,
    label: project.project_name,
    count: project.effective_role_keys.length || unexplainedIn(project).length,
    tone: unexplainedIn(project).length > 0 ? ("warn" as const) : undefined,
  }));
  // Then one per connected system, named and with no count: a system is one
  // card, not a list (A-10 §1).
  const sections = [
    ...projectSections,
    ...(isOperator ? (systems.data ?? []) : []).map((t) => ({
      id: `sys-${t.target}`,
      projectId: "",
      // The section heading below reads "Network storage"; the chip says the same.
      label: "Network storage",
      count: undefined,
      tone: undefined,
    })),
  ];
  const appList = (apps.data ?? []).map((a) => a.application);
  // A folded project opens on the way to it.
  const openProject = ({ projectId }: { projectId: string }) =>
    setFolded((prev) => {
      const next = new Set(prev);
      next.delete(projectId);
      return next;
    });

  return (
    <div className="flex flex-col gap-4">
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
          lede={PERSON_LEDE}
          meta={
            <MetaRow>
              {[
                user.email,
                user.title || null,
                user.team || null,
                // Advanced only: the board's Basic header is name, email, role
                // and team. When it is shown it sits last, after every
                // human-readable fact, and never stands in for a name.
                // An inline bit, not a region: it tints in with the view. The meta
                // row drops a falsy item and its dot together, so it goes at once.
                advanced ? (
                  <span key="id" className="tint-in">
                    <CopyableValue inline value={user.id} label="Zitadel ID" />
                  </span>
                ) : null,
              ]}
            </MetaRow>
          }
          actions={
            isOperator ? (
              <>
                <Button onClick={() => setBundlesOpen(true)}>Manage bundles</Button>
                <Button variant="accent" onClick={() => setGrantOpen(true)}>
                  {GIVE_ROLE_LABEL}
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
          {confirmation === "unreachable" && access.data.projects.length > 0 && (
            <Notice tone="warn" title="Zitadel couldn’t be read just now.">
              What follows is Syndra’s own record, which may be out of date.
            </Notice>
          )}
          <Unfold open={advanced && isOperator} gap={16}>
            <AccountKind user={user} />
          </Unfold>
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
          </div>

          {multiSource && (
            // An explanation, so a caption under the heading and not a box (A-04 §7.1a).
            <p className="text-[14px] leading-[1.55] text-muted">
              <strong className="font-semibold text-ink">
                {multiSource.projectName} / {roleName(multiSource.projectId, multiSource.roleKey)} is held twice
              </strong>{" "}
              , {multiSource.explanation}. Removing one would not remove this role.
            </p>
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

          {access.data.projects.length === 0 && (
            <Card>
              <EmptyState
                title={isOperator ? `${user.name} holds no access yet` : "You hold no access yet"}
                guidance={`Access appears here once ${
                  isOperator ? "they are" : "you are"
                } given a bundle or direct access, or when a request ${
                  isOperator ? "of theirs" : "of yours"
                } is approved.`}
              />
            </Card>
          )}

          {/* The read line: how current the page is on the left, once, and a chip
              per project at the right end (RULES, grouped lists). Not drawn
              while the read is still out, or with nothing to read it about. */}
          {access.data.projects.length > 0 && !upstreamGrants.isLoading && (
            <>
              <ReadLine
                sections={sections}
                label="Projects on this page"
                onOpen={openProject}
                className="gap-x-4 px-5"
              >
                {(readAt || confirmation === "unreachable") && (
                  <ReadFreshness state={{ readAt }} subject="What Zitadel confirmed" />
                )}
              </ReadLine>
              {/* Once the read line has scrolled away, the same projects float at the top. */}
              <SectionPill
                sections={sections}
                label="Projects on this page"
                onOpen={openProject}
                gap={16}
              />
            </>
          )}

          {access.data.projects.map((project) => {
            const open = !folded.has(project.project_id);
            const signInLine = isOperator ? projectSignInLine(project.project_id, appList, signIns.data) : null;
            return (
              <Card key={project.project_id}>
                <div
                  id={`proj-${project.project_id}`}
                  className="flex scroll-mt-4 flex-wrap items-center gap-3 px-5 py-4"
                >
                  <button
                    type="button"
                    aria-expanded={open}
                    aria-label={`${open ? "Fold" : "Open"} ${project.project_name}`}
                    onClick={() => toggleProject(project.project_id)}
                    className="group -my-2 -ml-2 flex size-[var(--tap)] flex-none items-center justify-center"
                  >
                    <FoldChevron open={open} />
                  </button>
                  <ResolvedProjectName className="type-card-title" name={project.project_name} resolved={project.project_name_resolved} id={project.project_id} />
                  <ProjectRoleTally
                    recorded={project.effective_role_keys.length}
                    unexplained={unexplainedIn(project).length}
                  />
                  {/* Use beside the access it uses, where Revoke is decided (A-10 §6). */}
                  {signInLine && (
                    <span className="tint-in text-[13.5px] text-faint [text-wrap:pretty]">· {signInLine}</span>
                  )}
                  {/* When Zitadel was read is the read line's job now; what stays
                      here is what is specific to THIS project's grant. */}
                  <ZitadelGrantId
                    userId={userId}
                    projectId={project.project_id}
                    id={zitadelGrantByProject.get(project.project_id)}
                    isOperator={isOperator}
                    loading={upstreamGrants.isLoading}
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
                </div>

                <Unfold open={open}>
                  <RoleGroup
                    userId={userId}
                    confirmation={confirmation}
                    inZitadel={zitadelRolesByProject.get(project.project_id)}
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
                    userId={userId}
                    confirmation={confirmation}
                    inZitadel={zitadelRolesByProject.get(project.project_id)}
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
                  <UnexplainedRoles
                    roleKeys={unexplainedIn(project)}
                    projectId={project.project_id}
                    userId={userId}
                    isOperator={isOperator}
                  />
                </Unfold>
              </Card>
            );
          })}

          {/* Something they use, under the projects whose roles reach it (A-10 §1).
              Operators only: a member's own storage is on Network storage. */}
          {isOperator && <PersonStorage userId={userId} name={user.name} />}

          {/* Advisory notes. Never an error — cleanup_hints are opinions. */}
          {access.data.cleanup_hints.map((hint) => (
            <p key={hint} className="max-w-[840px] px-1 text-[14px] leading-[1.55] text-muted">
              {hint}
            </p>
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
 * Roles Zitadel holds on this project that no Syndra record explains.
 *
 * Both lists come from the same response, so this is a subtraction rather than
 * a second opinion: observed minus recorded. An empty result is only an empty
 * result once something has been observed — `observed_role_keys` is null until
 * then, and null subtracts to nothing rather than to "all of them are fine".
 */
function unexplainedIn(project: {
  effective_role_keys: string[];
  observed_role_keys?: string[] | null;
}): string[] {
  if (!project.observed_role_keys) return [];
  return project.observed_role_keys.filter((key) => !project.effective_role_keys.includes(key));
}

/**
 * What this project holds, in two numbers that mean different things.
 *
 * They are never added together. "3 roles" is what Syndra decided; anything
 * unexplained is access it did not, and a combined "4 roles" would hide exactly
 * the row an operator is here to find. The second number is only ever shown
 * when it is non-zero, so an ordinary project reads as one plain count.
 */
function ProjectRoleTally({ recorded, unexplained }: { recorded: number; unexplained: number }) {
  return (
    <span className="flex flex-wrap items-center gap-2 text-[13.5px]">
      {recorded > 0 || unexplained === 0 ? (
        <span className="text-faint">
          {recorded} {recorded === 1 ? "role" : "roles"}
        </span>
      ) : null}
      {unexplained > 0 && (
        <span className="font-semibold text-warn-text">
          {unexplained} {unexplained === 1 ? "role" : "roles"} Syndra never gave
        </span>
      )}
    </span>
  );
}

/**
 * The drift band, on the page of the person it concerns.
 *
 * This access is real — Zitadel holds it and they can use it right now — and
 * nothing in Syndra says who gave it or why. Before this band existed the
 * project rendered as an empty card reading "0 roles", which is the one
 * sentence that is flatly untrue about it.
 *
 * Deliberately not styled as an error: it is a question, not a fault, and the
 * answer might be "that was fine". Triage lives on Drift, and the link goes
 * there rather than offering a removal this screen cannot explain.
 */
function UnexplainedRoles({
  roleKeys,
  projectId,
  userId,
  isOperator,
}: {
  roleKeys: string[];
  projectId: string;
  /** Drift opens on this person's rows, not the whole project's. */
  userId: string;
  isOperator: boolean;
}) {
  const roleName = useRoleName();
  if (roleKeys.length === 0) return null;
  return (
    <div className="row-divider px-5 py-3.5">
      {/* A member is told nothing more: an unknown source is not theirs to read about (G17). */}
      {isOperator && (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <StatusSeat tone="warn" word="Not given by Syndra" />
          <span className="text-[13.5px] text-muted">In Zitadel, with no bundle, rule or grant behind it.</span>
        </div>
      )}
      <div className={`${isOperator ? "mt-2.5 " : ""}flex flex-wrap items-center gap-2`}>
        {sortByName(roleKeys, (key) => roleName(projectId, key), (key) => key).map((key) => (
          <span
            key={key}
            className="rounded-pill border border-line-strong px-2.5 py-1 text-[13.5px] font-semibold"
          >
            {roleName(projectId, key)}
          </span>
        ))}
        {isOperator && (
          <ArrowLink href={driftHref({ project: projectId, user: userId, role: roleKeys.length === 1 ? roleKeys[0] : "" })}>
            Decide on Drift
          </ArrowLink>
        )}
      </div>
    </div>
  );
}

/**
 * The project heading's line about Zitadel — the EVIDENCE, not the verdict.
 *
 * This used to repeat the row's own sentence: the heading said "In Zitadel ·
 * read 22:51" and every role beneath it said "In Zitadel · read 22:51" again,
 * so a card with two roles made the same claim three times and none of the
 * three said anything the others did not.
 *
 * Split by what each can actually know. A grant in Zitadel is per (user,
 * project) and carries every role, so the heading is the right place for whether
 * it exists at all. When Zitadel was read is true for the
 * whole page and is said once, on the read line. Whether one particular role is
 * in the grant differs per row, so that stays on the row.
 *
 * Four states, and none of them guesses. An absent grant is stated as absent
 * rather than shown as a dash: Syndra listing roles for a project Zitadel has
 * no grant for is a real condition, and naming it is not the same as
 * interpreting it — Drift is where that gets triaged.
 */
function ZitadelGrantId({
  userId,
  projectId,
  id,
  isOperator,
  loading,
  unsent,
}: {
  /** Drift's Side by side opens on this person and project. */
  userId: string;
  projectId: string;
  id: string | undefined;
  isOperator: boolean;
  loading: boolean;
  /** Every role here is still queued, so Zitadel has not been told yet. */
  unsent: boolean;
}) {
  if (loading) return null;
  if (!id) {
    if (unsent) return <span className="text-[13px] text-faint">Not sent to Zitadel yet</span>;
    if (isOperator) {
      return <ArrowLink href={driftHref({ project: projectId, user: userId })}>Side by side on Drift</ArrowLink>;
    }
    return <span className="text-[13px] text-faint">No grant in Zitadel yet</span>;
  }
  // The grant id lives in the role's lineage, never in a heading (owner, A-17 Q6).
  return null;
}

/** The one window for ending access (A-04 §4): a row inside it offers Extend. */
const EXTEND_WITHIN_DAYS = 14;

/**
 * Extend, in the seat beside a row's end date (A-07 §19), the same write Expiring access
 * and Home make: re-submitting the grant with a later date renews it in place. The result
 * takes the button's seat until the next read; a refusal leaves the button beside it.
 */
function ExtendSeat({
  userId,
  projectId,
  projectName,
  roleKey,
}: {
  userId: string;
  projectId: string;
  projectName: string;
  roleKey: string;
}) {
  const extend = useCreateGrant(userId);
  const roleName = useRoleName();
  // No sheet of its own: a small one holds the step if extending would cancel a change still
  // waiting for you to send (A-04 §14).
  const step = useCancelStep({ host: "Extend this access", goAhead: "extend", standalone: true });
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const done = outcome !== null && !statesNothingChanged(outcome.kind);
  return (
    <Crossfade id={done ? "outcome" : extend.isPending ? "forecast" : "open"}>
      {extend.isPending && !done ? (
        <ActionForecast word={`${FORECAST.extend} by ${EXTEND_DAYS} days`} />
      ) : outcome && done ? (
        <ActionOutcome placement="seat" word={outcome.message} outcome={outcome} person={userId} />
      ) : (
    <>
      <Button
        size="sm"
        variant="outline"
        isPending={extend.isPending}
        aria-label={`Extend ${roleName(projectId, roleKey)} in ${projectName}`}
        onClick={async () => {
          try {
            const result = await step.run(() =>
              extend.mutateAsync({
                project_id: projectId,
                role_key: roleKey,
                reason: "Extended from the person's page",
                duration_days: EXTEND_DAYS,
                extend: true,
              }),
            );
            if (result === KEPT) return;
            setOutcome(outcomeFromExtend(result, EXTEND_DAYS));
          } catch (error) {
            setOutcome(outcomeFromError(error));
          }
        }}
      >
        Extend
      </Button>
      {step.element}
    </>
      )}
    </Crossfade>
  );
}

function RoleGroup({
  userId,
  label,
  confirmation,
  inZitadel,
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
  userId: string;
  // "Given" (source_roles), not "Granted": the per-row state below is what
  // carries the truth about whether it landed. A hardcoded "Granted" sitting
  // above a row that says "Waiting for you to send" contradicted it directly.
  label: "Given" | "Automatic";
  /** Whether Zitadel could be asked at all, and what it said. */
  confirmation: Confirmation;
  /** The roles Zitadel reports for this project. Absent when it was not asked. */
  inZitadel: Set<string> | undefined;
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
  const catalog = useGlobalRoleCatalog();
  const roleName = useRoleName();
  if (roles.length === 0) return null;
  const waitsFor = (key: string) =>
    catalog.data?.some((r) => r.project_id === projectId && r.role_key === key && r.waits_for_send);

  return (
    <>
      <div className="row-divider px-5 pb-2 pt-1.5">
        <span className="type-label">{label}</span>
      </div>
      {sortByName(roles, (role) => roleName(projectId, role.role_key), (role) => role.role_key).map((role) => {
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
        const verb = strongest?.queued ? "Withdraw" : strongest?.kind === "direct" ? "Revoke" : "Remove";

        return (
          <div key={role.role_key} id={`role-${projectId}-${role.role_key}`}>
          <div className="flex flex-wrap items-center gap-[18px] px-5 py-3">
            <div className="w-[230px] shrink-0 text-[15px] font-semibold">
              {roleName(projectId, role.role_key)}
              {waitsFor(role.role_key) && <WaitsMark className="ml-2" />}
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
                <>
                  {givenAt(role.reasons) && (
                    <span className="text-muted">Given {formatAgoShort(givenAt(role.reasons))}</span>
                  )}
                  <NotSentYet person={userId} />
                </>
              ) : standing === false ? (
                // Recorded here, absent there, and nothing queued to carry it.
                // This is the state the whole page exists to make visible: the
                // one time it happened, every element said "Granted" and only a
                // note at the top of the project said otherwise.
                <StatusSeat tone="danger" word="Not in Zitadel" />
              ) : standing === null && confirmation === "unreachable" ? (
                <StatusSeat tone="warn" word="Could not check Zitadel" />
              ) : standing === null ? (
                // confirmation === "unknown": still loading, or the read has
                // not settled yet. Never nothing here — an unlabeled blank
                // reads as the row having no status at all, which is one more
                // wrong reading this page exists to rule out.
                <StatusSeat tone="neutral" word="Checking Zitadel…" />
              ) : (
                <>
                  {/* The positive mark. The truth is Zitadel, so a role it
                      confirmed says so outright, not only silence where the
                      negative states would otherwise complain. The read time
                      is on the read line: every role was read from the same
                      observation. */}
                  <StatusSeat tone="healthy" word="In Zitadel" />
                  {expires ? (
                    <span className="font-semibold text-warn-text">
                      Ends {formatShortDate(expires)}
                      {remaining !== null && remaining > 0 ? ` · ${expiryLabel(expires, "short")}` : ""}
                    </span>
                  ) : sources.length > 1 ? (
                    <span className="text-faint">Held {sources.length} ways</span>
                  ) : strongest?.kind === "mapping" ? (
                    <span className="text-faint">Nobody clicked this</span>
                  ) : (
                    <span className="text-faint">{grantsResolved ? "No end date" : "End date unavailable"}</span>
                  )}
                </>
              )}
            </span>

            {isOperator && grant && remaining !== null && remaining >= 0 && remaining <= EXTEND_WITHIN_DAYS && (
              <ExtendSeat
                userId={userId}
                projectId={projectId}
                projectName={projectName}
                roleKey={role.role_key}
              />
            )}

            {isOperator && strongest && (
              // The verb is the label (RULES): Revoke when a person loses access
              // they hold, Withdraw while the change is still waiting to be
              // sent, Remove when a part comes out of a whole. It opens the
              // removal that belongs to this source; the dialog still decides
              // what is left and names it.
              <Button
                size="sm"
                variant={verb === "Withdraw" ? "outline" : "danger"}
                aria-label={`${verb} ${roleName(projectId, role.role_key)} in ${projectName}`}
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
              >
                {verb}
              </Button>
            )}

          </div>
          {/* Advanced reveals lineage in place, on the same URL, by Unfold (A-08
              §3). Beside the row rather than inside its wrapping line, whose
              18px row gap is outside anything that could fold: here the space
              above it is its own (`pt-2.5`, with the row's own `py-3`). */}
          <Unfold open={advanced}>
            <div className="px-5 pb-3 pt-2.5">
              <div className="w-full">
                <dl className="grid grid-cols-[118px_1fr] gap-x-4 gap-y-1.5 text-[13.5px]">
                  <dt className="text-faint">Grant id</dt>
                  <dd>
                    {grant ? (
                      <Mono className="text-muted">{grant.id}</Mono>
                    ) : (
                      <Mono className="text-muted">derived, no row in direct_grants</Mono>
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
                      <dt className="text-faint">Given by</dt>
                      <dd>
                        <UserName id={grant.granted_by} />
                      </dd>
                    </>
                  )}
                </dl>
              </div>
            </div>
          </Unfold>

          {/* No dead ends: Basic names the cause and offers one scoped jump. It
              leaves the way the lineage arrives, when Advanced opens. */}
          <Unfold open={!advanced && strongest?.kind === "mapping"}>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-5 pb-3 pt-2.5">
              <span className="text-[13px] text-muted">This came from an automatic rule.</span>
              <Button size="sm" onClick={() => onReveal(`role-${projectId}-${role.role_key}`)}>
                Show the rule
              </Button>
            </div>
          </Unfold>
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
  projects: Array<{ project_id: string; project_name: string; source_roles: AccessRole[]; derived_roles: AccessRole[] }>,
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
          projectId: project.project_id,
          projectName: project.project_name,
          roleKey: role.role_key,
          explanation: `through ${parts.join(" and ")}`,
        };
      }
    }
  }
  return null;
}
