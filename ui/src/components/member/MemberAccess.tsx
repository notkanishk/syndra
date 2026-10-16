"use client";

import { ResolvedProjectName } from "@/components/names";
import { MemberCatalog } from "@/components/member/MemberCatalog";
import { RecentChanges, askHref } from "@/components/member/RecentChanges";
import { EmptyState, ErrorState, RowSkeleton } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card } from "@/components/ui/Card";
import {
  nothingSentYet,
  orderedSources,
  sourceQualifier,
  type RoleReason,
} from "@/components/access/AccessSource";
import { Withheld } from "@/components/ui/Withheld";
import { useCatalogProjects } from "@/lib/queries/useCatalogUsers";
import { useMyChanges, type MyChange } from "@/lib/queries/useMyChanges";
import { useUserAccess, useUserGrants } from "@/lib/queries/useUsers";
import type { SessionUser } from "@/lib/session";
import { daysUntil, formatShortDate, humanizeKey } from "@/lib/format";

/**
 * Member · My access (route-me, 52b + 52c). The landing route for members, and
 * the one screen that explains to somebody why they can badge into the laser
 * bay. What just changed leads; then what you have, each role with why; then
 * everything else on offer.
 *
 * Access source becomes a SENTENCE, not a chip: a member has no vocabulary to
 * attach to Direct / Via bundle / Automatic. No jargon anywhere on this screen
 * — no "derived", no effective_role_keys, no role keys.
 */
export function MemberAccess({ session }: { session: SessionUser }) {
  const access = useUserAccess(session.id);
  const grants = useUserGrants(session.id);
  const changes = useMyChanges();
  const catalog = useCatalogProjects();
  // What Zitadel holds for this person arrives WITH the records that explain
  // it, in one response. It used to be a second call, which meant this screen
  // could render a headline from Syndra's records against rows marked from
  // Zitadel's — two answers to one question, disagreeing in front of the
  // person whose access it is.
  const observation = access.data?.observation;
  const standing = (project: (typeof projects)[number], roleKey: string): Standing => {
    // Absent, not empty: nothing has been observed yet, so nothing may be
    // reported as missing.
    if (!project.observed_role_keys || !observation?.read_at) return "checking";
    if (project.observed_role_keys.includes(roleKey)) return "held";
    return observation.current === false || observation.truncated ? "unreachable" : "missing";
  };

  if (access.isLoading) {
    return (
      <Card>
        <RowSkeleton rows={4} avatar={false} label="Loading your access" />
      </Card>
    );
  }
  if (access.error) {
    return (
      <ErrorState
        title="Couldn't load your access."
        error={access.error}
        onRetry={() => access.refetch()}
      />
    );
  }

  const projects = access.data?.projects ?? [];
  // The headline counts what they HOLD, not what was decided for them. Summing
  // effective_role_keys put "You have four roles" above four rows each saying
  // the role had not reached Zitadel yet.
  const roleCount = access.data?.observed_role_count;
  const rooms = projects.filter((project) => (project.observed_role_keys ?? []).length > 0).length;

  const ending = (grants.data ?? [])
    .filter((grant) => grant.expires_at && (daysUntil(grant.expires_at) ?? -1) >= 0)
    .map((grant) => grant.expires_at!)
    .sort();

  // Labels from the catalogue the page already reads below; a role it doesn't
  // list falls back to its key, humanised.
  const roleName = (projectId: string, roleKey: string) =>
    catalog.data
      .find((project) => project.id === projectId)
      ?.roles.find((role) => role.key === roleKey)?.label || humanizeKey(roleKey);

  // An approval still on its way (N19), so its role's row can say so.
  const feed = changes.data ?? [];
  const waiting = new Map(
    feed
      .filter((change) => change.kind === "request_approved" && change.waiting)
      .map((change) => [`${change.project_id}/${change.role_key}`, change]),
  );

  // What they already hold, so the catalogue below can mark it rather than
  // offering them a second copy of something they have.
  const heldByProject = new Map(
    projects.map((project) => [
      project.project_id,
      new Set([...project.source_roles, ...project.derived_roles].map((role) => role.role_key)),
    ]),
  );

  // Only what still applies — a lifted or lapsed hold is history, not
  // something they need to read about on the page that answers "what can I
  // use right now". Same object PersonAccess shows an operator, member voice.
  const inForce = (access.data?.allowances ?? []).filter((allowance) => allowance.in_force);
  const summary = summarySentence(roleCount, rooms, ending);

  return (
    <div className="flex flex-col gap-[22px]">
      <div className="flex flex-col gap-2">
        <h1 className="type-page-title">
          Hi {firstName(session.name)}.{" "}
          <span className="text-faint">Here&rsquo;s what you can use.</span>
        </h1>
        {summary && <div className="text-[14.5px] text-faint">{summary}</div>}
      </div>

      {/* Leads when there's news; gives up its seat on a quiet month. Not
          drawn until the read lands, so nothing unread is shown. */}
      {feed.length > 0 && <RecentChanges items={feed} roleName={roleName} />}

      {inForce.length > 0 && (
        <Card>
          <div className="px-5 py-4">
            <Withheld
              items={inForce.map((allowance) => ({
                field: allowance.field,
                value: allowance.value,
                reason: allowance.reason,
                target: allowance.target,
                reviewDue: allowance.review_due,
              }))}
            />
          </div>
        </Card>
      )}

      {projects.length === 0 ? (
        <Card>
          {/* The screen that generates the support message, so it carries the
              move that resolves it rather than describing one. There is nobody
              to name: no project records an owner anywhere in the product, so
              this says what it can do instead of who to find. */}
          <EmptyState
            title="You don't have access to anything yet."
            guidance="If there's a machine or a space you need, ask for it and makerspace staff will decide. Everything the makerspace offers is listed below."
            action={{ label: "Ask for access", href: "/requests?ask" }}
          />
        </Card>
      ) : (
        <div className="flex flex-wrap gap-[18px]">
          {projects.map((project) => {
            const roles = [...project.source_roles, ...project.derived_roles]
              .map((role) => ({
                ...role,
                expiresAt: expiryFor(grants.data, project.project_id, role.role_key),
              }))
              // The role that runs out leads its card.
              .sort((a, b) => {
                const [x, y] = [a.expiresAt ?? "~", b.expiresAt ?? "~"];
                return x < y ? -1 : x > y ? 1 : 0;
              });
            return (
              <Card key={project.project_id} className="min-w-[min(420px,100%)] flex-1">
                <div className="px-5 py-4 type-card-title">
                  <ResolvedProjectName
                    name={project.project_name}
                    resolved={project.project_name_resolved}
                    id={project.project_id}
                  />
                </div>
                {roles.map((role) => (
                  <RoleRow
                    key={role.role_key}
                    name={roleName(project.project_id, role.role_key)}
                    projectId={project.project_id}
                    roleKey={role.role_key}
                    reasons={role.reasons}
                    expiresAt={role.expiresAt}
                    standing={standing(project, role.role_key)}
                    waiting={waiting.get(`${project.project_id}/${role.role_key}`)}
                  />
                ))}
                {/* Something they hold that Syndra has no reason for. The
                    operator's People page calls this "unexplained"; the person
                    holding it should not be the only one who cannot see it. */}
                {(project.observed_role_keys ?? [])
                  .filter((roleKey) => !project.effective_role_keys.includes(roleKey))
                  .map((roleKey) => (
                    <div key={`held-${roleKey}`} className="row-divider flex flex-col gap-0.5 px-5 py-3">
                      <span className="text-[15px] font-semibold">{roleName(project.project_id, roleKey)}</span>
                      <span className="text-[13.5px] text-muted">
                        You can use this. Nobody wrote down who gave it to you.
                      </span>
                    </div>
                  ))}
              </Card>
            );
          })}
        </div>
      )}

      <MemberCatalog heldByProject={heldByProject} />

      {/*
        The workshop password card used to sit here, last on the page, and is
        deleted rather than commented out. It called a route the backend
        removed; a member sets a credential per system now, from Network
        storage. If the door and machine bridge ever needs a card on this page,
        lift the working panel out of MyStorage; do not restore this one.
      */}
    </div>
  );
}

/**
 * Whether Zitadel holds the role, from its own answer. Healthy is silence:
 * only "Not there yet" (after a complete read that lacks it) and "Couldn't
 * check" (a partial or failed read cannot conclude an absence) speak.
 */
type Standing = "held" | "missing" | "checking" | "unreachable";

function StandingMark({ standing }: { standing: Standing }) {
  if (standing === "missing") {
    return <span className="text-[13.5px] font-semibold text-warn-text">Not there yet</span>;
  }
  if (standing === "unreachable") {
    return <span className="text-[13.5px] text-faint">Couldn&rsquo;t check</span>;
  }
  return null;
}

/**
 * One role: its name, then why you have it. A role given until a date is
 * tinted and offers a way to keep it; an approval still on its way says why
 * (N19), in the member's words — never the operator's "Waiting for you to
 * send", which a member can't do.
 */
function RoleRow({
  name,
  projectId,
  roleKey,
  reasons,
  expiresAt,
  standing,
  waiting,
}: {
  name: string;
  projectId: string;
  roleKey: string;
  reasons: RoleReason[];
  expiresAt: string | null;
  standing: Standing;
  waiting?: MyChange;
}) {
  const [strongest] = orderedSources(reasons);
  const ends = Boolean(expiresAt) && strongest?.kind === "direct" && !waiting;
  return (
    <div
      className={`row-divider flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-3 ${ends ? "bg-warn-soft" : ""}`}
      data-ends={ends || undefined}
    >
      <span className="flex min-w-0 flex-[1_1_220px] flex-col gap-0.5">
        <span className="text-[15px] font-semibold">{name}</span>
        <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
          {waiting ? (
            <span className="text-[13.5px] font-semibold text-warn-text">
              Not there yet · approved {approvedWhen(waiting.at)}, and {rowReason(waiting.wait_reason)}
            </span>
          ) : nothingSentYet(reasons) ? (
            <span className="text-[13.5px] font-semibold text-warn-text">Not there yet</span>
          ) : (
            <>
              <Reason strongest={strongest} expiresAt={ends ? expiresAt : null} />
              <StandingMark standing={standing} />
            </>
          )}
        </span>
      </span>
      {ends && <ArrowLink href={askHref(projectId, roleKey, true)}>Ask to keep it longer</ArrowLink>}
    </div>
  );
}

function approvedWhen(iso: string): string {
  return new Date(iso).toDateString() === new Date().toDateString() ? "today" : `on ${formatShortDate(iso)}`;
}

function rowReason(waitReason: string | undefined): string {
  return waitReason?.startsWith("halted:")
    ? "Syndra couldn't reach the sign-in system; staff will send it"
    : "makerspace staff check this role before it reaches you";
}

/**
 * The access source, said the way a member would say it.
 *   bundle  → "Because you're in Lab Tech"
 *   direct  → "Given to you until 20 Oct · 10 days left"   (amber — it ends)
 *   mapping → "Comes with door access, automatically"
 */
function Reason({ strongest, expiresAt }: { strongest?: RoleReason; expiresAt?: string | null }) {
  if (!strongest) return <span className="text-[13.5px] text-muted">Part of your membership</span>;

  if (strongest.kind === "direct") {
    if (expiresAt) {
      const left = daysUntil(expiresAt) ?? 0;
      return (
        <span className="text-[13.5px] font-semibold text-warn-text">
          Given to you until {formatShortDate(expiresAt)} · {left} {left === 1 ? "day" : "days"} left
        </span>
      );
    }
    return <span className="text-[13.5px] text-muted">Given to you</span>;
  }

  if (strongest.kind === "bundle") {
    return (
      <span className="text-[13.5px] text-muted">
        Because you&rsquo;re in{" "}
        <strong className="font-semibold text-ink">{strongest.bundle_name ?? "a group"}</strong>
      </span>
    );
  }

  const from = sourceQualifier(strongest);
  return (
    <span className="text-[13.5px] text-muted">
      {from ? `Comes with ${humanizeKey(from.split("/").pop()?.trim() ?? from)}, automatically` : "Comes with your other access, automatically"}
    </span>
  );
}

function expiryFor(
  grants: Array<{ project_id: string; role_key: string; expires_at?: string | null }> | undefined,
  projectId: string,
  roleKey: string,
): string | null {
  return (
    grants?.find((grant) => grant.project_id === projectId && grant.role_key === roleKey)
      ?.expires_at ?? null
  );
}

function firstName(name: string): string {
  return name.trim().split(/\s+/)[0] || name;
}

/**
 * "Four roles across two rooms. One runs out on 20 Oct." It counts what
 * Zitadel holds and names the date. `roles` is undefined while nothing has
 * been observed, and then no number is named.
 */
export function summarySentence(roles: number | undefined, rooms: number, ending: string[]): string | null {
  if (roles === undefined) return "Checking what you can use.";
  if (roles === 0) return null;
  const base = `${capitalise(countWord(roles))} ${roles === 1 ? "role" : "roles"} across ${countWord(rooms)} ${
    rooms === 1 ? "room" : "rooms"
  }.`;
  if (ending.length === 0) return base;
  const first = formatShortDate(ending[0]);
  return ending.length === 1
    ? `${base} One runs out on ${first}.`
    : `${base} ${capitalise(countWord(ending.length))} run out, the first on ${first}.`;
}

const WORDS = ["no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"];

function countWord(count: number): string {
  return WORDS[count] ?? String(count);
}

function capitalise(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
