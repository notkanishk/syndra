"use client";

import { useMemo, useState } from "react";

import { ProjectName, UserName } from "@/components/names";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { JsonView } from "@/components/ui/JsonView";
import { Modal } from "@/components/ui/Modal";
import { Pulse } from "@/components/ui/Pulse";
import { useIntents, type IntentStatus, type ProvisioningIntent } from "@/lib/queries/useIntents";
import {
  useOnboardingTriggers,
  useWebhookEvents,
  type OnboardingTriggerRow,
  type WebhookEventRow,
} from "@/lib/queries/useOperations";

type TabKey = "intents" | "webhook" | "onboarding";

const TABS: Array<{ key: TabKey; label: string; description: string }> = [
  {
    key: "intents",
    label: "Intents",
    description: "Provisioning intents claimed by the sync service.",
  },
  {
    key: "webhook",
    label: "Webhook events",
    description: "Zitadel events received by the backend.",
  },
  {
    key: "onboarding",
    label: "Onboarding triggers",
    description: "Welcome-bundle assignment events.",
  },
];

const INTENT_STATUSES: IntentStatus[] = ["pending", "in_flight", "succeeded", "failed"];

function pulseTone(status: string): "success" | "warn" | "error" | "info" {
  if (status === "succeeded" || status === "completed" || status === "processed") return "success";
  if (status === "failed") return "error";
  if (status === "in_flight" || status === "pending") return "warn";
  return "info";
}

function relativeAge(timestamp: string): string {
  if (!timestamp) return "—";
  const ms = Date.now() - new Date(timestamp).getTime();
  if (Number.isNaN(ms) || ms < 0) return "—";
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function truncate(text: string | undefined | null, max = 80): string {
  if (!text) return "";
  return text.length > max ? `${text.slice(0, max)}…` : text;
}

interface PayloadModalProps {
  payload: unknown;
  title: string;
  onClose: () => void;
}

function PayloadModal({ payload, title, onClose }: PayloadModalProps) {
  return (
    <Modal open onClose={onClose} labelledBy="payload-modal-title" size="lg">
      <Eyebrow>Payload</Eyebrow>
      <h3 id="payload-modal-title" className="text-base font-semibold text-on-surface mt-1">
        {title}
      </h3>
      <div className="mt-4 max-h-[60vh] overflow-auto rounded-card border border-outline-variant bg-surface-container-lowest p-4">
        <JsonView value={payload} />
      </div>
      <div className="mt-4 flex justify-end">
        <Button type="button" variant="ghost" size="sm" onClick={onClose}>
          Close
        </Button>
      </div>
    </Modal>
  );
}

/**
 * Operations queues client. Exposes three live-polling views — provisioning
 * intents, Zitadel webhook events, and onboarding triggers — all with the
 * same row affordance: status `<Pulse/>`, age, target identification (via
 * `<UserName/>` and `<ProjectName/>`), truncated last-error tooltip, and a
 * "View payload" button that opens the raw record in a `<JsonView/>` modal
 * so operators can debug failures without leaving the page.
 *
 * Refetch cadence is 5s per the per-resource hooks; no extra polling logic
 * lives here — backgrounded tabs pause polling automatically via the hooks.
 */
export default function OperationsClient() {
  const [activeTab, setActiveTab] = useState<TabKey>("intents");
  const [statusFilter, setStatusFilter] = useState<IntentStatus | "all">("all");
  const [payloadView, setPayloadView] = useState<{
    title: string;
    data: unknown;
  } | null>(null);

  const intentsQuery = useIntents(statusFilter === "all" ? {} : { status: statusFilter });
  const webhookQuery = useWebhookEvents();
  const onboardingQuery = useOnboardingTriggers();

  const intents = useMemo(() => intentsQuery.data ?? [], [intentsQuery.data]);
  const webhook = useMemo(() => webhookQuery.data ?? [], [webhookQuery.data]);
  const onboarding = useMemo(() => onboardingQuery.data ?? [], [onboardingQuery.data]);

  // Status filter pill row — disabled for tabs whose backend doesn't support
  // server-side filtering (webhook events accept ?status= but onboarding does
  // not). Keeping the UI uniform but the wiring honest avoids client-side
  // filtering of a paged-server payload.
  const filterPillsAvailable = activeTab === "intents";

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow tone="primary">Operations</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Live operator queues
        </h1>
        <p className="text-on-surface-variant mt-2 max-w-2xl">
          Visibility into the provisioning pipeline — intents en route to the
          sync service, webhook events received from Zitadel, and onboarding
          triggers awaiting welcome-bundle assignment. Refreshes every 5s.
        </p>
      </header>

      <Card variant="glass">
        <div role="tablist" aria-label="Operations queues" className="flex flex-wrap gap-1 border-b border-outline-variant pb-3 mb-4">
          {TABS.map((tab) => (
            <button
              key={tab.key}
              role="tab"
              type="button"
              aria-selected={activeTab === tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`rounded-full px-3 py-1.5 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container ${
                activeTab === tab.key
                  ? "bg-primary-container/20 text-primary-container"
                  : "text-on-surface-variant hover:bg-surface-container"
              }`}
            >
              {tab.label}
              {tab.key === "intents" && intents.length > 0 ? (
                <span className="ml-1.5 text-[10px]">({intents.length})</span>
              ) : null}
              {tab.key === "webhook" && webhook.length > 0 ? (
                <span className="ml-1.5 text-[10px]">({webhook.length})</span>
              ) : null}
              {tab.key === "onboarding" && onboarding.length > 0 ? (
                <span className="ml-1.5 text-[10px]">({onboarding.length})</span>
              ) : null}
            </button>
          ))}
        </div>

        {filterPillsAvailable && (
          <div className="flex flex-wrap items-center gap-2 mb-4">
            <Eyebrow tone="muted">Status</Eyebrow>
            <button
              type="button"
              onClick={() => setStatusFilter("all")}
              aria-pressed={statusFilter === "all"}
              className={`rounded-full border px-2.5 py-1 text-[11px] transition-colors ${
                statusFilter === "all"
                  ? "border-primary-container bg-primary-container/15 text-primary-container"
                  : "border-outline-variant text-on-surface-variant hover:border-primary-container/50"
              }`}
            >
              All
            </button>
            {INTENT_STATUSES.map((status) => (
              <button
                key={status}
                type="button"
                onClick={() => setStatusFilter(status)}
                aria-pressed={statusFilter === status}
                className={`rounded-full border px-2.5 py-1 text-[11px] capitalize transition-colors ${
                  statusFilter === status
                    ? "border-primary-container bg-primary-container/15 text-primary-container"
                    : "border-outline-variant text-on-surface-variant hover:border-primary-container/50"
                }`}
              >
                {status.replace("_", " ")}
              </button>
            ))}
          </div>
        )}

        {activeTab === "intents" && (
          <IntentsTable
            rows={intents}
            loading={intentsQuery.isLoading}
            onView={(row) =>
              setPayloadView({ title: `Intent ${row.idempotency_key}`, data: row })
            }
          />
        )}
        {activeTab === "webhook" && (
          <WebhookTable
            rows={webhook}
            loading={webhookQuery.isLoading}
            onView={(row) =>
              setPayloadView({ title: `Webhook ${row.event_type} (${row.idempotency_key})`, data: row })
            }
          />
        )}
        {activeTab === "onboarding" && (
          <OnboardingTable
            rows={onboarding}
            loading={onboardingQuery.isLoading}
            onView={(row) =>
              setPayloadView({ title: `Onboarding ${row.source} (${row.idempotency_key})`, data: row })
            }
          />
        )}
      </Card>

      {payloadView && (
        <PayloadModal
          payload={payloadView.data}
          title={payloadView.title}
          onClose={() => setPayloadView(null)}
        />
      )}
    </div>
  );
}

interface QueueTableProps<T> {
  rows: T[];
  loading: boolean;
  onView: (row: T) => void;
}

function emptyState(message: string) {
  return (
    <div className="rounded-card border border-dashed border-outline-variant bg-surface-container-low px-4 py-8 text-center">
      <p className="text-sm text-on-surface-variant">{message}</p>
    </div>
  );
}

function IntentsTable({ rows, loading, onView }: QueueTableProps<ProvisioningIntent>) {
  if (loading && rows.length === 0) {
    return <p className="text-sm text-on-surface-variant">Loading intents…</p>;
  }
  if (rows.length === 0) {
    return emptyState("No intents matching this filter.");
  }
  return (
    <ul className="divide-y divide-outline-variant rounded-card border border-outline-variant bg-surface-container-low">
      {rows.map((row) => (
        <li key={row.id} className="px-4 py-3 flex items-center gap-3">
          <Pulse variant={pulseTone(row.status)} ariaLabel={row.status} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-sm font-semibold text-on-surface">{row.action}</span>
              <Badge variant="outline" className="text-[10px] capitalize">{row.status.replace("_", " ")}</Badge>
              <span className="text-xs text-on-surface-variant truncate">
                <UserName id={row.target_uid} fallback={row.target_uid} />
                {" · "}
                <ProjectName id={row.source_project} fallback={row.source_project} />
                {" : "}
                <span className="font-mono">{row.source_role}</span>
              </span>
            </div>
            {row.error_message && (
              <p className="mt-1 text-[11px] text-[var(--error)] truncate" title={row.error_message}>
                {truncate(row.error_message, 120)}
              </p>
            )}
          </div>
          <span className="text-[11px] text-on-surface-variant whitespace-nowrap">
            {relativeAge(row.created_at)}
          </span>
          <Button type="button" variant="ghost" size="sm" onClick={() => onView(row)}>
            Payload
          </Button>
        </li>
      ))}
    </ul>
  );
}

function WebhookTable({ rows, loading, onView }: QueueTableProps<WebhookEventRow>) {
  if (loading && rows.length === 0) {
    return <p className="text-sm text-on-surface-variant">Loading webhook events…</p>;
  }
  if (rows.length === 0) {
    return emptyState("No webhook events recorded yet.");
  }
  return (
    <ul className="divide-y divide-outline-variant rounded-card border border-outline-variant bg-surface-container-low">
      {rows.map((row) => (
        <li key={row.id} className="px-4 py-3 flex items-center gap-3">
          <Pulse variant={pulseTone(row.status)} ariaLabel={row.status} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-sm font-semibold text-on-surface">{row.event_type}</span>
              <Badge variant="outline" className="text-[10px] capitalize">{row.status}</Badge>
              <span className="text-xs text-on-surface-variant truncate">
                <UserName id={row.user_id} fallback={row.user_id} />
                {" · "}
                <ProjectName id={row.source_project} fallback={row.source_project} />
                {row.role_key ? ` : ${row.role_key}` : ""}
              </span>
            </div>
            {row.error_message && (
              <p className="mt-1 text-[11px] text-[var(--error)] truncate" title={row.error_message}>
                {truncate(row.error_message, 120)}
              </p>
            )}
          </div>
          <span className="text-[11px] text-on-surface-variant whitespace-nowrap">
            {relativeAge(row.created_at)}
          </span>
          <Button type="button" variant="ghost" size="sm" onClick={() => onView(row)}>
            Payload
          </Button>
        </li>
      ))}
    </ul>
  );
}

function OnboardingTable({ rows, loading, onView }: QueueTableProps<OnboardingTriggerRow>) {
  if (loading && rows.length === 0) {
    return <p className="text-sm text-on-surface-variant">Loading onboarding triggers…</p>;
  }
  if (rows.length === 0) {
    return emptyState("No onboarding triggers fired yet.");
  }
  return (
    <ul className="divide-y divide-outline-variant rounded-card border border-outline-variant bg-surface-container-low">
      {rows.map((row) => (
        <li key={row.id} className="px-4 py-3 flex items-center gap-3">
          <Pulse variant={pulseTone(row.status)} ariaLabel={row.status} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-sm font-semibold text-on-surface capitalize">{row.source}</span>
              <Badge variant="outline" className="text-[10px] capitalize">{row.status}</Badge>
              <span className="text-xs text-on-surface-variant truncate">
                <UserName id={row.user_id} fallback={row.user_id} />
              </span>
            </div>
            {row.error_message && (
              <p className="mt-1 text-[11px] text-[var(--error)] truncate" title={row.error_message}>
                {truncate(row.error_message, 120)}
              </p>
            )}
          </div>
          <span className="text-[11px] text-on-surface-variant whitespace-nowrap">
            {relativeAge(row.created_at)}
          </span>
          <Button type="button" variant="ghost" size="sm" onClick={() => onView(row)}>
            Payload
          </Button>
        </li>
      ))}
    </ul>
  );
}
