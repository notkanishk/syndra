"use client";

import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
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
  useDeleteBundle,
  useRemoveBundleRole,
  useSetWelcomeBundle,
  useUpdateBundle,
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
        lede="A bundle is a set of roles given together. Put a person in a bundle and they hold every role in it; publish a new version and you choose whether they move."
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
                className={`row-divider flex w-full items-center gap-2.5 px-4 py-3 text-left motion-tint ${
                  activeId === bundle.id ? "bg-accent-soft/60" : "hover:bg-[var(--hover)]"
                }`}
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[14.5px] font-semibold">{bundle.name}</span>
                  {bundle.is_welcome && (
                    <span className="block truncate text-[12.5px] text-faint">
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
            key={active.id}
            bundleId={active.id}
            name={active.name}
            description={active.description ?? ""}
            isWelcome={active.is_welcome ?? false}
            holders={active.holder_count ?? 0}
            welcomeName={rows.find((bundle) => bundle.is_welcome)?.name}
            // The deleted bundle is still `selected` until this clears it, and the list would
            // otherwise fall back to showing the first bundle under the old id's heading.
            onDeleted={() => setSelected(null)}
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
  description,
  isWelcome,
  holders,
  welcomeName,
  onDeleted,
}: {
  bundleId: string;
  name: string;
  description: string;
  isWelcome: boolean;
  holders: number;
  welcomeName?: string;
  onDeleted: () => void;
}) {
  const roles = useBundleRoles(bundleId);
  const impact = useBundleImpact(bundleId);
  const setWelcome = useSetWelcomeBundle();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [pendingRemoval, setPendingRemoval] = useState<BundleRoleRow | null>(null);
  const [adding, setAdding] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [deleting, setDeleting] = useState(false);
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
          // Rename sits on the title because that is what it changes. It is not an edit in the
          // versioning sense — it reaches nobody and publishes nothing — so it deliberately does
          // not join the working-copy controls further down.
          action={
            <Button size="sm" variant="ghost" onClick={() => setRenaming(true)}>
              Rename
            </Button>
          }
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
                setOutcome({
                  kind: "applied",
                  message: `${name} is now the default for new members`,
                  detail: "Members who joined before this keep what they already hold.",
                });
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {isWelcome ? "On" : `Set to ${name}`}
          </Button>
        </div>

        {outcome && <ActionOutcome outcome={outcome} className="mx-5 mb-4" />}

        {/*
          Retiring the bundle is the last row on the card, under everything it does, because it
          is the one action here that ends rather than changes. Outline red — the solid confirm
          lives inside the dialog.
        */}
        <div className="flex flex-wrap items-center gap-3 px-5 py-4">
          <div className="min-w-[240px] flex-1">
            <div className="text-[14.5px] font-semibold">Retire this bundle</div>
            <p className="mt-0.5 text-[13px] text-muted">
              {holders === 0
                ? "Nobody holds it, so nothing is revoked."
                : `The ${holders} ${holders === 1 ? "person" : "people"} holding it lose whatever only this bundle gave them.`}
            </p>
          </div>
          <Button variant="danger" onClick={() => setDeleting(true)}>
            Delete bundle
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

      {renaming && (
        <RenameBundleDialog
          bundleId={bundleId}
          name={name}
          description={description}
          onClose={() => setRenaming(false)}
        />
      )}

      {deleting && (
        <DeleteBundleDialog
          bundleId={bundleId}
          name={name}
          holders={holders}
          isWelcome={isWelcome}
          roleCount={roleRows.length}
          onCancel={() => setDeleting(false)}
          onDeleted={() => {
            setDeleting(false);
            onDeleted();
          }}
        />
      )}
    </div>
  );
}

/**
 * Rename and re-describe. No confirmation-mode field and no version note, because neither is
 * what this changes: the name is what operators call the bundle, not what it grants.
 */
function RenameBundleDialog({
  bundleId,
  name,
  description,
  onClose,
}: {
  bundleId: string;
  name: string;
  description: string;
  onClose: () => void;
}) {
  const update = useUpdateBundle();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [nextName, setNextName] = useState(name);
  const [nextDescription, setNextDescription] = useState(description);

  const trimmed = nextName.trim();
  const unchanged = trimmed === name && nextDescription === description;

  return (
    <Modal open onClose={onClose} busy={update.isPending} size="sm" labelledBy="rename-bundle">
      <ModalHeader
        title={`Rename ${name}`}
        titleId="rename-bundle"
        lede="Nobody's access changes. This is what the bundle is called, not what it grants."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="bundle-rename">Name</FieldLabel>
          <Input
            id="bundle-rename"
            value={nextName}
            onChange={(event) => setNextName(event.target.value)}
          />
        </div>
        <div>
          <FieldLabel htmlFor="bundle-redescribe">What it&rsquo;s for</FieldLabel>
          <Input
            id="bundle-redescribe"
            value={nextDescription}
            onChange={(event) => setNextDescription(event.target.value)}
          />
        </div>
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="accent"
          disabled={!trimmed || unchanged}
          isPending={update.isPending}
          reason={!trimmed ? "A bundle needs a name." : unchanged ? "Nothing changed yet." : undefined}
          onClick={async () => {
            try {
              await update.mutateAsync({
                id: bundleId,
                name: trimmed,
                description: nextDescription,
              });
              setOutcome({
                kind: "applied",
                message: trimmed === name ? "Description saved" : `Now called ${trimmed}`,
              });
              onClose();
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Save
        </Button>
        <Button disabled={update.isPending} onClick={onClose}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/**
 * The consequence, before the click.
 *
 * Two facts drive it. Holders lose whatever only this bundle gave them — "only" because anybody
 * who also has the role by rule or direct grant keeps it, worked out per person by the backend.
 * And if this is the welcome bundle, onboarding stops handing anything out, which has no other
 * place to be said once the row is gone.
 *
 * The version history goes with it. That is worth stating plainly rather than discovering: it is
 * the one part of a deletion nothing can restore.
 */
function DeleteBundleDialog({
  bundleId,
  name,
  holders,
  isWelcome,
  roleCount,
  onCancel,
  onDeleted,
}: {
  bundleId: string;
  name: string;
  holders: number;
  isWelcome: boolean;
  roleCount: number;
  onCancel: () => void;
  onDeleted: () => void;
}) {
  const remove = useDeleteBundle();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  // The bundle is gone in both of these: `queued` means its withdrawals are
  // waiting, not that the delete is.
  const gone = outcome?.kind === "applied" || outcome?.kind === "queued";

  return (
    <Modal
      open
      onClose={gone ? onDeleted : onCancel}
      busy={remove.isPending}
      size="md"
      labelledBy="delete-bundle"
    >
      <ModalHeader
        title={`Delete ${name}?`}
        titleId="delete-bundle"
        lede={
          holders === 0
            ? "Nobody holds it. Deleting it takes nothing away from anybody."
            : `${holders} ${holders === 1 ? "person holds" : "people hold"} it right now.`
        }
      />

      <div className="px-6">
        <div className="danger-note px-4 py-3.5 text-[14px] leading-[1.55]">
          <div className="type-label mb-1 text-danger-text">What happens</div>
          <ul className="flex flex-col gap-1 text-muted">
            {holders > 0 && (
              <li>
                Each holder loses whichever of its {roleCount}{" "}
                {roleCount === 1 ? "role" : "roles"} nothing else gives them — a rule or direct
                access to the same role keeps it.
              </li>
            )}
            {isWelcome && (
              <li className="font-semibold text-danger-text">
                This is the default for new members. Onboarding will stop handing anything out
                until another bundle is set.
              </li>
            )}
            <li>Its version history goes with it, and cannot be brought back.</li>
          </ul>
        </div>
      </div>

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter note="Emptying the bundle instead leaves it assignable and grants nothing.">
        {!gone && (
        <Button
          variant="dangerConfirm"
          isPending={remove.isPending}
          onClick={async () => {
            try {
              const result = await remove.mutateAsync(bundleId);
              const n = result.cascade?.enqueued ?? 0;
              const auto = result.cascade?.mode === "auto";
              // The welcome-bundle consequence is folded into the same report
              // rather than fired as a second notification. It is the same
              // event, and it is the half an operator most needs: from now on
              // a new member receives nothing.
              const orphaned = result.was_welcome
                ? " New members no longer receive a bundle — set another as the default."
                : "";
              setOutcome(
                n === 0
                  ? {
                      kind: "applied",
                      message: `${name} deleted`,
                      detail: `Nobody's access changed — it carried nothing anybody held.${orphaned}`,
                    }
                  : {
                      kind: auto ? "applied" : "queued",
                      message: `${name} deleted — ${n} ${n === 1 ? "change" : "changes"} ${
                        auto ? "applied" : "waiting"
                      }`,
                      detail: auto
                        ? `The revocations it set off went with it.${orphaned}`
                        : `The revocations it set off wait under Pending changes until you send them.${orphaned}`,
                    },
              );
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Delete and revoke
        </Button>
        )}
        {/* `onDeleted` clears the parent's selection, and the parent's own
            comment says why it must: the deleted bundle stays selected
            otherwise, and the list falls back to the first bundle under the old
            id's heading. It was never called. Called from HERE rather than from
            the mutation, because clearing the selection unmounts this dialog —
            and the outcome the operator has not read yet goes with it. */}
        <Button disabled={remove.isPending} onClick={gone ? onDeleted : onCancel}>
          {gone ? "Done" : "Keep it"}
        </Button>
      </ModalFooter>
    </Modal>
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
  const [outcome, setOutcome] = useState<Outcome | null>(null);

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

      {outcome && <ActionOutcome outcome={outcome} className="mb-4" />}

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
              setOutcome({
                // Nobody's access moved: the working copy did. Same distinction
                // the add-roles panel makes, in the other direction.
                kind: "no_change",
                message: `Dropped from ${bundleName}'s working copy`,
                detail: "Nobody loses it until you publish a version and move them onto it.",
              });
            } catch (error) {
              setOutcome(outcomeFromError(error));
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
  const [outcome, setOutcome] = useState<Outcome | null>(null);
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
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="accent"
          disabled={!name.trim()}
          isPending={create.isPending}
          onClick={async () => {
            try {
              await create.mutateAsync({ name: name.trim(), description });
              setOutcome({
                kind: "applied",
                message: `${name} created`,
                detail: "It carries no roles yet, so holding it grants nothing.",
              });
              setName("");
              setDescription("");
              onClose();
            } catch (error) {
              setOutcome(outcomeFromError(error));
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
