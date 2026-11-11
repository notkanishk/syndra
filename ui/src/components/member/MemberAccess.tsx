"use client";

import { useSchemaSource } from "@/lib/queries/useFieldSchema";
import { ResolvedProjectName } from "@/components/names";
import { MemberCatalog } from "@/components/member/MemberCatalog";
import { headerLine, type NamedRole } from "@/components/member/headerLine";
import { RecentChanges } from "@/components/member/RecentChanges";
import { AskForAccess } from "@/components/requests/AskForAccess";
import { EmptyState, ErrorState, RowSkeleton } from "@/components/states";
import { ReadGate } from "@/components/states/ReadGate";
import { Button } from "@/components/ui/Button";
import { ARROW_LINK, ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { BELOW_HEADER, usePageHead } from "@/components/ui/PageHeader";
import {
  givenAt,
  orderedSources,
  sourceQualifier,
  type RoleReason,
} from "@/components/access/AccessSource";
import { Withheld } from "@/components/ui/Withheld";
import { useCatalogProjects } from "@/lib/queries/useCatalogUsers";
import { holds, onItsWay as onItsWayStanding, standingTone, standingWords, type Standing } from "@/lib/standing";
import { useMyChanges, type MyChange } from "@/lib/queries/useMyChanges";
import { useUserAccess, useUserGrants } from "@/lib/queries/useUsers";
import type { SessionUser } from "@/lib/session";
import { useEffect, useState } from "react";
import { expiryLabel, formatShortDate, humanizeKey } from "@/lib/format";
import { ShellNotices } from "@/components/shell/ShellNotices";
import { greeting, lastHere, WELCOME } from "@/lib/greeting";
import { Crossfade, glideTo } from "@/lib/motion";
import { useMyStorage } from "@/lib/queries/useMyStorage";
import { useRequestsMine } from "@/lib/queries/useRequests";
import { sortByName } from "@/lib/sort";
import { roleName } from "@/lib/names";

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
  // The member's home is a screen's first paint (S11): the greeting tints in, then the cards arrive.
  const head = usePageHead();
  const access = useUserAccess(session.id);
  // Only the holds in force are drawn, and only they need their system's
  // fields to be worded: no hold, no read of /me/targets on this card's behalf
  // (review finding 6; the rail reads it for its own rows).
  const inForce = (access.data?.allowances ?? []).filter((allowance) => allowance.in_force);
  const schemaOf = useSchemaSource(false, inForce.length > 0);
  const grants = useUserGrants(session.id);
  const changes = useMyChanges();
  const catalog = useCatalogProjects();
  const storage = useMyStorage();
  const myRequests = useRequestsMine();
  // "Ask to keep it" asks here and stays here (A-19 P10.4): the dialog is mounted on this page.
  const [asking, setAsking] = useState<{ projectId: string; roleKey: string } | null>(null);
  // The line and the greeting depend on the reader's clock and zone, which the
  // server doesn't share: they appear on mount, in a seat held by an invisible
  // stand-in so nothing below moves (same rule as Home).
  const [now, setNow] = useState<Date | null>(null);
  useEffect(() => setNow(new Date()), []);
  // What Zitadel holds for this person arrives WITH the records that explain
  // it, in one response. It used to be a second call, which meant this screen
  // could render a headline from Syndra's records against rows marked from
  // Zitadel's — two answers to one question, disagreeing in front of the
  // person whose access it is.
  const observation = access.data?.observation;
  // Whether they hold a role is the backend's answer (lib/standing.ts), only
  // read here. A role it says nothing about has no standing: not checked.
  const standing = (project: (typeof projects)[number], roleKey: string): Standing =>
    project.standings?.[roleKey] ?? null;

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

  // An unread grants list is not "nothing ends": the end dates wait for the
  // read, and a role's seat says so rather than "Given to you" (A-08 §3.9).
  const grantsRead = !grants.isLoading && !grants.error;

  // Labels from the catalogue the page already reads below; a role it doesn't
  // list falls back to its key, humanised.
  const roleNameIn = (projectId: string, roleKey: string) =>
    roleName(catalog.data.find((project) => project.id === projectId)?.roles.find((role) => role.key === roleKey)?.label, roleKey);

  // The role a rule gives each role for, read off the access already on the
  // page: /me/changes says a rule gave it, not which role it follows. Two
  // different triggers for one role is ambiguous, so it says "a rule" instead.
  const triggerRole = (projectId: string, roleKey: string) => {
    const project = projects.find((p) => p.project_id === projectId);
    const role = [...(project?.source_roles ?? []), ...(project?.derived_roles ?? [])].find(
      (r) => r.role_key === roleKey,
    );
    const triggers = new Set(
      (role?.reasons ?? [])
        .filter((reason) => reason.kind === "mapping" && reason.trigger_project && reason.trigger_role)
        .map((reason) => `${reason.trigger_project}\u0000${reason.trigger_role}`),
    );
    if (triggers.size !== 1) return undefined;
    const [triggerProject, trigger] = Array.from(triggers)[0].split("\u0000");
    return roleNameIn(triggerProject, trigger);
  };

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
      // What they already have, by the backend's one rule (holdings): the same
      // answer that refuses a second ask, so the catalogue never offers it.
      new Set(
        Object.entries(project.holdings ?? {})
          .filter(([, h]) => h.kind !== "none")
          .map(([roleKey]) => roleKey),
      ),
    ]),
  );

  // Only what still applies — a lifted or lapsed hold is history, not
  // something they need to read about on the page that answers "what can I
  // use right now". Same object PersonAccess shows an operator, member voice.
  const unread = !observation?.read_at;

  // A project's name for the line: the read's own when it resolved, else the
  // catalogue's. Never an id, so a project with neither is simply not named.
  const projectName = (projectId: string): string | undefined => {
    const project = projects.find((p) => p.project_id === projectId);
    if (project?.project_name_resolved) return project.project_name;
    return catalog.data.find((p) => p.id === projectId)?.name;
  };
  const named = (projectId: string, roleKey: string): NamedRole => {
    const project = projectName(projectId);
    const role = roleNameIn(projectId, roleKey);
    return { label: project ? `${role} in ${project}` : role, projectId, project };
  };
  const held = (projectId: string, roleKey: string) =>
    holds(projects.find((p) => p.project_id === projectId)?.standings?.[roleKey]);
  const today = (iso: string) => now !== null && new Date(iso).toDateString() === now.toDateString();
  // Roles that are on their way, from either answer: an approval in the feed
  // (a request, not a role's holding), or a role whose standing says so.
  const onItsWay = new Map<string, NamedRole>();
  waiting.forEach((change) => onItsWay.set(`${change.project_id}/${change.role_key}`, named(change.project_id, change.role_key)));
  const missing = new Map<string, NamedRole>();
  for (const project of projects) {
    for (const role of [...project.source_roles, ...project.derived_roles]) {
      const key = `${project.project_id}/${role.role_key}`;
      if (waiting.has(key)) continue;
      const s = standing(project, role.role_key);
      if (onItsWayStanding(s)) onItsWay.set(key, named(project.project_id, role.role_key));
      else if (s === "not_held") missing.set(key, named(project.project_id, role.role_key));
    }
  }
  const line = now
    ? headerLine({
        loading: access.isLoading,
        read: !unread,
        now,
        firstVisit: session.firstVisit,
        // Only said when the feed was read and holds nothing after the previous
        // sign-in: a record of a change, never a count of access (A-20 P18).
        quietSince:
          !changes.isLoading && !changes.error && !feed.some((c) => new Date(c.at).getTime() > Date.parse(session.previousSignIn ?? ""))
            ? (lastHere(session.previousSignIn, now)?.on ?? undefined)
            : undefined,
        // Only what Zitadel holds, with the grant's end: the record supplies the date, never the count.
        ending: grantsRead
          ? (grants.data ?? []).flatMap((grant) =>
              grant.expires_at && held(grant.project_id, grant.role_key)
                ? [{ ...named(grant.project_id, grant.role_key), endsAt: grant.expires_at }]
                : [],
            )
          : [],
        missing: Array.from(missing.values()),
        storage: (() => {
          const target = (storage.data ?? []).find(
            (t) => t.entitled && (!t.credential.set || t.credential.needs_re_enrolment),
          );
          return target ? { name: target.name } : undefined;
        })(),
        requests: (myRequests.data ?? [])
          .filter((r) => r.status === "pending")
          .map((r) => ({
            at: r.created_at,
            label: projectName(r.project_id) ? named(r.project_id, r.role_key).label : undefined,
          })),
        onItsWay: Array.from(onItsWay.values()),
        startedToday: feed.flatMap((change) => {
          const project = projectName(change.project_id);
          return (change.kind === "given" || change.kind === "request_approved") &&
            !change.waiting &&
            today(change.at) &&
            held(change.project_id, change.role_key) &&
            project
            ? [{ project, projectId: change.project_id }]
            : [];
        }),
      })
    : null;
  // Zitadel never read (A-06 §2, N23): nothing has been observed, so the cards
  // keep their headers and show no rows. A row would say what Syndra decided
  // as though it were what the person can use.

  return (
    <div className="flex flex-col gap-4">
      <div className={head.className} data-settled={head["data-settled"]} style={BELOW_HEADER}>
        <h1 className="type-page-title">
          {now ? greeting(now) : <span className="invisible">Afternoon</span>}, {firstName(session.name)}.{" "}
          <span className="text-faint">{session.firstVisit ? WELCOME : <>Here&rsquo;s what you can use.</>}</span>
        </h1>
        {/* One line, always there, in the same seat in every state, so nothing below it moves. */}
        <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-0 text-[14.5px] text-muted">
          {/* The welcome is the title's second sentence (A-20 P2), so the ladder
              speaks here on the first visit too: "From today, you can use …"
              is never hidden on its day. */}
          {line ? (
            <>
              <span className="flex items-center gap-2.5">
                {line.tone !== "none" && <StatusDot tone={line.tone} />}
                <Crossfade id={line.text}>{line.text}</Crossfade>
              </span>
              {line.link?.href && <ArrowLink href={line.link.href}>{line.link.label}</ArrowLink>}
              {line.link?.projectId && (
                <button
                  type="button"
                  className={ARROW_LINK}
                  onClick={() => {
                    const target = document.getElementById(`card-${line.link!.projectId}`);
                    if (target) void glideTo(target);
                  }}
                >
                  {line.link.label}
                  <span aria-hidden>↓</span>
                </button>
              )}
            </>
          ) : (
            <span className="invisible">Checking what you can use.</span>
          )}
        </div>
        {/* The shell's Notices, under the whole head (A-20 P3, G7). */}
        <ShellNotices />
      </div>

      {/* The read is one hand-over (S7): a skeleton only if it is slow, then the cards arrive. */}
      <ReadGate
        loading={access.isLoading}
        skeleton={
          <Card>
            <RowSkeleton rows={4} avatar={false} label="Loading your access" />
          </Card>
        }
      >
      {/* Leads, and keeps its seat on a quiet month (structure never moves
          with data). Not drawn until the read lands, so nothing unread is
          called quiet. */}
      {changes.data && (
        <RecentChanges
          items={feed}
          roleName={roleNameIn}
          triggerRole={triggerRole}
          endOf={(projectId, roleKey) => expiryFor(grants.data, projectId, roleKey)}
        />
      )}

      {inForce.length > 0 && (
        <Card>
          <div className="px-5 py-4">
            <Withheld
              schemaOf={schemaOf}
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
            guidance="If there's a machine or a space you need, ask for it. Everything the makerspace offers is listed below."
            action={{ label: "Ask for access", href: "/requests?ask" }}
          />
        </Card>
      ) : (
        <div className="flex flex-wrap gap-4">
          {sortByName(projects, (p) => p.project_name || "", (p) => p.project_id).map((project) => {
            const roles = [...project.source_roles, ...project.derived_roles]
              .map((role) => ({
                ...role,
                expiresAt: expiryFor(grants.data, project.project_id, role.role_key),
              }))
              // The role that ends leads its card.
              .sort((a, b) => {
                const [x, y] = [a.expiresAt ?? "~", b.expiresAt ?? "~"];
                return x < y ? -1 : x > y ? 1 : 0;
              });
            return (
              <Card key={project.project_id} className="min-w-[min(420px,100%)] flex-1">
                <CardHeader
                  title={
                    // The id is the target of the header line's jump: its title flashes on landing.
                    <span id={`card-${project.project_id}`}>
                      <ResolvedProjectName
                        name={project.project_name}
                        resolved={project.project_name_resolved}
                        id={project.project_id}
                      />
                    </span>
                  }
                />
                {!unread && roles.map((role) => (
                  <RoleRow
                    key={role.role_key}
                    name={roleNameIn(project.project_id, role.role_key)}
                    reasons={role.reasons}
                    expiresAt={role.expiresAt}
                    endKnown={grantsRead}
                    standing={standing(project, role.role_key)}
                    unread={unread}
                    waiting={waiting.get(`${project.project_id}/${role.role_key}`)}
                    askedToKeep={(myRequests.data ?? []).some(
                      (r) =>
                        r.status === "pending" &&
                        r.kind === "extend" &&
                        r.project_id === project.project_id &&
                        r.role_key === role.role_key,
                    )}
                    onAskToKeep={() => setAsking({ projectId: project.project_id, roleKey: role.role_key })}
                  />
                ))}
                {/* Something they hold that Syndra has no reason for. The
                    operator's People page calls this "unexplained"; the person
                    holding it should not be the only one who cannot see it. */}
                {sortByName(
                  // Only to LIST them; what each says comes from its standing.
                  (project.observed_role_keys ?? []).filter((roleKey) => !project.effective_role_keys.includes(roleKey)),
                  (roleKey) => roleNameIn(project.project_id, roleKey),
                  (roleKey) => roleKey,
                ).map((roleKey) => (
                    <div key={`held-${roleKey}`} className="row-divider px-5 py-3">
                      <span className="text-[15px] font-semibold">{roleNameIn(project.project_id, roleKey)}</span>
                      <StandingMark standing={standing(project, roleKey)} unread={unread} />
                    </div>
                  ))}
              </Card>
            );
          })}
        </div>
      )}

      <MemberCatalog heldByProject={heldByProject} />
      </ReadGate>

      {/*
        The workshop password card used to sit here, last on the page, and is
        deleted rather than commented out. It called a route the backend
        removed; a member sets a credential per system now, from Network
        storage. If the door and machine bridge ever needs a card on this page,
        lift the working panel out of MyStorage; do not restore this one.
      */}

      {/* Asked from the row, answered in the row: the page does not leave (A-19 P10.4). */}
      {asking && (
        <AskForAccess
          key={`${asking.projectId}:${asking.roleKey}`}
          open
          onClose={() => setAsking(null)}
          initial={asking}
          userId={session.id}
          selfDecides={session.role === "admin"}
        />
      )}
    </div>
  );
}

/**
 * Whether Zitadel holds the role, from its own answer. Healthy is silence:
 * only "Not there yet" (after a complete read that lacks it) and "Couldn't
 * check" (a partial or failed read cannot conclude an absence) speak.
 */
function StandingMark({ standing, unread }: { standing: Standing; unread: boolean }) {
  // Zitadel not read yet is not a failure: nothing to say until it has been.
  if (standing == null && unread) return null;
  const words = standingWords(standing, "member");
  if (!words) return null;
  const warn = standingTone(standing) === "warn";
  return <span className={`text-[13.5px] ${warn ? "font-semibold text-warn-text" : "text-faint"}`}>{words}</span>;
}

/**
 * One role: its name, then why you have it. A role given until a date
 * offers a way to keep it; an approval still on its way says why
 * (N19), in the member's words — never the operator's "Waiting for you to
 * send", which a member can't do.
 */
function RoleRow({
  name,
  reasons,
  expiresAt,
  endKnown,
  standing,
  unread,
  waiting,
  askedToKeep,
  onAskToKeep,
}: {
  name: string;
  reasons: RoleReason[];
  expiresAt: string | null;
  /** Whether the grants list was read: without it a missing end date is unknown, not absent. */
  endKnown: boolean;
  standing: Standing;
  /** Zitadel has not been read for them yet (the view's observation). */
  unread: boolean;
  waiting?: MyChange;
  /** A request to keep this role is waiting for makerspace staff. */
  askedToKeep: boolean;
  onAskToKeep: () => void;
}) {
  const [strongest] = orderedSources(reasons);
  const ends = Boolean(expiresAt) && strongest?.kind === "direct" && !waiting;
  return (
    <div
      className="row-divider flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-3"
      data-ends={ends || undefined}
    >
      <span className="flex min-w-0 flex-[1_1_220px] flex-col gap-0.5">
        <span className="text-[15px] font-semibold">{name}</span>
        <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
          {waiting || onItsWayStanding(standing) ? (
            // When it was queued, from the record; the feed's time only when the read has none.
            <OnItsWay at={givenAt(reasons) ?? waiting?.at} sent={standing === "sent"} />
          ) : (
            <>
              <Reason strongest={strongest} expiresAt={ends ? expiresAt : null} endKnown={endKnown} />
              <StandingMark standing={standing} unread={unread} />
            </>
          )}
        </span>
      </span>
      {/* It opens a dialog, so it is a button with no arrow (A-04 §7.5). The answer takes the
          button's seat, so the row does not change shape. */}
      {ends && (
        <Crossfade id={askedToKeep ? "asked" : "open"}>
          {askedToKeep ? (
            <span className="inline-flex min-h-[32px] items-center gap-2 text-[13.5px] font-semibold text-muted">
              <StatusDot tone="neutral" />
              Asked to keep it
            </span>
          ) : (
            <Button variant="outline" size="sm" onClick={onAskToKeep}>
              Ask to keep it
            </Button>
          )}
        </Crossfade>
      )}
    </div>
  );
}

/**
 * A role that was given and hasn't been sent (A-04 §7.3). A member reads "On
 * its way" with a neutral dot, never the operator's "Waiting for you to send",
 * which a member can't do, and never a phrase that reads as automatic (A-04 §7.3). The reason it is held (a role that always waits, or Syndra not
 * reaching the sign-in system) is the operator's to read; the member's
 * question is only whether to expect it.
 */
function OnItsWay({ at, sent }: { at?: string; sent?: boolean }) {
  return (
    <span className="flex flex-col gap-0.5">
      <span className="inline-flex items-center gap-2 text-[13.5px] font-semibold text-muted">
        <StatusDot tone="neutral" />
        On its way
      </span>
      <span className="pl-[15px] text-[12.5px] text-faint">
        {at ? `Given ${givenWhen(at)}. ` : ""}
        {sent ? "Syndra has sent it. It should show up soon." : "It starts working once makerspace staff send it."}
      </span>
    </span>
  );
}

function givenWhen(iso: string): string {
  return new Date(iso).toDateString() === new Date().toDateString() ? "today" : formatShortDate(iso);
}

/**
 * The access source, said the way a member would say it.
 *   bundle  → "Because you're in Lab Tech"
 *   direct  → "Given to you until 20 Oct · 10 days left"   (amber — it ends)
 *   mapping → "Comes with door access, automatically"
 */
function Reason({
  strongest,
  expiresAt,
  endKnown,
}: {
  strongest?: RoleReason;
  expiresAt?: string | null;
  endKnown: boolean;
}) {
  if (!strongest) return <span className="text-[13.5px] text-muted">Part of your membership</span>;

  if (strongest.kind === "direct") {
    if (expiresAt) {
      return (
        <span className="text-[13.5px] font-semibold text-warn-text">
          Given to you until {formatShortDate(expiresAt)} · {expiryLabel(expiresAt, "left")}
        </span>
      );
    }
    return (
      <span className="text-[13.5px] text-muted">{endKnown ? "Given to you" : "End date unavailable"}</span>
    );
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
