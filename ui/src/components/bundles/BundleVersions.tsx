"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { RoleRef, UserAvatar, UserName } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge } from "@/components/ui/Badge";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { ReadFreshness, readStateOfQuery } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { formatShortDate } from "@/lib/format";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useSetBundleInclude, type BundleRow } from "@/lib/queries/useBundles";
import {
  type BundleIncluder,
  useApplyMoveHolders,
  useBundleHolders,
  type BundleHolder,
  useBundleVersions,
  useRehearseMoveHolders,
  type BundleVersion,
} from "@/lib/queries/useBundleVersions";
import dynamic from "next/dynamic";

// Opened on demand, so it stays out of the page's first load.
const RehearsalDialog = dynamic(() => import("@/components/ui/RehearsalDialog").then((m) => m.RehearsalDialog));

/**
 * A bundle's history, and who is standing on each part of it.
 *
 * The holder count is what makes this a working surface rather than a
 * changelog. "v4 · 3 people, v2 · 11 people" is a fact about the makerspace an
 * operator has to act on; a list of version numbers with dates is not.
 */
export function BundleVersions({
  bundleId,
  name,
  readOnly = false,
}: {
  bundleId: string;
  name: string;
  /** A retired bundle: its versions to read, with nobody on them and nothing to move. */
  readOnly?: boolean;
}) {
  const versions = useBundleVersions(bundleId);
  const holders = useBundleHolders(bundleId);
  const [moving, setMoving] = useState<BundleVersion | null>(null);

  const rows = useMemo(() => versions.data ?? [], [versions.data]);
  const latest = rows[0];

  // Everyone who is not on the newest version. The set the "catch them up"
  // action operates on, and the number the header leads with.
  const behind = useMemo(
    () => (holders.data ?? []).filter((h) => latest && h.version_id !== latest.id),
    [holders.data, latest],
  );

  return (
    <Card>
      <CardHeader
        title="Versions"
        count={rows.length}
        note={
          readOnly
            ? "Kept as they were when it was retired"
            : behind.length > 0
              ? `${behind.length} ${behind.length === 1 ? "person is" : "people are"} on an older version`
              : "Everybody is on the latest version"
        }
        action={
          !readOnly && behind.length > 0 && latest ? (
            <Button size="sm" onClick={() => setMoving(latest)}>
              Move everyone to v{latest.version}
            </Button>
          ) : undefined
        }
      />
      <p className="row-divider px-5 pb-3.5 text-[13.5px] leading-[1.5] text-muted">
        {readOnly
          ? `Everyone who had ${name} lost what it gave when it was retired. Its versions stay here to read.`
          : `Each person holds a specific version of ${name}. Publishing a new version does not move them unless you choose to.`}
      </p>

      <ListStates
        isLoading={versions.isLoading}
        error={versions.error}
        isEmpty={rows.length === 0}
        onRetry={() => versions.refetch()}
        errorTitle="Couldn't load this bundle's versions."
        skeleton={<RowSkeleton rows={3} avatar={false} label="Loading versions" />}
        empty={
          <EmptyState
            title="No versions yet."
            guidance="A version is written the first time this bundle is published."
          />
        }
      >
        {rows.map((version, index) => (
          <div key={version.id} className="row-divider px-5 py-3.5">
            <div className="flex flex-wrap items-baseline gap-3">
              <span className="font-display text-[17px] leading-none">v{version.version}</span>
              {index === 0 ? (
                <Badge tone={readOnly ? "neutral" : "accent"}>{readOnly ? "Last" : "Current"}</Badge>
              ) : !readOnly && version.holder_count > 0 ? (
                // Only an older version with people ON it is worth marking.
                // Marking every superseded version would make the marking
                // meaningless, and the empty ones are just history.
                <Badge tone="warn">
                  {version.holder_count} {version.holder_count === 1 ? "person" : "people"} still
                  on this version
                </Badge>
              ) : null}
              <span className="flex-1" />
              <span className="text-[13px] text-faint">
                {version.published_by === "system" ? "Syndra (automatic)" : <UserName id={version.published_by} />}
                {" · "}
                <Relative iso={version.published_at} />
              </span>
            </div>

            {version.note && (
              <p className="mt-1 max-w-[70ch] text-[13.5px] leading-[1.5] text-muted">
                {version.note}
              </p>
            )}

            <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[13.5px] text-muted">
              {(version.roles ?? []).length === 0 ? (
                <span className="text-faint">Contained no roles.</span>
              ) : (
                (version.roles ?? []).map((role) => (
                  <RoleRef
                    key={`${role.zitadel_project_id}:${role.zitadel_role_key}`}
                    projectId={role.zitadel_project_id}
                    roleKey={role.zitadel_role_key}
                  />
                ))
              )}
            </div>

            {!readOnly && version.holder_count > 0 && index > 0 && (
              <div className="mt-2.5">
                <Button size="sm" onClick={() => setMoving(rows[0])}>
                  Move these {version.holder_count} {version.holder_count === 1 ? "person" : "people"} to v{rows[0].version}
                </Button>
              </div>
            )}
          </div>
        ))}
      </ListStates>

      {moving && (
        <MoveHoldersDialog
          bundleId={bundleId}
          name={name}
          target={moving}
          userIds={
            // Everyone not already on the target. Moving somebody onto the
            // version they are on is a no-op the plan would have to explain,
            // and a plan full of no-ops buries the rows that matter.
            (holders.data ?? [])
              .filter((h) => h.version_id !== moving.id)
              .map((h) => h.user_id)
          }
          onClose={() => setMoving(null)}
        />
      )}
    </Card>
  );
}

/**
 * The People tab: who has this bundle, grouped by the version they're on,
 * with the move for anyone behind.
 */
export function BundlePeople({ bundleId, name }: { bundleId: string; name: string }) {
  const versions = useBundleVersions(bundleId);
  const holders = useBundleHolders(bundleId);
  const [moving, setMoving] = useState<BundleVersion | null>(null);
  const rows = useMemo(() => versions.data ?? [], [versions.data]);
  return (
    <div className="flex flex-col gap-3.5">
      {(holders.data ?? []).length > 0 && (
        <ReadFreshness state={readStateOfQuery(holders)} subject={`Who has ${name}`} className="px-5" />
      )}
      <Card>
        <ListStates
          isLoading={holders.isLoading || versions.isLoading}
          error={holders.error ?? versions.error}
          isEmpty={(holders.data ?? []).length === 0}
          onRetry={() => holders.refetch()}
          errorTitle={`Couldn't load who has ${name}.`}
          skeleton={<RowSkeleton rows={4} label="Loading people" />}
          empty={
            <EmptyState
              title={`Nobody has ${name} yet.`}
              guidance="Add people from here, or give it to someone from their page."
            />
          }
        >
          <Holders bundleId={bundleId} onMoveTo={(v) => setMoving(v)} versions={rows} />
        </ListStates>
        {moving && (
          <MoveHoldersDialog
            bundleId={bundleId}
            name={name}
            target={moving}
            userIds={(holders.data ?? []).filter((h) => h.version_id !== moving.id).map((h) => h.user_id)}
            onClose={() => setMoving(null)}
          />
        )}
      </Card>
    </div>
  );
}

/** Who holds this bundle, grouped by the version they are standing on. */
function Holders({
  bundleId,
  versions,
  onMoveTo,
}: {
  bundleId: string;
  versions: BundleVersion[];
  onMoveTo: (version: BundleVersion) => void;
}) {
  const holders = useBundleHolders(bundleId);
  const rows = holders.data ?? [];
  if (rows.length === 0) return null;

  const byVersion = new Map<number, typeof rows>();
  for (const holder of rows) {
    byVersion.set(holder.version, [...(byVersion.get(holder.version) ?? []), holder]);
  }
  const latest = versions[0];

  // Grouped by version, newest first (Bundles 6): each group says how many,
  // an older one says it's behind and offers the move; each person says how
  // they got it (B7) and since when.
  return (
    <div className="flex flex-col">
      {Array.from(byVersion.entries())
        .sort((a, b) => b[0] - a[0])
        .map(([version, people]) => {
          const behind = latest !== undefined && latest.version !== version;
          return (
            <section key={version} id={`on-v${version}`} className="scroll-mt-6 border-t border-line first:border-t-0">
              <div className="flex flex-wrap items-center gap-3 px-5 pb-2 pt-4">
                <h3 className="type-card-title">On v{version}</h3>
                <span className={`text-[13px] ${behind ? "font-semibold text-warn-text" : "text-faint"}`}>
                  {people.length} {people.length === 1 ? "person" : "people"}
                  {behind ? ` · v${latest.version} is the latest` : ""}
                </span>
                {behind && (
                  <Button size="sm" className="ml-auto" onClick={() => onMoveTo(latest)}>
                    Move {people.length === 1 ? "them" : `these ${people.length}`} to v{latest.version}
                  </Button>
                )}
              </div>
              {people.map((holder) => (
                <div key={holder.user_id} className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-1 px-5 py-3">
                  <span className="flex min-w-[min(220px,100%)] flex-1 items-center gap-3">
                    <UserAvatar id={holder.user_id} size="row" />
                    <Link
                      href={`/users/${encodeURIComponent(holder.user_id)}`}
                      className="text-[14.5px] font-semibold text-ink link-hover"
                    >
                      <UserName id={holder.user_id} />
                    </Link>
                  </span>
                  <span className="min-w-0 flex-1 text-[13.5px] text-muted">{howTheyGotIt(holder)}</span>
                  <span className="w-[110px] text-right text-[13.5px] text-muted">
                    {formatShortDate(holder.assigned_at)}
                  </span>
                </div>
              ))}
            </section>
          );
        })}
    </div>
  );
}

/** "Given by Kanishk Singh" · "Approved request" · "Moved from v2" · "Given when they joined". */
function howTheyGotIt(holder: BundleHolder) {
  switch (holder.source) {
    case "moved":
      return holder.moved_from_version ? `Moved from v${holder.moved_from_version}` : "Moved from an older version";
    case "request":
      return "Approved request";
    case "welcome":
      return "Given when they joined";
    case "direct":
      return holder.assigned_by ? (
        <>
          Given by <UserName id={holder.assigned_by} fallback="someone" />
        </>
      ) : (
        "Given by hand"
      );
    default:
      return null;
  }
}

/**
 * Moving holders between versions, rehearsed. Moving somebody BACKWARDS is a
 * legitimate answer and revokes, which is why this uses the same destructive
 * treatment the other revoking surfaces do rather than reading as a tidy-up.
 */
function MoveHoldersDialog({
  bundleId,
  name,
  target,
  userIds,
  onClose,
}: {
  bundleId: string;
  name: string;
  target: BundleVersion;
  userIds: string[];
  onClose: () => void;
}) {
  const rehearse = useRehearseMoveHolders(bundleId);
  const apply = useApplyMoveHolders(bundleId);

  return (
    <RehearsalDialog
      title={`Move to ${name} v${target.version}`}
      lede={`${userIds.length} ${userIds.length === 1 ? "person" : "people"} would move to v${target.version}. Their access changes to whatever v${target.version} contains — anything only their current version gave them is revoked (their access to it ends).`}
      noun={["person", "people"]}
      destructive
      // A move whose rows all resolve to "no change to their access" still
      // moves the PIN, which is what clears the stale-holder count this screen
      // leads with. Catching everybody up is the act; nobody gaining or losing
      // a role is a good outcome of it, not a reason to refuse it.
      definitionLabel={`Move ${userIds.length} ${userIds.length === 1 ? "person" : "people"} to v${target.version}`}
      onRehearse={(acknowledgeScope) =>
        rehearse.mutateAsync({
          version_id: target.id,
          user_ids: userIds,
          acknowledge_scope: acknowledgeScope,
        })
      }
      onApply={(planId) =>
        apply.mutateAsync({ version_id: target.id, user_ids: userIds, plan_id: planId || undefined })
      }
      onClose={onClose}
    />
  );
}

/**
 * Through other bundles (Bundles 6): the bundles that include this one, each
 * a square tile with the version it includes and its people. "Update to vN"
 * puts the latest into that bundle's working copy; its people get it when it
 * publishes, rehearsed there as usual.
 */
export function ThroughOtherBundles({
  name,
  bundleId,
  latest,
  includers,
  bundles,
}: {
  name: string;
  bundleId: string;
  latest: number;
  includers: BundleIncluder[];
  bundles: BundleRow[];
}) {
  const people = includers.reduce((n, i) => n + i.people, 0);
  return (
    <section id="through-bundles" className="card scroll-mt-6 overflow-hidden" style={{ borderStyle: "dashed" }}>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-5 py-4">
        <h3 className="type-card-title">Through other bundles</h3>
        <span className="text-[13.5px] text-faint">
          {people} {people === 1 ? "person" : "people"} in {includers.length}{" "}
          {includers.length === 1 ? "bundle" : "bundles"}
        </span>
      </div>
      <p className="row-divider px-5 py-2.5 text-[13.5px] leading-[1.55] text-muted [text-wrap:pretty]">
        These bundles include {name}, so their people have it too. Each is pinned to a version. Updating one
        puts v{latest} into its next version; its people get it when that bundle publishes, with the usual
        review.
      </p>
      {includers.map((includer) => (
        <IncluderTile
          key={includer.bundle_id}
          childId={bundleId}
          includer={includer}
          latest={latest}
          purpose={bundles.find((b) => b.id === includer.bundle_id)?.description}
        />
      ))}
    </section>
  );
}

function IncluderTile({
  childId,
  includer,
  latest,
  purpose,
}: {
  childId: string;
  includer: BundleIncluder;
  latest: number;
  purpose?: string;
}) {
  const update = useSetBundleInclude(includer.bundle_id);
  const [failure, setFailure] = useState<Outcome | null>(null);
  const pinned = includer.included_version;
  const behind = pinned > 0 && pinned < latest;
  const waiting = includer.draft_version === latest;
  const gap = latest - pinned;
  const href = `/bundles/${encodeURIComponent(includer.bundle_id)}`;
  const tile = includer.name
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");

  return (
    <div className="row-divider px-5 py-3">
      <div className="flex flex-wrap items-center gap-x-[18px] gap-y-2">
        <span className="flex min-w-0 flex-[1_1_260px] items-center gap-3">
          <span
            aria-hidden
            className="inline-flex size-[34px] flex-none items-center justify-center rounded-[10px] border border-dashed border-line-strong bg-surface-2 font-display text-[13px] font-semibold text-muted"
          >
            {tile}
          </span>
          <span className="flex min-w-0 flex-col gap-px">
            <Link
              href={href}
              className="text-[15px] font-semibold text-ink link-hover"
            >
              {includer.name}
            </Link>
            <span className="text-[12.5px] text-faint">Bundle{purpose ? ` · ${purpose}` : ""}</span>
          </span>
        </span>
        <span className={`flex-[1_1_160px] text-[13.5px] ${behind ? "font-semibold text-warn-text" : "text-muted"}`}>
          {pinned === 0
            ? `v${includer.draft_version} in its next version`
            : `Includes v${pinned}${behind ? `, ${gap === 1 ? "one" : gap} behind` : ""}`}
        </span>
        <span className="w-[110px] flex-none text-right text-[13.5px] text-muted">
          {includer.people} {includer.people === 1 ? "person" : "people"}
        </span>
        <span className="flex w-[150px] flex-none justify-end">
          {behind &&
            (waiting ? (
              <span className="text-right text-[13px] text-faint">v{latest} in its next version</span>
            ) : (
              <Button
                size="sm"
                isPending={update.isPending}
                onClick={async () => {
                  setFailure(null);
                  try {
                    await update.mutateAsync({ child: childId, version: latest });
                  } catch (error) {
                    setFailure(outcomeFromError(error));
                  }
                }}
              >
                Update to v{latest}
              </Button>
            ))}
        </span>
      </div>
      {failure && <ActionOutcome outcome={failure} className="mt-2.5" />}
    </div>
  );
}
