"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { AddRolesToBundle } from "@/components/bundles/AddRolesToBundle";
import { BundleVersions } from "@/components/bundles/BundleVersions";
import { PublishVersionDialog } from "@/components/bundles/PublishVersionDialog";
import { RoleRef as RoleRefInline } from "@/components/names";
import { draftChangeCount, useBundleDraft } from "@/lib/queries/useBundleVersions";
import { RoleRef, UserName } from "@/components/names";
import {
  useBundleImpact,
  useBundleRoles,
  useBundles,
  useCreateBundle,
  useRemoveBundleRole,
  useSetWelcomeBundle,
  type BundleRoleRow,
} from "@/lib/queries/useBundles";
import { useMappingRules } from "@/lib/queries/useMappingRules";

/**
 * S1 · Bundles. Advanced, because this is the machine that acts on everyone:
 * publishing a version can change access for every holder at once.
 *
 * Editing does not. Since versioning, an edit changes the working copy and
 * reaches nobody — the consequence belongs to Publish, which is rehearsed. Copy
 * on this screen has to keep saying which of the two it is describing; the
 * first cut of versioning left several sentences behind on the old behaviour,
 * and a stale sentence here reads as a change that has already been applied.
 *
 * Assigning a bundle to one person is Basic and lives on their page. The rule
 * generalises: acting on one person is Basic; changing the thing that acts on
 * everyone is Advanced.
 *
 * Three columns — bundles, the roles inside the selected one, and impact. The
 * impact panel occupies the space where a confirmation dialog would have been,
 * so the consequence is on screen BEFORE you commit rather than after you click.
 */
export default function BundlesPage() {
  const bundles = useBundles();
  const [selected, setSelected] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const rows = bundles.data ?? [];
  const activeId = selected ?? rows[0]?.id ?? null;
  const active = rows.find((bundle) => bundle.id === activeId);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Bundles"
        meta="A named set of roles handed out as one unit."
        actions={
          <Button variant="accent" onClick={() => setCreating(true)}>
            New bundle
          </Button>
        }
      />

      <div className="flex flex-wrap items-start gap-5">
        <Card className="w-[240px] min-w-[210px] flex-none">
          <CardHeader title="All bundles" count={rows.length} />
          <ListStates
            isLoading={bundles.isLoading}
            error={bundles.error}
            isEmpty={rows.length === 0}
            onRetry={() => bundles.refetch()}
            errorTitle="Couldn't load bundles."
            skeleton={<RowSkeleton rows={4} avatar={false} label="Loading bundles" />}
            empty={
              <EmptyState
                title="No bundles yet."
                guidance="A bundle is how a set of roles gets handed out as one unit."
                action={{ label: "Create the first one", onClick: () => setCreating(true) }}
              />
            }
          >
            {rows.map((bundle) => (
              <button
                key={bundle.id}
                type="button"
                onClick={() => setSelected(bundle.id)}
                aria-current={activeId === bundle.id ? "true" : undefined}
                className={`row-divider flex w-full items-center gap-2.5 px-4 py-3 text-left transition-colors ${
                  activeId === bundle.id ? "bg-accent-soft/60" : "hover:bg-[var(--hover)]"
                }`}
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[14.5px] font-semibold">{bundle.name}</span>
                  {bundle.is_welcome && (
                    <span className="block truncate text-[12px] text-faint">
                      Default for new members
                    </span>
                  )}
                </span>
                <span className="flex shrink-0 items-center gap-1.5 text-[13.5px] text-faint">
                  {/* Two facts the list has to carry: an edit nobody published,
                      and holders an earlier publish left behind. Both are
                      invisible from inside the bundle you happen to have open. */}
                  {(bundle.unpublished_changes ?? 0) > 0 && (
                    <span
                      aria-label={`${bundle.unpublished_changes} unpublished changes`}
                      className="h-1.5 w-1.5 rounded-pill bg-accent"
                    />
                  )}
                  {(bundle.stale_holders ?? 0) > 0 && (
                    <span className="text-warn-text">{bundle.stale_holders}&uarr;</span>
                  )}
                  {bundle.holder_count ?? 0}
                </span>
              </button>
            ))}
          </ListStates>
        </Card>

        {active ? (
          <BundleWorkspace
            bundleId={active.id}
            name={active.name}
            isWelcome={active.is_welcome ?? false}
            holders={active.holder_count ?? 0}
            welcomeName={rows.find((bundle) => bundle.is_welcome)?.name}
          />
        ) : null}
      </div>

      <CreateBundleDialog open={creating} onClose={() => setCreating(false)} />
    </div>
  );
}

function BundleWorkspace({
  bundleId,
  name,
  isWelcome,
  holders,
  welcomeName,
}: {
  bundleId: string;
  name: string;
  isWelcome: boolean;
  holders: number;
  welcomeName?: string;
}) {
  const roles = useBundleRoles(bundleId);
  const impact = useBundleImpact(bundleId);
  const setWelcome = useSetWelcomeBundle();
  const [pendingRemoval, setPendingRemoval] = useState<BundleRoleRow | null>(null);
  const [adding, setAdding] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const draft = useBundleDraft(bundleId);
  const pending = draftChangeCount(draft.data);

  const roleRows = roles.data ?? [];

  return (
    <div className="flex min-w-[420px] flex-1 flex-wrap items-start gap-5">
      <Card className="min-w-[340px] flex-1">
        <CardHeader
          title={name}
          note={`${roleRows.length} ${roleRows.length === 1 ? "role" : "roles"} · ${holders} ${
            holders === 1 ? "holder" : "holders"
          }${draft.data ? ` · latest v${draft.data.latest_version}` : ""}`}
        />

        <div className="row-divider px-5 py-3 text-[13.5px] leading-[1.55] text-muted">
          Editing here changes what the NEXT version will grant. Nobody&rsquo;s access moves until
          you publish, and publishing asks whether the {holders}{" "}
          {holders === 1 ? "person" : "people"} already holding it come along.
        </div>

        {/*
          The unpublished state is a strip on the card, not a badge somewhere
          quieter. A bundle can sit edited-but-unpublished indefinitely and look
          finished, and "I changed that weeks ago" is the failure this prevents.
        */}
        {pending > 0 && draft.data && (
          <div className="accent-note row-divider flex flex-wrap items-start gap-3 px-5 py-3.5">
            <div className="min-w-[260px] flex-1">
              <div className="text-[14.5px] font-semibold">
                {pending} unpublished {pending === 1 ? "change" : "changes"}
              </div>
              <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[13.5px]">
                {draft.data.added.map((role) => (
                  <span
                    key={`+${role.zitadel_project_id}:${role.zitadel_role_key}`}
                    className="flex items-baseline gap-1.5"
                  >
                    <span className="font-semibold text-accent-text">+</span>
                    <RoleRefInline
                      projectId={role.zitadel_project_id}
                      roleKey={role.zitadel_role_key}
                    />
                  </span>
                ))}
                {draft.data.removed.map((role) => (
                  <span
                    key={`-${role.zitadel_project_id}:${role.zitadel_role_key}`}
                    className="flex items-baseline gap-1.5"
                  >
                    <span className="font-semibold text-danger-text">&minus;</span>
                    <RoleRefInline
                      projectId={role.zitadel_project_id}
                      roleKey={role.zitadel_role_key}
                    />
                  </span>
                ))}
              </div>
            </div>
            <Button variant="accent" onClick={() => setPublishing(true)}>
              Publish v{draft.data.next_version}
            </Button>
          </div>
        )}

        <ListStates
          isLoading={roles.isLoading}
          error={roles.error}
          isEmpty={roleRows.length === 0}
          onRetry={() => roles.refetch()}
          errorTitle="Couldn't load this bundle's roles."
          skeleton={<RowSkeleton rows={3} avatar={false} label="Loading roles" />}
          empty={
            <EmptyState
              title="This bundle carries no roles yet."
              guidance="Add some below. Until then, assigning it grants nothing."
              action={{ label: "Add roles", onClick: () => setAdding(true) }}
            />
          }
        >
          {roleRows.map((role) => {
            const selectedForRemoval =
              pendingRemoval?.zitadel_project_id === role.zitadel_project_id &&
              pendingRemoval?.zitadel_role_key === role.zitadel_role_key;
            return (
              <div
                key={`${role.zitadel_project_id}:${role.zitadel_role_key}`}
                className={`row-divider flex items-center gap-3 px-5 py-3 text-[14.5px] ${
                  selectedForRemoval ? "bg-warn-soft" : ""
                }`}
              >
                <span className="min-w-0 flex-1 truncate">
                  <RoleRef projectId={role.zitadel_project_id} roleKey={role.zitadel_role_key} />
                </span>
                <Button
                  size="sm"
                  onClick={() => setPendingRemoval(selectedForRemoval ? null : role)}
                >
                  {selectedForRemoval ? "Keep it" : "Drop"}
                </Button>
              </div>
            );
          })}
        </ListStates>

        <div className="row-divider flex items-center gap-3 px-5 py-3.5">
          <Button variant="accent" onClick={() => setAdding(true)}>
            Add roles
          </Button>
          <span className="text-[13.5px] text-faint">
            Search every project at once, tick what belongs here.
          </span>
        </div>

        <div className="row-divider flex flex-wrap items-center gap-3 px-5 py-4">
          <div className="min-w-[240px] flex-1">
            <div className="text-[14.5px] font-semibold">Default for new members</div>
            <p className="mt-0.5 text-[13px] text-muted">
              {isWelcome
                ? `${name} is assigned automatically when somebody is onboarded.`
                : `Currently ${welcomeName ?? "nothing"}. Exactly one bundle can hold this.`}
            </p>
          </div>
          <Button
            disabled={isWelcome}
            isPending={setWelcome.isPending}
            onClick={async () => {
              try {
                await setWelcome.mutateAsync(bundleId);
                toast.success(`${name} is now the default for new members.`);
              } catch (error) {
                toast.error(error instanceof Error ? error.message : "That didn't save.");
              }
            }}
          >
            {isWelcome ? "On" : `Set to ${name}`}
          </Button>
        </div>
      </Card>

      <div className="min-w-[320px] flex-1">
        {pendingRemoval ? (
          <RemovalImpact
            bundleId={bundleId}
            bundleName={name}
            role={pendingRemoval}
            onCancel={() => setPendingRemoval(null)}
          />
        ) : (
          <div className="flex flex-col gap-5">
            <HoldersPanel
              name={name}
              holders={impact.data?.users ?? []}
              isLoading={impact.isLoading}
            />
            <BundleVersions bundleId={bundleId} name={name} />
          </div>
        )}
      </div>

      {publishing && draft.data && (
        <PublishVersionDialog
          bundleId={bundleId}
          name={name}
          draft={draft.data}
          onClose={() => setPublishing(false)}
        />
      )}

      {adding && (
        <AddRolesToBundle
          bundleId={bundleId}
          name={name}
          holders={holders}
          onClose={() => setAdding(false)}
        />
      )}
    </div>
  );
}

/**
 * What dropping this role WOULD do, if the next version is published and the
 * current holders are moved onto it.
 *
 * Every sentence here is conditional, and that is the correction versioning
 * forced. The panel used to read "affects 14 people now" and "14 people lose
 * it" beside a red confirm button — which was true when a removal cascaded on
 * save, and became a lie the moment it stopped. An operator reading it would
 * have taken the edit for a revocation already applied, and either believed a
 * door was locked while it was open or gone looking for a change that had not
 * happened.
 *
 * The content is unchanged and still worth showing: who loses the role, who
 * keeps it through a rule, and what cascades away with it is exactly what you
 * want before deciding whether to make the edit at all. Only the tense moved,
 * and the tone with it — amber, because this is consequential and has not
 * happened, rather than red, which is reserved for the click that does it.
 */
function RemovalImpact({
  bundleId,
  bundleName,
  role,
  onCancel,
}: {
  bundleId: string;
  bundleName: string;
  role: BundleRoleRow;
  onCancel: () => void;
}) {
  const impact = useBundleImpact(bundleId);
  const rules = useMappingRules();
  const remove = useRemoveBundleRole(bundleId);

  const holders = impact.data?.users ?? [];

  // Who keeps the role anyway, because a rule produces it. Removing it from the
  // bundle changes nothing for them, and saying so is the difference between a
  // safe click and an outage.
  const coveringRule = (rules.data ?? []).find(
    (rule) =>
      rule.target_project === role.zitadel_project_id && rule.target_role === role.zitadel_role_key,
  );

  // What else goes with it: a rule that reads this role as its input stops
  // firing for anybody who loses it.
  const cascades = useMemo(
    () =>
      (rules.data ?? []).filter(
        (rule) =>
          rule.source_project === role.zitadel_project_id &&
          rule.source_role === role.zitadel_role_key,
      ),
    [rules.data, role],
  );

  return (
    <div className="rounded-card border border-warn-line bg-warn-soft p-5">
      <div className="type-label mb-1 text-warn-text">
        Dropping <RoleRef projectId={role.zitadel_project_id} roleKey={role.zitadel_role_key} />{" "}
        from the working copy
      </div>
      <p className="mb-2.5 max-w-[60ch] text-[13.5px] leading-[1.55] text-ink/[.78]">
        Nobody loses anything today. This is what would happen to the{" "}
        {holders.length} {holders.length === 1 ? "person" : "people"} holding{" "}
        {bundleName} <em>if</em> you publish the next version and move them onto it.
      </p>

      <ul className="mb-4 flex flex-col gap-2 text-[14px] leading-[1.5]">
        <li className="flex items-start gap-2.5">
          <span aria-hidden className="mt-[7px] h-2 w-2 flex-none rounded-pill bg-danger" />
          <span>
            <strong className="font-semibold">
              {holders.length} {holders.length === 1 ? "person would lose" : "people would lose"} it
              through this bundle
            </strong>
            {coveringRule ? (
              <span className="text-muted"> — unless a rule also gives it to them</span>
            ) : (
              <span className="text-muted"> — no rule reproduces it</span>
            )}
          </span>
        </li>

        {coveringRule && (
          <li className="flex items-start gap-2.5">
            <span
              aria-hidden
              className="mt-[7px] h-2 w-2 flex-none rounded-pill bg-ink/30"
            />
            <span className="text-muted">
              Anybody holding <RoleRef projectId={coveringRule.source_project} roleKey={coveringRule.source_role} /> would keep it — an automatic rule produces it
              independently.
            </span>
          </li>
        )}

        {cascades.map((rule) => (
          <li key={rule.id} className="flex items-start gap-2.5">
            <span
              aria-hidden
              className="mt-[6px] h-2.5 w-2.5 flex-none rounded-pill border border-dashed border-ink/40"
            />
            <span className="text-muted">
              They would also lose <RoleRef projectId={rule.target_project} roleKey={rule.target_role} /> by cascade — this role is that rule&rsquo;s input.
            </span>
          </li>
        ))}
      </ul>

      <div className="flex flex-wrap items-center gap-2.5">
        {/* Not `dangerConfirm`. That treatment is for a click that takes
            access away, and this one edits a draft — dressing it as the
            destructive act is the same misreading the copy above used to
            invite. The destructive confirm lives on Publish, where the
            revocation actually happens. */}
        <Button
          isPending={remove.isPending}
          onClick={async () => {
            try {
              await remove.mutateAsync({
                projectId: role.zitadel_project_id,
                roleKey: role.zitadel_role_key,
              });
              toast.success(`Dropped from ${bundleName}'s working copy.`, {
                description: "Nobody loses it until you publish a version and move them onto it.",
              });
              onCancel();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "The edit didn't save.");
            }
          }}
        >
          Drop it from the working copy
        </Button>
        <Button onClick={onCancel}>Cancel</Button>
      </div>
    </div>
  );
}

function HoldersPanel({
  name,
  holders,
  isLoading,
}: {
  name: string;
  holders: Array<{ id: string; name: string }>;
  isLoading: boolean;
}) {
  return (
    <Card>
      <CardHeader title="Who holds it" count={holders.length} />
      {isLoading ? (
        <RowSkeleton rows={3} avatar={false} label="Loading holders" />
      ) : holders.length === 0 ? (
        <div className="px-5 py-4 text-[14px] text-muted">
          Nobody holds {name} yet, so editing it changes nothing today.
        </div>
      ) : (
        holders.slice(0, 12).map((holder) => (
          <div key={holder.id} className="row-divider px-5 py-2.5 text-[14px]">
            <UserName id={holder.id} fallback={holder.name} />
          </div>
        ))
      )}
      {holders.length > 12 && (
        <div className="row-divider px-5 py-2.5 text-[13px] text-faint">
          and {holders.length - 12} more
        </div>
      )}
    </Card>
  );
}

function CreateBundleDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const create = useCreateBundle();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  if (!open) return null;

  return (
    <Modal open onClose={onClose} busy={create.isPending} size="sm" labelledBy="new-bundle-title">
      <ModalHeader
        title="New bundle"
        titleId="new-bundle-title"
        lede="Name it after the person it describes, not the roles inside it."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="bundle-name">Name</FieldLabel>
          <Input
            id="bundle-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="Who is this for?"
          />
        </div>
        <div>
          <FieldLabel htmlFor="bundle-description">What is it for?</FieldLabel>
          <Input
            id="bundle-description"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Student staff who run the fabrication lab"
          />
        </div>
      </div>
      <ModalFooter>
        <Button
          variant="accent"
          disabled={!name.trim()}
          isPending={create.isPending}
          onClick={async () => {
            try {
              await create.mutateAsync({ name: name.trim(), description });
              toast.success(`${name} created. It carries no roles yet.`);
              setName("");
              setDescription("");
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "That didn't save.");
            }
          }}
        >
          Create bundle
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}
