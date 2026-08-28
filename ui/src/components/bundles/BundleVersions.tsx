"use client";

import { useMemo, useState } from "react";

import { RoleRef, UserAvatar, UserName } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import { Relative } from "@/components/ui/Time";
import {
  useApplyMoveHolders,
  useBundleHolders,
  useBundleVersions,
  useRehearseMoveHolders,
  type BundleVersion,
} from "@/lib/queries/useBundleVersions";

/**
 * A bundle's history, and who is standing on each part of it.
 *
 * The holder count is what makes this a working surface rather than a
 * changelog. "v4 · 3 people, v2 · 11 people" is a fact about the makerspace an
 * operator has to act on; a list of version numbers with dates is not.
 */
export function BundleVersions({ bundleId, name }: { bundleId: string; name: string }) {
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
          behind.length > 0
            ? `${behind.length} ${behind.length === 1 ? "person is" : "people are"} on an older version`
            : "Everybody is on the latest version"
        }
        action={
          behind.length > 0 && latest ? (
            <Button size="sm" onClick={() => setMoving(latest)}>
              Move everyone to v{latest.version}
            </Button>
          ) : undefined
        }
      />
      <p className="row-divider px-5 pb-3.5 text-[13.5px] leading-[1.5] text-muted">
        Each person holds a specific version of {name}. Publishing a new version does not move them
        unless you choose to.
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
                <Badge tone="accent">Current</Badge>
              ) : version.holder_count > 0 ? (
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

            {version.holder_count > 0 && index > 0 && (
              <div className="mt-2.5">
                <Button size="sm" onClick={() => setMoving(rows[0])}>
                  Move these {version.holder_count} {version.holder_count === 1 ? "person" : "people"} to v{rows[0].version}
                </Button>
              </div>
            )}
          </div>
        ))}
      </ListStates>

      <Holders bundleId={bundleId} onMoveTo={(v) => setMoving(v)} versions={rows} />

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

  return (
    <div className="border-t border-line">
      <div className="px-5 pb-1 pt-3.5 type-label">Who is on what</div>
      {Array.from(byVersion.entries())
        .sort((a, b) => b[0] - a[0])
        .map(([version, people]) => (
          <div key={version} className="row-divider flex flex-wrap items-center gap-3 px-5 py-3">
            <span className="w-[52px] shrink-0 font-display text-[15px]">v{version}</span>
            <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-3 gap-y-1.5">
              {people.map((holder) => (
                <span key={holder.user_id} className="flex items-center gap-1.5 text-[14px]">
                  <UserAvatar id={holder.user_id} size="row" />
                  <UserName id={holder.user_id} />
                </span>
              ))}
            </div>
            {latest && latest.version !== version && (
              <Button size="sm" onClick={() => onMoveTo(latest)}>
                Move to v{latest.version}
              </Button>
            )}
          </div>
        ))}
    </div>
  );
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
      onRehearse={() => rehearse.mutateAsync({ version_id: target.id, user_ids: userIds })}
      onApply={() => apply.mutateAsync({ version_id: target.id, user_ids: userIds })}
      onClose={onClose}
    />
  );
}
