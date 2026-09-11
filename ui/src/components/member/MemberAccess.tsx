"use client";

import { ResolvedProjectName } from "@/components/names";
import { MemberCatalog } from "@/components/member/MemberCatalog";
import { EmptyState, ErrorState, RowSkeleton } from "@/components/states";
import { ButtonLink } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import {
  NotSentYet,
  nothingSentYet,
  orderedSources,
  sourceQualifier,
  type RoleReason,
} from "@/components/access/AccessSource";
import { Withheld } from "@/components/ui/Withheld";
import { useUpstreamUserGrants } from "@/lib/queries/useUpstream";
import { useUserAccess, useUserGrants } from "@/lib/queries/useUsers";
import type { SessionUser } from "@/lib/session";
import { daysUntil, formatShortDate, humanizeKey, roleLabel } from "@/lib/format";

/**
 * Member · My access. The landing route for members, and the one screen that
 * explains to somebody why they can badge into the laser bay.
 *
 * Access source becomes a SENTENCE, not a chip: a member has no vocabulary to
 * attach to Direct / Via bundle / Automatic. No jargon anywhere on this screen
 * — no "derived", no effective_role_keys, no role keys.
 */
export function MemberAccess({ session }: { session: SessionUser }) {
  const access = useUserAccess(session.id);
  const grants = useUserGrants(session.id);
  // What Zitadel actually holds for this person, read through the observer —
  // the same pipe the operator page uses. Syndra's own list says what was
  // decided; this says what is usable right now.
  const upstream = useUpstreamUserGrants(session.id);
  const held = new Set(
    (upstream.data?.items ?? []).flatMap((g) => g.roleKeys.map((rk) => `${g.projectId}:${rk}`)),
  );
  const standing = (projectId: string, roleKey: string): Standing => {
    if (upstream.isLoading) return "checking";
    if (upstream.error || !upstream.data) return "unreachable";
    if (held.has(`${projectId}:${roleKey}`)) return "held";
    return upstream.data.complete === false ? "unreachable" : "missing";
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
  const bundles = access.data?.bundles ?? [];
  const roleCount = projects.reduce((total, project) => total + project.effective_role_keys.length, 0);

  const expiringGrant = (grants.data ?? [])
    .filter((grant) => grant.expires_at)
    .sort((a, b) => (a.expires_at! < b.expires_at! ? -1 : 1))[0];

  const expiringRole = expiringGrant
    ? findRoleLabel(projects, expiringGrant.project_id, expiringGrant.role_key)
    : null;

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

  return (
    <div className="flex flex-col gap-[22px]">
      <div>
        <h1 className="type-page-title">
          Hi {firstName(session.name)}.{" "}
          <span className="text-ink/40">Here&rsquo;s what you can use.</span>
        </h1>
        <div className="mt-2 text-[14.5px] text-faint">{summarySentence(bundles.length, roleCount, Boolean(expiringGrant))}</div>
      </div>

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
              move that resolves it rather than describing one. Design asked for
              the person to ask by name — "ask Kabir Rao, who looks after
              Fabrication" — and there is nobody to name: no project records an
              owner anywhere in the product. Naming a plausible one would be the
              single worst place in Syndra to invent a fact, so this says what
              it can do instead of who to find. */}
          <EmptyState
            title="You don't have access to anything yet."
            guidance="Nobody has given you access. If there's a machine or a space you need, ask for it here and makerspace staff will decide — everything the makerspace offers is listed below."
            action={{ label: "Ask for access", href: "/requests" }}
          />
        </Card>
      ) : (
        <div className="flex flex-wrap gap-[18px]">
          {projects.map((project) => (
            <Card key={project.project_id} className="min-w-[420px] flex-1">
              <div className="px-5 py-4 font-display text-[22px] font-semibold">
                <ResolvedProjectName
                  name={project.project_name}
                  resolved={project.project_name_resolved}
                  id={project.project_id}
                />
              </div>
              {[...project.source_roles, ...project.derived_roles].map((role) => (
                <div
                  key={role.role_key}
                  className="row-divider flex items-center gap-3.5 px-5 py-3"
                >
                  <span className="flex-1 text-[15px] font-semibold">
                    {humanizeKey(role.role_key)}
                  </span>
                  <ReasonSentence
                    reasons={role.reasons}
                    expiresAt={expiryFor(grants.data, project.project_id, role.role_key)}
                    standing={standing(project.project_id, role.role_key)}
                  />
                </div>
              ))}
            </Card>
          ))}
        </div>
      )}

      {expiringGrant && (
        <div className="warn-note flex flex-wrap items-center gap-4 px-5 py-4">
          <div className="min-w-[300px] flex-1 text-[15px] leading-[1.55] text-ink/80">
            <strong className="font-semibold text-warn-text">
              {expiringRole ?? "One of your permissions"} runs out on{" "}
              {formatShortDate(expiringGrant.expires_at)}.
            </strong>{" "}
            If you still need it, ask and makerspace staff can extend it.
          </div>
          <ButtonLink href="/requests" variant="accent">
            Request an extension
          </ButtonLink>
        </div>
      )}

      <MemberCatalog heldByProject={heldByProject} />

      {/*
        The workshop password card used to sit here, last on the page, and is
        deleted rather than commented out.

        It called PUT /users/{uid}/shadow-credential, which the backend removed
        (internal/handlers/vault.go says so where the handler stood): a member
        sets a credential per target now, through
        POST /me/targets/{target}/credential. Network storage already does that
        and is the one working door.

        What stood here was 385 lines calling a route that 404s, kept alive by
        a test that mocked the dead hook into passing — which is what made it
        look maintained. If the door and machine bridge ever needs a card on
        this page, lift the working panel out of MyStorage; do not restore this
        one from history.
      */}
    </div>
  );
}

/**
 * The access source, said the way a member would say it.
 *   bundle  → "Because you're in Lab Tech"
 *   direct  → "Given to you until 2 Aug"   (amber — it ends)
 *   mapping → "Comes with door access, automatically"
 */
type Standing = "held" | "missing" | "checking" | "unreachable";

/**
 * The one line that says whether the role is usable right now, from Zitadel's
 * own answer. "Not there yet" only after a complete read — a partial or failed
 * read cannot conclude an absence, so it says it could not check.
 */
function StandingMark({ standing }: { standing: Standing }) {
  switch (standing) {
    case "held":
      return <span className="text-[12.5px] font-semibold text-healthy">Ready to use</span>;
    case "missing":
      return <span className="text-[12.5px] font-semibold text-warn-text">Not there yet</span>;
    case "checking":
      return <span className="text-[12.5px] text-faint">Checking…</span>;
    default:
      return <span className="text-[12.5px] text-faint">Couldn&rsquo;t check</span>;
  }
}

function ReasonSentence({
  reasons,
  expiresAt,
  standing,
}: {
  reasons: RoleReason[];
  expiresAt?: string | null;
  standing: Standing;
}) {
  const [strongest] = orderedSources(reasons);
  if (!strongest) return <span className="text-[13.5px] text-faint">Part of your membership</span>;

  // Recorded, not yet sent: nothing that gives this role has left Syndra's
  // outbox, so this row is a decision on file, not access they can use yet.
  if (nothingSentYet(reasons)) {
    return <NotSentYet className="text-[13.5px]" />;
  }
  return (
    <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
      <Reason strongest={strongest} expiresAt={expiresAt} />
      <StandingMark standing={standing} />
    </span>
  );
}

function Reason({ strongest, expiresAt }: { strongest: RoleReason; expiresAt?: string | null }) {

  if (strongest.kind === "direct") {
    const remaining = daysUntil(expiresAt);
    if (expiresAt) {
      return (
        <span className="text-[13.5px] font-semibold text-warn-text">
          Given to you until {formatShortDate(expiresAt)}
          {remaining !== null && remaining >= 0 ? ` · ${remaining} days` : ""}
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

/**
 * The role rows above live inside a card headed by their project, so they name
 * the role alone. This label does not: the expiry warning sits outside every
 * card, and a member holding "Operator" in three projects cannot tell from
 * "Operator runs out on 12 Aug" which one they are about to lose.
 */
function findRoleLabel(
  projects: Array<{
    project_id: string;
    project_name: string;
    source_roles: Array<{ role_key: string }>;
    derived_roles: Array<{ role_key: string }>;
  }>,
  projectId: string,
  roleKey: string,
): string | null {
  const project = projects.find((entry) => entry.project_id === projectId);
  if (!project) return null;
  const found = [...project.source_roles, ...project.derived_roles].find(
    (role) => role.role_key === roleKey,
  );
  return found ? roleLabel(project.project_name, found.role_key) : null;
}

function firstName(name: string): string {
  return name.trim().split(/\s+/)[0] || name;
}

function summarySentence(bundles: number, roles: number, expiring: boolean): string {
  const parts: string[] = [];
  if (bundles > 0) parts.push(`${countWord(bundles)} ${bundles === 1 ? "membership" : "memberships"}`);
  parts.push(`${countWord(roles)} ${roles === 1 ? "permission" : "permissions"}`);
  const base = parts.join(" and ");
  return expiring ? `${capitalise(base)}. One expires soon.` : `${capitalise(base)}.`;
}

const WORDS = ["no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"];

function countWord(count: number): string {
  return WORDS[count] ?? String(count);
}

function capitalise(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
