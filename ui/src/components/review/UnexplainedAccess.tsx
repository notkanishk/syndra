"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserName } from "@/components/names";
import {
  useAttributeDrift,
  useDriftItems,
  useMarkExternalDrift,
  useReconcileNow,
  useRevokeDrift,
} from "@/lib/queries/useDrift";
import { useReconciliationDiff } from "@/lib/queries/useGrants";
import type { DriftItem } from "@/lib/queries/useGovernance";
import { Relative } from "@/components/ui/Time";

type Tab = "triage" | "reconciliation";
type Resolution = "attribute" | "revoke" | "external";

/**
 * Every row here has to make "what is this, and what happens if I revoke it"
 * answerable in about two seconds. That is the whole design constraint: the
 * three resolutions sit on the row, each says what it does to the access, and
 * the destructive one is an outline until it reaches its dialog.
 */
export function UnexplainedAccess() {
  const params = useSearchParams();
  const router = useRouter();
  const tab: Tab = params.get("tab") === "reconciliation" ? "reconciliation" : "triage";

  const drift = useDriftItems();
  const reconcile = useReconcileNow();
  const [pending, setPending] = useState<{ item: DriftItem; resolution: Resolution } | null>(null);

  const items = drift.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Unexplained access"
        meta="Access that exists in the identity provider which MkAuth never caused."
        actions={
          <Button
            isPending={reconcile.isPending}
            onClick={async () => {
              try {
                await reconcile.mutateAsync();
                toast.success("Scan finished.");
              } catch (error) {
                toast.error(error instanceof Error ? error.message : "The scan didn't run.");
              }
            }}
          >
            Scan now
          </Button>
        }
      />

      <div className="flex gap-2">
        {(["triage", "reconciliation"] as const).map((entry) => (
          <button
            key={entry}
            type="button"
            onClick={() => router.replace(entry === "triage" ? "?" : "?tab=reconciliation")}
            aria-current={tab === entry ? "page" : undefined}
            className={`rounded-pill px-4 py-2 text-[14.5px] transition-colors duration-150 ${
              tab === entry ? "bg-tint-3 font-semibold text-ink" : "text-muted hover:text-ink"
            }`}
          >
            {entry === "triage" ? "Triage queue" : "Reconciliation"}
          </button>
        ))}
      </div>

      {tab === "triage" ? (
        <Card>
          <CardHeader
            title="Needs a decision"
            count={items.length}
            tone="danger"
            note="Nothing here resolves on its own."
          />
          <ListStates
            isLoading={drift.isLoading}
            error={drift.error}
            isEmpty={items.length === 0}
            onRetry={() => drift.refetch()}
            errorTitle="Couldn't load the triage queue."
            skeleton={<RowSkeleton rows={5} label="Loading unexplained access" />}
            empty={
              <EmptyState
                title="Everything is explained."
                guidance="Every grant in the identity provider traces back to something MkAuth did."
              />
            }
          >
            {items.map((item) => (
              <div key={item.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
                <div className="min-w-[220px] flex-1">
                  <div className="text-[15px] font-semibold">
                    <UserName id={item.user_id} />
                  </div>
                  <div className="text-[13.5px] text-muted">
                    <ProjectName id={item.project_id} /> ·{" "}
                    {item.role_keys.map((key) => (
                      <Mono key={key} className="mr-1.5">
                        {key}
                      </Mono>
                    ))}
                  </div>
                </div>
                <Badge tone={item.drift_type === "zitadel_only" ? "danger" : "warn"}>
                  {item.drift_type === "zitadel_only" ? "Only upstream" : "Only in MkAuth"}
                </Badge>
                <div className="w-[150px] text-[13px] text-faint">
                  found <Relative iso={item.detected_at} />
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button size="sm" onClick={() => setPending({ item, resolution: "attribute" })}>
                    Adopt in MkAuth
                  </Button>
                  <Button size="sm" onClick={() => setPending({ item, resolution: "external" })}>
                    Owned elsewhere
                  </Button>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => setPending({ item, resolution: "revoke" })}
                  >
                    Revoke
                  </Button>
                </div>
              </div>
            ))}
          </ListStates>
        </Card>
      ) : (
        <Reconciliation />
      )}

      <ResolutionDialog pending={pending} onClose={() => setPending(null)} />
    </div>
  );
}

/**
 * The three resolutions, each with the consequence stated. Only Revoke takes
 * a solid destructive fill, and only inside this dialog.
 */
function ResolutionDialog({
  pending,
  onClose,
}: {
  pending: { item: DriftItem; resolution: Resolution } | null;
  onClose: () => void;
}) {
  const attribute = useAttributeDrift();
  const revoke = useRevokeDrift();
  const external = useMarkExternalDrift();

  if (!pending) return null;
  const { item, resolution } = pending;
  const busy = attribute.isPending || revoke.isPending || external.isPending;

  const copy = {
    attribute: {
      title: "Adopt this access in MkAuth?",
      lede: "MkAuth takes ownership of it. The access stays exactly as it is; from now on MkAuth explains it and manages its lifecycle.",
      confirm: "Adopt in MkAuth",
      variant: "accent" as const,
    },
    external: {
      title: "Is this owned by another system?",
      lede: "The access stays and stops being flagged. Use this when a known integration legitimately manages it — not to quiet something you haven't identified.",
      confirm: "Owned elsewhere",
      variant: "accent" as const,
    },
    revoke: {
      title: "Revoke this access?",
      lede: "The grant is removed from the identity provider. If a person is using it right now, they lose it at the next token refresh.",
      confirm: "Revoke access",
      variant: "dangerConfirm" as const,
    },
  }[resolution];

  return (
    <Modal open onClose={onClose} busy={busy} size="sm" labelledBy="triage-title">
      <ModalHeader title={copy.title} titleId="triage-title" lede={copy.lede} />
      <div className="px-6">
        <div className="rounded-inner bg-tint-1 px-4 py-3.5 text-[14.5px]">
          <UserName id={item.user_id} /> · <ProjectName id={item.project_id} />
          <div className="mt-1">
            {item.role_keys.map((key) => (
              <Mono key={key} className="mr-2 text-muted">
                {key}
              </Mono>
            ))}
          </div>
        </div>
      </div>
      <ModalFooter>
        <Button
          variant={copy.variant}
          isPending={busy}
          onClick={async () => {
            try {
              if (resolution === "attribute") {
                await attribute.mutateAsync({ id: item.id, body: { source: "direct" } });
              } else if (resolution === "revoke") {
                await revoke.mutateAsync({ id: item.id });
              } else {
                await external.mutateAsync({ id: item.id, body: {} });
              }
              toast.success("Resolved.");
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "That didn't go through.");
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
 * Reconciliation — the MkAuth ↔ Zitadel diff, relocated here from the retired
 * /grants route. It exists to spot drift before it widens, so it leads with
 * the two "only in" lists rather than a combined ledger.
 */
function Reconciliation() {
  const diff = useReconciliationDiff();
  const data = diff.data;

  const empty =
    !data ||
    (data.only_in_mkauth.length === 0 &&
      data.only_in_zitadel.length === 0 &&
      data.drift.length === 0);

  return (
    <div className="flex flex-col gap-[18px]">
      {data?.truncated && (
        <div className="warn-note px-5 py-3.5 text-[14px] text-warn-text">
          This comparison stopped early at the safety cap, so it is incomplete. Treat an empty
          section as &ldquo;nothing found so far&rdquo;, not as &ldquo;nothing exists&rdquo;.
        </div>
      )}

      <Card>
        <CardHeader
          title="Compared with the identity provider"
          note={
            data ? (
              <>
                generated <Relative iso={data.generated_at} />
              </>
            ) : undefined
          }
        />
        <ListStates
          isLoading={diff.isLoading}
          error={diff.error}
          isEmpty={empty}
          onRetry={() => diff.refetch()}
          errorTitle="Couldn't compare with the identity provider."
          skeleton={<RowSkeleton rows={4} avatar={false} label="Comparing" />}
          empty={
            <EmptyState
              title="The two sides agree."
              guidance="Every grant MkAuth expects exists upstream, and nothing upstream is unaccounted for."
            />
          }
        >
          <DiffSection
            label="Only in MkAuth"
            hint="MkAuth expects these, the identity provider doesn't have them. Usually a queued write that never drained."
            rows={(data?.only_in_mkauth ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.role_keys,
            }))}
          />
          <DiffSection
            label="Only upstream"
            hint="These exist in the identity provider with nothing in MkAuth explaining them. They are the triage queue's source."
            rows={(data?.only_in_zitadel ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.role_keys,
            }))}
          />
          <DiffSection
            label="Different on both sides"
            hint="Same person and project, different role sets."
            rows={(data?.drift ?? []).map((row) => ({
              userId: row.user_id,
              projectId: row.project_id,
              roles: row.only_in_zitadel.length > 0 ? row.only_in_zitadel : row.only_in_mkauth,
            }))}
          />
        </ListStates>
      </Card>
    </div>
  );
}

function DiffSection({
  label,
  hint,
  rows,
}: {
  label: string;
  hint: string;
  rows: Array<{ userId: string; projectId: string; roles: string[] }>;
}) {
  if (rows.length === 0) return null;
  return (
    <>
      <div className="row-divider px-5 pb-1.5 pt-3">
        <span className="type-label">
          {label} · {rows.length}
        </span>
        <p className="mt-1 max-w-[70ch] text-[13px] text-faint">{hint}</p>
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
