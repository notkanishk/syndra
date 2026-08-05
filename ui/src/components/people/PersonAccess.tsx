"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { AccessSourceList, orderedSources, sourceQualifier, type RoleReason } from "@/components/access/AccessSource";
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
import { peopleHref } from "@/lib/people-filters";
import { useCrumb } from "@/lib/page-crumb";
import { useUpstreamUserGrants } from "@/lib/queries/useUpstream";
import { useUserAccess, useUserGrants } from "@/lib/queries/useUsers";
import { daysUntil, formatShortDate, humanizeKey } from "@/lib/format";
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

  // Zitadel's own grant ids, in Advanced only, and only for an operator — the endpoint behind
  // this is operator-gated, so asking as a member would be asking for a 403. Fetched lazily for
  // the same reason: a member's own page must not fire a request that can only fail.
  //
  // Keyed by project, because that is Zitadel's own shape. One grant per (user, project) carries
  // every role they hold there, so this id belongs on the project, not repeated onto each role
  // row as though each had its own.
  const upstreamGrants = useUpstreamUserGrants(advanced && isOperator ? userId : null);
  const zitadelGrantByProject = useMemo(
    () => new Map((upstreamGrants.data?.items ?? []).map((grant) => [grant.projectId, grant.id])),
    [upstreamGrants.data],
  );

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

  return (
    <div className="flex flex-col gap-[18px]">
      <div className="flex items-start gap-[22px]">
        <Avatar name={user.name} size="header" />
        <PageHeader
          className="flex-1"
          title={user.name}
          meta={
            <MetaRow>
              {[
                user.email,
                user.title || null,
                user.team || null,
                // The id stays reachable — this is the one page where an
                // operator genuinely needs it — but it sits last, after every
                // human-readable fact, and never stands in for a name.
                <Mono key="id" title="Zitadel user id">
                  {user.id}
                </Mono>,
                // Operator-only for the same reason the Activity tab is: /audit
                // is operator-gated, so a member following this link would land
                // on a page they cannot read.
                isOperator ? (
                  <Link
                    key="trail"
                    href={`/audit?user=${encodeURIComponent(user.id)}`}
                    className="font-semibold text-accent-text"
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
      <div className="flex gap-2">
        {tabsFor(isOperator).map((entry) => (
          <button
            key={entry}
            type="button"
            onClick={() => setTab(entry)}
            aria-current={tab === entry ? "page" : undefined}
            className={`rounded-pill px-4 py-2 text-[14.5px] motion-tint ${
              tab === entry ? "bg-tint-3 font-semibold text-ink" : "text-muted hover:text-ink"
            }`}
          >
            {TAB_LABELS[entry]}
          </button>
        ))}
      </div>

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
                <Chip>
                  {bundle.name}
                  {bundle.pinned_version ? (
                    <span className="text-faint">
                      {" "}
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
                className="text-[13.5px] font-semibold text-accent-text"
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

          {access.data.projects.map((project) => (
            <Card key={project.project_id}>
              <div className="flex flex-wrap items-center gap-3 px-5 py-4">
                <span className="type-card-title">{project.project_name}</span>
                <span className="text-[13.5px] text-faint">
                  {project.effective_role_keys.length}{" "}
                  {project.effective_role_keys.length === 1 ? "role" : "roles"}
                </span>
                {advanced && isOperator && (
                  <ZitadelGrantId
                    id={zitadelGrantByProject.get(project.project_id)}
                    loading={upstreamGrants.isLoading}
                    unreachable={Boolean(upstreamGrants.error)}
                  />
                )}
              </div>

              <RoleGroup
                label="Granted"
                roles={project.source_roles}
                projectId={project.project_id}
                projectName={project.project_name}
                grantsByRole={grantsByRole}
                advanced={advanced}
                isOperator={isOperator}
                onRemove={setRemoval}
                onReveal={revealInAdvanced}
              />
              <RoleGroup
                label="Automatic"
                roles={project.derived_roles}
                projectId={project.project_id}
                projectName={project.project_name}
                grantsByRole={grantsByRole}
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
          <RemovalDialog removal={removal} onClose={() => setRemoval(null)} />
        </>
      )}
    </div>
  );
}

/**
 * Zitadel's grant id for this project — the handle an operator needs to find the same grant in
 * Zitadel's own console, or to quote it in a ticket. Advanced only: in Basic, a raw identifier
 * next to a project name is noise around the thing that matters.
 *
 * Four states, and none of them guesses. An absent grant is stated as absent rather than shown
 * as a dash: Syndra listing roles for a project Zitadel has no grant for is a real condition,
 * and naming it is not the same as interpreting it — Reconciliation is where that gets triaged.
 */
function ZitadelGrantId({
  id,
  loading,
  unreachable,
}: {
  id: string | undefined;
  loading: boolean;
  unreachable: boolean;
}) {
  if (loading) return null;
  if (unreachable) {
    return (
      <span className="text-[13px] text-faint" title="Zitadel could not be read just now">
        Zitadel grant · unavailable
      </span>
    );
  }
  if (!id) {
    return (
      <span className="text-[13px] text-faint">
        Zitadel grant · none — see Review › Reconciliation
      </span>
    );
  }
  return (
    <span className="text-[13px] text-faint">
      Zitadel grant · <Mono title="Zitadel user-grant id">{id}</Mono>
    </span>
  );
}

function RoleGroup({
  label,
  roles,
  projectId,
  projectName,
  grantsByRole,
  advanced,
  isOperator,
  onRemove,
  onReveal,
}: {
  label: "Granted" | "Automatic";
  roles: AccessRole[];
  projectId: string;
  projectName: string;
  grantsByRole: Map<string, { id: string; expires_at?: string | null; granted_by: string }>;
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

        return (
          <div
            key={role.role_key}
            id={`role-${projectId}-${role.role_key}`}
            className="flex flex-wrap items-center gap-[18px] px-5 py-3"
          >
            <div className="w-[230px] shrink-0 text-[15px] font-semibold">
              {humanizeKey(role.role_key)}{" "}
              <Mono className="font-normal text-faint">{role.role_key}</Mono>
            </div>

            <div className="min-w-0 flex-1">
              <AccessSourceList reasons={role.reasons} />
            </div>

            <span className="shrink-0 text-[13.5px]">
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
                  })
                }
                className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-pill border border-line-strong text-[15px] leading-none text-muted motion-tint hover:text-ink"
              >
                ⋯
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
                      <dd>{grant.granted_by}</dd>
                    </>
                  )}
                </dl>
              </div>
            )}

            {/* No dead ends: Basic names the cause and offers one scoped jump. */}
            {!advanced && strongest?.kind === "mapping" && (
              <div className="w-full">
                <button
                  type="button"
                  onClick={() => onReveal(`role-${projectId}-${role.role_key}`)}
                  className="mt-1 inline-flex items-center gap-2 rounded-pill bg-accent-soft px-4 py-2 text-[13.5px] font-semibold text-accent-text"
                >
                  This came from an automatic rule — Open automation details →
                </button>
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
          return "a direct grant";
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
