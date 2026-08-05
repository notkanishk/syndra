"use client";

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
        title="Identity provider"
        meta={health.data?.domain ? <Mono>{health.data.domain}</Mono> : "Zitadel"}
      />

      {health.isLoading ? (
        <Card>
          <RowSkeleton rows={2} avatar={false} label="Checking the identity provider" />
        </Card>
      ) : health.error ? (
        <ErrorState
          title="Couldn't reach the health endpoint."
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
              ? `Last rotated ${formatLongDate(rotation.data.last_rotated_at)}`
              : "The key signs every claim-injection call. Rotating it invalidates the old one immediately."
          }
        />
        <StatCard
          label="Projects upstream"
          value={
            live
              ? `${health.data?.projects_total ?? projects.data?.length ?? 0}`
              : `${projects.data?.length ?? 0}`
          }
          tone="neutral"
          detail={live ? "As of the last successful read." : "From Syndra's own cache."}
        />
        <StatCard
          label="Mode"
          value={health.data?.mode === "live" ? "Live" : "Local policy only"}
          tone={health.data?.mode === "live" ? "neutral" : "warn"}
          detail={
            health.data?.mode === "live"
              ? "Syndra reads and writes the real directory."
              : "No management client is configured, so nothing Syndra decides reaches a machine."
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
        note="Zitadel signs every claim-injection call with it. Zitadel never expires it — rotation is a decision, not a schedule."
      />

      <div className="flex flex-col gap-4 px-5 py-4">
        <p className="max-w-[84ch] text-[14.5px] leading-[1.6]">{rotationSentence(status, error)}</p>

        {loading ? (
          <RowSkeleton rows={1} avatar={false} label="Reading the key's rotation state" />
        ) : !status?.key_installed && !error ? (
          <CommandBlock
            tone="warn"
            command="make zitadel-actions-register"
            caption="No key is installed, so nothing is being verified. Registering the Action targets mints one."
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
            caption="Run this in a terminal on the deployment host. This panel does not rotate anything itself — the restart afterwards is yours to make, and a half-finished rotation is worse than an old key."
            steps={[
              <>
                Rotation writes the new key to{" "}
                <Mono>zitadel/actions/.action-signing-key.&lt;target&gt;</Mono> and backs up the
                old one alongside it.
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
                Verify: <Mono>make zitadel-actions-verify</Mono>.
              </>,
            ]}
          />
        )}

        <p className="max-w-[84ch] text-[13.5px] leading-[1.55] text-muted">
          Between the rotation and the restart, Syndra rejects Action calls as unsigned and Zitadel
          issues tokens with stock claims only. Nobody is locked out — custom claims simply go
          missing for the length of the gap, so keep it short and do it outside a session.
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
    return "Couldn't read the key's rotation state, so its age is unknown. The key itself is unaffected — this is the status endpoint, not the key.";
  }
  if (!status) return "";

  if (!status.key_installed) {
    return "No signing key is installed, so signature verification is passing every Action request through unchecked. Anything that can reach the endpoint can shape a token. Install a key before this deployment carries real access.";
  }

  const threshold = status.threshold_days ?? 90;
  const age = status.age_days;

  if (age === undefined) {
    return `The key is installed and verifying, but its rotation date is unset, unparseable, or in the future — so its age can't be checked against the ${threshold}-day threshold. Set ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT to the value rotate.sh prints.`;
  }
  if (status.status === "stale") {
    return `The key is ${age} days old — past twice the ${threshold}-day threshold. Rotate it now.`;
  }
  if (status.status === "warn") {
    return `The key is ${age} days old, over the ${threshold}-day threshold. Schedule a rotation.`;
  }
  return `The key is ${age} days old, within the ${threshold}-day threshold. Nothing to do.`;
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
          Syndra is reading the live directory. Queued writes drain as they are confirmed.
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
          ? "Not configured — Syndra is running on local policy only."
          : "Unreachable — the last call to the identity provider failed."}
      </div>
      <p className="mt-1 max-w-[80ch] text-[14px] leading-[1.55] text-muted">
        {detail ||
          "Syndra is serving its own cache. Nothing is lost: writes stay queued and in order until it returns."}
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
  const rows: Array<{ label: string; path: string; href: string }> = [
    { label: "Projects and their roles", path: "/zitadel/projects", href: "/zitadel/projects" },
    { label: "Users", path: "/zitadel/users", href: "/zitadel/users" },
    { label: "Grants", path: "/zitadel/grants", href: "/zitadel/grants" },
  ];

  return (
    <Card>
      <CardHeader
        title="Inspect upstream directly"
        note="What the identity provider holds, not what Syndra thinks it holds."
      />
      {rows.map((row) => (
        <div key={row.path} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
          <span className="min-w-[200px] flex-1 text-[14.5px]">{row.label}</span>
          <Mono className="w-[190px] shrink-0 truncate text-faint">{row.path}</Mono>
          {reachable ? (
            <ButtonLink href={row.href} size="sm">
              Open
            </ButtonLink>
          ) : (
            <Button size="sm" disabled>
              Open
            </Button>
          )}
        </div>
      ))}
      {!reachable && (
        <div className="row-divider border-dashed border-danger-line bg-danger-soft px-5 py-3 text-[13.5px] text-danger-text">
          All three are disabled while the provider is unreachable — they read live, and there is
          nothing to read.
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
        <span className="type-card-title text-danger-text">Direct writes to the provider</span>
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
              Everything here bypasses Syndra entirely — no ledger row, no outbox entry, no audit
              trail tying the change to a decision. Three things follow from that. The next cache
              compile can overwrite what you did. The drift sweep will report it as unexplained
              access created by somebody it cannot name. And nobody reading Change history
              afterwards will find out it happened. Prefer the equivalent action in Syndra: grant
              from a person&rsquo;s page, remove from role membership, edit roles under Access.
            </p>
          </div>

          <div className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
            <span className="min-w-[200px] flex-1 text-[14.5px]">
              Assign, change or remove a grant on one person
            </span>
            <Mono className="w-[190px] shrink-0 truncate text-faint">
              /zitadel/users/&#123;id&#125;/grants
            </Mono>
            {reachable ? (
              <ButtonLink href="/zitadel/users" size="sm" variant="danger">
                Open
              </ButtonLink>
            ) : (
              <Button size="sm" variant="danger" disabled>
                Open
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
                Open
              </ButtonLink>
            ) : (
              <Button size="sm" variant="danger" disabled>
                Open
              </Button>
            )}
          </div>

          {!reachable && (
            <div className="row-divider border-dashed border-danger-line bg-danger-soft px-5 py-3 text-[13.5px] text-danger-text">
              Disabled — the provider is unreachable, and these write to it live.
            </div>
          )}
        </>
      )}
    </div>
  );
}

function rotationHeadline(status: RotationStatus | undefined, error: unknown): string {
  if (error) return "Unknown";
  if (!status?.key_installed) return "Not installed";
  const threshold = status.threshold_days ?? 90;
  const age = status.age_days;
  if (age === undefined) return status.status ?? "Installed";
  const remaining = threshold - age;
  if (remaining <= 0) return "Overdue";
  return `Rotates in ${remaining} day${remaining === 1 ? "" : "s"}`;
}
