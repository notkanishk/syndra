"use client";

import Link from "next/link";
import { useState } from "react";

import { BundleVersions } from "@/components/bundles/BundleVersions";
import { FoldHeading } from "@/components/bundles/FoldHeading";
import { RoleRef, UserName } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Chip } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Tabs } from "@/components/ui/Tabs";
import { formatShortDate } from "@/lib/format";
import { useCrumb } from "@/lib/page-crumb";
import { useBundleVersions } from "@/lib/queries/useBundleVersions";
import { useBundle, useBundleRoles, useRetiredBundles, type BundleRow } from "@/lib/queries/useBundles";

type Tab = "history" | "gave" | "people";

/** "Retired 19 Oct 2026 by Sam": the one status a retired bundle or rule carries. */
export function RetiredStatus({ at, by }: { at?: string | null; by?: string }) {
  return (
    <span className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
      <Chip className="!py-1 text-[13px] text-muted">Retired {formatShortDate(at ?? "")}</Chip>
      {by && (
        <span>
          by <UserName id={by} />
        </span>
      )}
    </span>
  );
}

/**
 * A retired bundle's page. The list leaves it out, but its page still opens: it stopped being
 * given and keeps its history, so this reads what it gave and how it changed, with nothing to
 * press. Nobody has it — they lost what it gave when it was retired.
 */
export function RetiredBundlePage({ bundleId }: { bundleId: string }) {
  const one = useBundle(bundleId);
  const bundle = one.data?.retired_at ? one.data : null;
  useCrumb(bundle?.name);

  if (!bundle) {
    // A 404 is the page's own "no bundle here"; any other failure is a failure to retry.
    const failed = one.error && (one.error as { status?: number }).status !== 404 ? one.error : null;
    return (
      <div className="flex flex-col gap-7">
        <PageHeader eyebrow="Bundles" title="Bundle" lede="What it gives, who has it, and its history." />
        <Card>
          <ListStates
            isLoading={one.isLoading}
            error={failed}
            isEmpty
            onRetry={() => one.refetch()}
            errorTitle="Couldn't load this bundle."
            skeleton={<RowSkeleton rows={3} avatar={false} label="Loading the bundle" />}
            empty={
              <EmptyState
                title="There's no bundle here."
                guidance="Every bundle that exists is on the list, and the retired ones are at the end of it."
                action={{ label: "All bundles", href: "/bundles" }}
              />
            }
          >
            {null}
          </ListStates>
        </Card>
      </div>
    );
  }
  return <RetiredBundleView bundleId={bundleId} bundle={bundle} />;
}

function RetiredBundleView({ bundleId, bundle }: { bundleId: string; bundle: BundleRow }) {
  const [tab, setTab] = useState<Tab>("history");
  const versions = useBundleVersions(bundleId);
  const gave = useBundleRoles(bundleId, { published: true });
  const last = (versions.data ?? []).reduce<number>((best, v) => Math.max(best, v.version), 0);
  const name = bundle.name;

  return (
    <div className="flex flex-col gap-7">
      <PageHeader
        eyebrow="Bundles"
        title={name}
        meta={<RetiredStatus at={bundle.retired_at} by={bundle.retired_by} />}
        lede={
          bundle.description ||
          "It isn't given to anyone any more and can't be given again. What it gave and how it changed stay here to read."
        }
      />

      <div className="flex flex-col gap-[18px]">
        <Tabs<Tab>
          label={`${name}: its history, what it gave, and who had it`}
          value={tab}
          onSelect={setTab}
          options={[
            { value: "history", label: <TabLabel text="History" n={(versions.data ?? []).length} /> },
            { value: "gave", label: <TabLabel text="What it gave" n={(gave.data ?? []).length} /> },
            { value: "people", label: <TabLabel text="People" n={0} /> },
          ]}
        />

        {tab === "history" && <BundleVersions bundleId={bundleId} name={name} readOnly />}
        {tab === "gave" && (
          <Card>
            <CardHeader
              title="What it gave"
              count={(gave.data ?? []).length}
              note={last > 0 ? `In v${last}, its last version` : undefined}
            />
            <ListStates
              isLoading={gave.isLoading}
              error={gave.error}
              isEmpty={(gave.data ?? []).length === 0}
              onRetry={() => gave.refetch()}
              errorTitle={`Couldn't load what ${name} gave.`}
              skeleton={<RowSkeleton rows={3} avatar={false} label="Loading roles" />}
              empty={<EmptyState title={`${name} gave no roles.`} guidance="Its last version held none." />}
            >
              {(gave.data ?? []).map((role) => (
                <div
                  key={`${role.zitadel_project_id}:${role.zitadel_role_key}`}
                  className="row-divider px-5 py-3.5 text-[14.5px]"
                >
                  <RoleRef projectId={role.zitadel_project_id} roleKey={role.zitadel_role_key} />
                </div>
              ))}
            </ListStates>
          </Card>
        )}
        {tab === "people" && (
          <Card>
            <EmptyState
              title="Nobody has it."
              guidance="Everyone who had it lost what it gave when it was retired."
            />
          </Card>
        )}
      </div>
    </div>
  );
}

function TabLabel({ text, n }: { text: string; n: number }) {
  return (
    <>
      {text} <span className="text-faint">{n}</span>
    </>
  );
}

/**
 * The end of the bundles list: a quiet fold of the bundles that stopped being given. Closed, it
 * reads nothing; opened, it lists them, each a link to its page. Read-only.
 */
export function RetiredBundlesFold() {
  const [open, setOpen] = useState(false);
  const retired = useRetiredBundles();
  // Nothing retired, or not read yet: the list ends where it ends, with no empty fold to open.
  const n = retired.data?.length ?? 0;
  if (n === 0 && !retired.error) return null;
  return (
    <Card>
      <FoldHeading open={open} onClick={() => setOpen(!open)} title="Retired bundles" count={n ? String(n) : ""} picked={false} />
      {open && <RetiredBundleRows />}
    </Card>
  );
}

function RetiredBundleRows() {
  const retired = useRetiredBundles();
  const rows = retired.data ?? [];
  return (
    <ListStates
      isLoading={retired.isLoading}
      error={retired.error}
      isEmpty={rows.length === 0}
      onRetry={() => retired.refetch()}
      errorTitle="Couldn't load the retired bundles."
      skeleton={<RowSkeleton rows={2} avatar={false} label="Loading retired bundles" />}
      empty={<EmptyState title="Nothing has been retired." guidance="A bundle you retire is listed here, and its history stays readable." />}
    >
      {rows.map((b) => (
        <Link
          key={b.id}
          href={`/bundles/${encodeURIComponent(b.id)}`}
          className="row-divider flex flex-wrap items-baseline gap-x-4 gap-y-1 px-5 py-3.5 hover-tint"
        >
          <span className="min-w-0 flex-1 text-[14.5px] font-semibold">{b.name}</span>
          <span className="text-[13px] text-faint">Retired {formatShortDate(b.retired_at ?? "")}</span>
        </Link>
      ))}
    </ListStates>
  );
}
