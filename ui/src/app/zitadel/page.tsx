"use client";

import { Term } from "@/components/ui/Term";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { ErrorState, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { CommandBlock } from "@/components/ui/CommandBlock";
import { PageHeader } from "@/components/ui/PageHeader";
import { request } from "@/lib/api-client";
import { useProjects } from "@/lib/queries/useProjects";
import { useZitadelHealth } from "@/lib/queries/useZitadel";
import { formatLongDate } from "@/lib/format";

interface RotationStatus {
  key_installed?: boolean;
  last_rotated_at?: string | null;
  age_days?: number;
  threshold_days?: number;
  status?: "ok" | "warn" | "stale" | string;
  rotate_command?: string;
}

/**
 * S9 · System › Identity provider.
 *
 * Read-mostly. Zitadel owns authorization state; this screen answers "is it
 * reachable, does it agree with us, and when was the signing key last rotated"
 * — it is not a second console for editing it.
 *
 * Health is a SENTENCE with a cause and a timestamp, never a green or red dot
 * on its own. A dot tells an operator that something is wrong and nothing else;
 * the sentence tells them what broke, when it last worked, and what Syndra is
 * doing about it in the meantime.
 */
export default function IdentityProviderPage() {
  const health = useZitadelHealth();
  const projects = useProjects();
  const rotation = useQuery({
    queryKey: ["zitadel", "rotation"],
    queryFn: () => request<RotationStatus>("/zitadel/action-rotation-status"),
    retry: false,
  });

  const live = health.data?.status === "ok";
  const notConfigured = health.data?.status === "disabled";

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Zitadel"
        lede={
          <>
            Whether Syndra can reach <Term name="zitadel">Zitadel</Term>, and what it holds
            right now. Nothing on this page changes anyone’s access.
          </>
        }
        meta={health.data?.domain ? <Mono>{health.data.domain}</Mono> : undefined}
      />

      {health.isLoading ? (
        <Card>
          <RowSkeleton rows={2} avatar={false} label="Checking whether Syndra can reach Zitadel" />
        </Card>
      ) : health.error ? (
        <ErrorState
          title="Couldn't check whether Zitadel is reachable."
          error={health.error}
          onRetry={() => health.refetch()}
        />
      ) : (
        <HealthVerdict
          live={live}
          notConfigured={notConfigured}
          latency={health.data?.latency_ms}
          detail={health.data?.error}
        />
      )}

      <div className="flex flex-wrap gap-[18px]">
        <StatCard
          label="Action signing key"
          value={rotationHeadline(rotation.data, rotation.error)}
          tone={rotation.data?.status === "ok" ? "neutral" : "warn"}
          detail={
            rotation.data?.last_rotated_at
              ? `Last replaced ${formatLongDate(rotation.data.last_rotated_at)}`
              : "A secret Syndra and Zitadel share so each can trust the other during sign-in."
          }
        />
        <StatCard
          label="Projects in Zitadel"
          value={
            live
              ? `${health.data?.projects_total ?? projects.data?.length ?? 0}`
              : `${projects.data?.length ?? 0}`
          }
          tone="neutral"
          detail={
            live
              ? "As Zitadel reported them just now."
              : "As Syndra last remembered them — Zitadel cannot be asked right now."
          }
        />
        <StatCard
          label="Connection"
          value={health.data?.mode === "live" ? "Connected" : "Not connected"}
          tone={health.data?.mode === "live" ? "neutral" : "warn"}
          detail={
            health.data?.mode === "live"
              ? "Syndra can read and change access in Zitadel."
              : "Syndra is not set up to talk to Zitadel, so decisions made here change nobody's real access."
          }
        />
      </div>

      <SigningKey status={rotation.data} loading={rotation.isLoading} error={rotation.error} />
      <UpstreamInspection reachable={live} />
      <UpstreamWrites reachable={live} />
      <TheName />
    </div>
  );
}

/**
 * The one place the name is explained to somebody who is already inside.
 *
 * It lives here rather than on its own page because this screen is about the
 * relationship it describes: Zitadel is the door, Syndra is the list. An
 * operator reading "is the provider reachable" is the one person for whom the
 * split is not decoration — it is the reason there are two systems.
 *
 * Deliberately last, deliberately quiet: no accent fill, no badge. Nothing on
 * this page is more important than the health sentence at the top.
 */
function TheName() {
  return (
    <Card className="px-5 py-[18px]">
      <p className="type-card-title">Syn keeps the door. Syndra keeps the list.</p>
      <p className="mt-2 max-w-[68ch] text-[14px] leading-[1.6] text-muted">
        Syn keeps the door of Frigg&rsquo;s hall and bars it against those who should not enter.
        Zitadel is that door: it decides whether you are who you say you are. Syndra decides what
        that gets you, and keeps the record of why.
      </p>
    </Card>
  );
}

/**
 * The signing key panel — read-only, and deliberately so.
 *
 * There is no "Rotate now" button and there should not be one. Rotation is two
 * writes that must both land: Zitadel mints a new key, and the backend must be
 * restarted holding it. A click that does the first and not the second leaves
 * every claim-injection call failing signature verification, with the browser
 * showing a success toast. The operator running it in a terminal sees the exit
 * code, the new key, and the restart they still owe.
 *
 * What the screen owes them in return is the command and everything that has to
 * happen after it — which is what the old console had and the rebuild dropped.
 */
function SigningKey({
  status,
  loading,
  error,
}: {
  status?: RotationStatus;
  loading: boolean;
  error: unknown;
}) {
  const rotateCommand = status?.rotate_command ?? "make zitadel-actions-rotate-key";

  return (
    <Card>
      <CardHeader
        title="Action signing key"
        note="Zitadel uses it to prove to Syndra that a sign-in request is genuine. It never expires on its own; someone has to decide to replace it."
      />

      <div className="flex flex-col gap-4 px-5 py-4">
        <p className="max-w-[84ch] text-[13.5px] leading-[1.55] text-muted">
          This section is for the person who runs the Syndra server. If that is not you, nothing
          here needs your attention unless the headline above is amber.
        </p>
        <p className="max-w-[84ch] text-[14.5px] leading-[1.6]">{rotationSentence(status, error)}</p>

        {loading ? (
          <RowSkeleton rows={1} avatar={false} label="Checking when the key was last replaced" />
        ) : !status?.key_installed && !error ? (
          <CommandBlock
            tone="warn"
            command="make zitadel-actions-register"
            caption="No key is set up, so Syndra cannot tell genuine Zitadel requests from forged ones. Run this to register with Zitadel; it creates a key."
            steps={[
              <>
                Copy the printed key into <Mono>ZITADEL_ACTION_SIGNING_KEY</Mono> in{" "}
                <Mono>.env</Mono>.
              </>,
              <>
                Restart the backend: <Mono>docker compose restart backend</Mono>.
              </>,
              <>
                Confirm it took: <Mono>make zitadel-actions-verify</Mono>.
              </>,
            ]}
          />
        ) : (
          <CommandBlock
            command={rotateCommand}
            caption="Run this in a terminal on the server Syndra runs on. This panel does not replace the key itself — you must restart afterwards, and a half-done replacement is worse than an old key."
            steps={[
              <>
                The command writes the new key to a file under <Mono>zitadel/actions/</Mono>, one
                per registered Action, and backs up the old one alongside it.
              </>,
              <>
                Put the new value in <Mono>ZITADEL_ACTION_SIGNING_KEY</Mono>, and the printed
                timestamp in <Mono>ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT</Mono> — without the
                second one this panel goes back to reporting an unknown age.
              </>,
              <>
                Restart the backend: <Mono>docker compose restart backend</Mono>.
              </>,
              <>
                Confirm it took: <Mono>make zitadel-actions-verify</Mono>.
              </>,
            ]}
          />
        )}

        <p className="max-w-[84ch] text-[13.5px] leading-[1.55] text-muted">
          Between replacing the key and restarting, people can still sign in, but the access
          details Syndra normally adds at sign-in are missing. Keep the gap short and do it when
          nobody is working.
        </p>
      </div>
    </Card>
  );
}

/**
 * One sentence carrying state, cause and consequence. The stat card above
 * gives the headline; this says what it means. `disabled` is the case worth
 * spelling out — it reads like "not set up yet" and actually means every
 * inbound Action request is being trusted unchecked.
 */
function rotationSentence(status: RotationStatus | undefined, error: unknown): string {
  if (error) {
    return "Couldn't check when the key was last replaced. The key itself still works.";
  }
  if (!status) return "";

  if (!status.key_installed) {
    return "No key is set up, so Syndra accepts every request that claims to come from Zitadel without checking. Anyone who can reach Syndra on the network could change what a sign-in grants. Set up a key before real access depends on this.";
  }

  const threshold = status.threshold_days ?? 90;
  const age = status.age_days;

  if (age === undefined) {
    return `The key works, but Syndra does not know when it was last replaced, so it cannot warn you when the ${threshold}-day limit is reached. Set ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT to the date the rotate command printed.`;
  }
  if (status.status === "stale") {
    return `The key is ${age} days old — more than twice the ${threshold}-day limit. Replace it now.`;
  }
  if (status.status === "warn") {
    return `The key is ${age} days old, over the ${threshold}-day limit. Plan to replace it.`;
  }
  return `The key is ${age} days old, within the ${threshold}-day limit. Nothing to do.`;
}

function HealthVerdict({
  live,
  notConfigured,
  latency,
  detail,
}: {
  live: boolean;
  notConfigured: boolean;
  latency?: number;
  detail?: string;
}) {
  if (live) {
    return (
      // A neutral panel with a healthy dot, not an accent-tinted one. Violet
      // means "you can act on this" everywhere else in the product, and there
      // is nothing here to act on; lime says so without filling a field.
      <div className="panel px-5 py-4">
        <div className="flex items-center gap-2.5 text-[15px] font-semibold">
          <span aria-hidden className="h-2 w-2 flex-none rounded-pill bg-healthy" />
          Reachable — answered in {latency ?? 0}ms.
        </div>
        <p className="mt-1 max-w-[80ch] text-[14px] leading-[1.55] text-muted">
          Syndra is talking to Zitadel normally. Changes you send from Pending changes reach
          Zitadel as soon as you send them.
        </p>
      </div>
    );
  }

  return (
    <div className={`${notConfigured ? "warn-note" : "danger-note"} px-5 py-4`}>
      <div
        className={`text-[15px] font-semibold ${
          notConfigured ? "text-warn-text" : "text-danger-text"
        }`}
      >
        {notConfigured
          ? "Not connected — Syndra is not set up to talk to Zitadel, so nothing decided here changes anyone's real access."
          : "Unreachable — Syndra's last attempt to reach Zitadel failed."}
      </div>
      <p className="mt-1 max-w-[80ch] text-[14px] leading-[1.55] text-muted">
        {detail ||
          "Syndra is showing what it last knew. Nothing is lost: changes waiting to be sent stay in order and go through when Zitadel is back."}
      </p>
    </div>
  );
}

function StatCard({
  label,
  value,
  detail,
  tone,
}: {
  label: string;
  value: string;
  detail: string;
  tone: "neutral" | "warn";
}) {
  return (
    <div className="card min-w-[260px] flex-1 px-5 py-4">
      <div className="type-label mb-2">{label}</div>
      <div
        className={`font-display text-[24px] font-semibold ${
          tone === "warn" ? "text-warn-text" : ""
        }`}
      >
        {value}
      </div>
      <p className="mt-1.5 max-w-[42ch] text-[13px] leading-[1.5] text-muted">{detail}</p>
    </div>
  );
}

/**
 * Reads only. All three are disabled while the provider is unreachable, with
 * the reason in visible copy rather than a tooltip: they read live, and there
 * is nothing to read.
 */
function UpstreamInspection({ reachable }: { reachable: boolean }) {
  const rows: Array<{ label: string; open: string; path: string; href: string }> = [
    {
      label: "Projects and their roles",
      open: "Open projects",
      path: "/zitadel/projects",
      href: "/zitadel/projects",
    },
    { label: "People", open: "Open people", path: "/zitadel/users", href: "/zitadel/users" },
    { label: "Roles held", open: "Open roles held", path: "/zitadel/grants", href: "/zitadel/grants" },
  ];

  return (
    <Card>
      <CardHeader
        title="Look inside Zitadel directly"
        note="What Zitadel holds, not what Syndra thinks it holds. Reading only; nothing here changes anything."
      />
      {rows.map((row) => (
        <div key={row.path} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
          <span className="min-w-[200px] flex-1 text-[14.5px]">{row.label}</span>
          <Mono className="w-[190px] shrink-0 truncate text-faint">{row.path}</Mono>
          {reachable ? (
            <ButtonLink href={row.href} size="sm">
              {row.open}
            </ButtonLink>
          ) : (
            <Button size="sm" disabled>
              {row.open}
            </Button>
          )}
        </div>
      ))}
      {!reachable && (
        <div className="row-divider border-dashed border-danger-line bg-danger-soft px-5 py-3 text-[13.5px] text-danger-text">
          All three are disabled while Zitadel is unreachable — they show live data, and there is
          none to show.
        </div>
      )}
    </Card>
  );
}

/**
 * The escape hatches, deliberately collapsed and deliberately loud.
 *
 * These write to the identity provider WITHOUT going through Syndra's ledger
 * or outbox, which means the next cache compile can quietly undo them and the
 * drift sweep will report them as unexplained access created by a stranger.
 * They exist because sometimes the only way out of a broken state is to reach
 * past the orchestration — but that is a last resort, and the screen says so
 * before it shows a single button.
 */
function UpstreamWrites({ reachable }: { reachable: boolean }) {
  const [open, setOpen] = useState(false);

  return (
    <div className="rounded-card border border-dashed border-danger-line">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        className="flex w-full flex-wrap items-center gap-3 px-5 py-4 text-left"
      >
        <span className="type-card-title text-danger-text">Change Zitadel directly (last resort)</span>
        <span className="flex-1" />
        <span className="text-[13.5px] text-faint">{open ? "Hide" : "Show"}</span>
      </button>

      {open && (
        <>
          <div className="danger-note mx-5 mb-4 px-4 py-3.5">
            <div className="text-[14.5px] font-semibold text-danger-text">
              Do not use these unless nothing else will do.
            </div>
            <p className="mt-1 max-w-[86ch] text-[14px] leading-[1.55] text-muted">
              Editing a project or a role here writes straight to Zitadel, so Syndra records nothing
              beyond one line in Audit. Two things follow. The next drift sweep — every six hours —
              lists what you changed under Drift, with nobody&rsquo;s name on it, for somebody to
              resolve by hand. And it will not appear in Change history. Do the same thing inside
              Syndra instead where you can: give access from a person&rsquo;s page, remove it from
              the role&rsquo;s member list, or edit roles under Access.
            </p>
          </div>

          <div className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
            <span className="min-w-[200px] flex-1 text-[14.5px]">
              Give, change or revoke one person&rsquo;s roles in Zitadel
            </span>
            <Mono className="w-[190px] shrink-0 truncate text-faint">
              /zitadel/users/&#123;id&#125;/grants
            </Mono>
            {reachable ? (
              <ButtonLink href="/zitadel/users" size="sm" variant="danger">
                Open people
              </ButtonLink>
            ) : (
              <Button size="sm" variant="danger" disabled>
                Open people
              </Button>
            )}
          </div>

          <div className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
            <span className="min-w-[200px] flex-1 text-[14.5px]">
              Create, rename or delete a role on a project
            </span>
            <Mono className="w-[190px] shrink-0 truncate text-faint">
              /zitadel/projects/&#123;id&#125;/roles
            </Mono>
            {reachable ? (
              <ButtonLink href="/zitadel/projects" size="sm" variant="danger">
                Open projects
              </ButtonLink>
            ) : (
              <Button size="sm" variant="danger" disabled>
                Open projects
              </Button>
            )}
          </div>

          {!reachable && (
            <div className="row-divider border-dashed border-danger-line bg-danger-soft px-5 py-3 text-[13.5px] text-danger-text">
              Disabled — Zitadel is unreachable, and these change it directly.
            </div>
          )}
        </>
      )}
    </div>
  );
}

function rotationHeadline(status: RotationStatus | undefined, error: unknown): string {
  if (error) return "Unknown";
  if (!status?.key_installed) return "Not set up";
  const threshold = status.threshold_days ?? 90;
  const age = status.age_days;
  if (age === undefined) return "Installed";
  const remaining = threshold - age;
  if (remaining <= 0) return "Due for replacement";
  // "Replace within", not "rotates in": nothing replaces this key by itself.
  return `Replace within ${remaining} day${remaining === 1 ? "" : "s"}`;
}
