"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { ErrorState, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
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
 * the sentence tells them what broke, when it last worked, and what MkAuth is
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
          detail={live ? "As of the last successful read." : "From MkAuth's own cache."}
        />
        <StatCard
          label="Mode"
          value={health.data?.mode === "live" ? "Live" : "Local policy only"}
          tone={health.data?.mode === "live" ? "neutral" : "warn"}
          detail={
            health.data?.mode === "live"
              ? "MkAuth reads and writes the real directory."
              : "No management client is configured, so nothing MkAuth decides reaches a machine."
          }
        />
      </div>

      <UpstreamInspection reachable={live} />
      <UpstreamWrites reachable={live} />
    </div>
  );
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
      <div className="accent-note px-5 py-4">
        <div className="text-[15px] font-semibold">
          Reachable — answered in {latency ?? 0}ms.
        </div>
        <p className="mt-1 max-w-[80ch] text-[14px] leading-[1.55] text-muted">
          MkAuth is reading the live directory. Queued writes drain as they are confirmed.
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
          ? "Not configured — MkAuth is running on local policy only."
          : "Unreachable — the last call to the identity provider failed."}
      </div>
      <p className="mt-1 max-w-[80ch] text-[14px] leading-[1.55] text-muted">
        {detail ||
          "MkAuth is serving its own cache. Nothing is lost: writes stay queued and in order until it returns."}
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
        note="What the identity provider holds, not what MkAuth thinks it holds."
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
 * These write to the identity provider WITHOUT going through MkAuth's ledger
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
              Everything here bypasses MkAuth entirely — no ledger row, no outbox entry, no audit
              trail tying the change to a decision. Three things follow from that. The next cache
              compile can overwrite what you did. The drift sweep will report it as unexplained
              access created by somebody it cannot name. And nobody reading Change history
              afterwards will find out it happened. Prefer the equivalent action in MkAuth: grant
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
