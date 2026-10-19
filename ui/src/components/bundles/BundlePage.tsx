"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";

import { AddRolesToBundle } from "@/components/bundles/AddRolesToBundle";
import { BundlePeople, BundleVersions, ThroughOtherBundles } from "@/components/bundles/BundleVersions";
import { IncludeChangeLine, PublishVersionDialog } from "@/components/bundles/PublishVersionDialog";
import { RoleRef } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Chip, Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { KEPT, PushedStep, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import { Tabs } from "@/components/ui/Tabs";
import { Term } from "@/components/ui/Term";
import { Unfold } from "@/components/ui/Unfold";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { formatList, formatShortDate, humanizeKey, roleLabel } from "@/lib/format";
import {
  outcomeFromError,
  statesNothingChanged,
  type ActionOutcome as Outcome,
} from "@/lib/outcome";
import { useCrumb } from "@/lib/page-crumb";
import {
  draftChangeCount,
  useBundleDraft,
  useBundleHolders,
  useBundleIncluders,
  useBundleVersions,
  useDiscardBundleDraft,
  useVersionsByBundle,
  type BundleHolder,
  type BundleInclude,
  type BundleIncluder,
  type BundleVersion,
} from "@/lib/queries/useBundleVersions";
import {
  useAddBundleRole,
  useBundleImpact,
  useBundleRoles,
  useBundles,
  useDeleteBundle,
  useRemoveBundleInclude,
  useRemoveBundleRole,
  useSetBundleInclude,
  useSetWelcomeBundle,
  useUpdateBundle,
  type BundleRoleRow,
  type BundleRow,
} from "@/lib/queries/useBundles";
import { useMappingRules } from "@/lib/queries/useMappingRules";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";

type Tab = "grants" | "people" | "history";

const idOf = (r: { zitadel_project_id: string; zitadel_role_key: string }) =>
  `${r.zitadel_project_id}:${r.zitadel_role_key}`;

/**
 * A bundle's own page (1c): laid out like a person's page. A header, the
 * unpublished changes in one violet panel above the tabs, then what it gives
 * as a card per project; who has it and its history are the other two tabs.
 * The default-for-new-members and retire controls sit at the end, under
 * everything the bundle does.
 *
 * Editing still reaches nobody: it changes the working copy, and Publish is
 * where people move, rehearsed. Every sentence here keeps saying which of the
 * two it is describing.
 */
export function BundlePage({ bundleId }: { bundleId: string }) {
  const router = useRouter();
  const bundles = useBundles();
  const live = (bundles.data ?? []).find((b) => b.id === bundleId) ?? null;
  // Remembered so the page outlives its own delete: the list refetch drops
  // the row while the delete dialog is still reporting what it did.
  const [remembered, setRemembered] = useState<BundleRow | null>(null);
  if (live && live.id !== remembered?.id) setRemembered(live);
  const bundle = live ?? remembered;
  useCrumb(bundle?.name);

  const [tab, setTab] = useState<Tab>("grants");
  const [publishing, setPublishing] = useState(false);
  const [adding, setAdding] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const roles = useBundleRoles(bundleId);
  const draft = useBundleDraft(bundleId);
  const versions = useBundleVersions(bundleId);
  const pending = draftChangeCount(draft.data);
  // Included bundles (B8): what this one takes in, and who takes this one in.
  const includes = useMemo(() => draft.data?.includes ?? [], [draft.data?.includes]);
  const childVersions = useVersionsByBundle(includes.map((i) => i.child_bundle_id));
  const includers = useBundleIncluders(bundleId);
  const holderRows = useBundleHolders(bundleId);

  // A stat jumps to its group on the People tab, opening the tab on the way.
  function jump(event: React.MouseEvent) {
    const link = (event.target as HTMLElement).closest("a");
    if (!link?.hash) return;
    event.preventDefault();
    setTab("people");
    requestAnimationFrame(() =>
      document.getElementById(link.hash.slice(1))?.scrollIntoView({ behavior: "smooth", block: "start" }),
    );
  }

  if (!bundle) {
    return (
      <div className="flex flex-col gap-7">
        <PageHeader eyebrow="Bundles" title="Bundle" lede="What it gives, who has it, and its history." />
        <Card>
          <ListStates
            isLoading={bundles.isLoading}
            error={bundles.error}
            isEmpty
            onRetry={() => bundles.refetch()}
            errorTitle="Couldn't load this bundle."
            skeleton={<RowSkeleton rows={3} avatar={false} label="Loading the bundle" />}
            empty={
              <EmptyState
                title="There's no bundle here."
                guidance="It may have been retired. Every bundle that exists is on the list."
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

  const name = bundle.name;
  const holders = bundle.holder_count ?? 0;
  const latest = (versions.data ?? []).reduce<{ version: number; published_at: string } | null>(
    (best, v) => (!best || v.version > best.version ? v : best),
    null,
  );
  // What it gives: its own roles and each included bundle's, a role held both ways counted once.
  const roleCount = new Set([
    ...(roles.data ?? []).map(idOf),
    ...includes.flatMap((i) => pinnedRoles(i, childVersions[i.child_bundle_id]).map(idOf)),
  ]).size;
  const includedBy = includers.data ?? [];
  const people = `${holders} ${holders === 1 ? "person" : "people"}`;
  const welcome = (bundles.data ?? []).find((b) => b.is_welcome);

  return (
    <div className="flex flex-col gap-7">
      <PageHeader
        eyebrow="Bundles"
        title={name}
        meta={[
          latest ? `v${latest.version} · published ${formatShortDate(latest.published_at)}` : null,
          `${roleCount} ${roleCount === 1 ? "role" : "roles"}`,
          people,
        ]
          .filter(Boolean)
          .join(" · ")}
        lede={bundle.description || undefined}
        actions={
          <>
            <Button onClick={() => setRenaming(true)}>Rename</Button>
            <ButtonLink
              variant="accent"
              href={`/users?bulk=1&give_bundle=${encodeURIComponent(bundleId)}`}
            >
              Add people
            </ButtonLink>
          </>
        }
      />

      {includedBy.length > 0 && latest && (
        <div onClickCapture={jump}>
          <IncludedStats latest={latest.version} holders={holderRows.data ?? []} includers={includedBy} />
        </div>
      )}

      <Unfold open={pending > 0 && Boolean(draft.data)}>
        {draft.data && (
          <UnpublishedPanel
            bundleId={bundleId}
            name={name}
            holders={holders}
            draft={draft.data}
            onPublish={() => setPublishing(true)}
          />
        )}
      </Unfold>

      <div className="flex flex-col gap-[18px]">
        <Tabs<Tab>
          label={`${name}: what it gives, who has it, and its history`}
          value={tab}
          onSelect={setTab}
          options={[
            { value: "grants", label: <TabLabel text="What it gives" n={roleCount} /> },
            { value: "people", label: <TabLabel text="People" n={holders} /> },
            { value: "history", label: <TabLabel text="History" n={(versions.data ?? []).length} /> },
          ]}
        />

        {tab === "grants" && (
          <WhatItGives
            bundleId={bundleId}
            name={name}
            roles={roles}
            draft={draft.data}
            childVersions={childVersions}
            onAdd={() => setAdding(true)}
          />
        )}
        {tab === "people" && (
          <>
            <BundlePeople bundleId={bundleId} name={name} />
            {includedBy.length > 0 && latest && (
              <ThroughOtherBundles
                name={name}
                bundleId={bundleId}
                latest={latest.version}
                includers={includedBy}
                bundles={bundles.data ?? []}
              />
            )}
          </>
        )}
        {tab === "history" && <BundleVersions bundleId={bundleId} name={name} />}
      </div>

      <EndCards
        bundleId={bundleId}
        name={name}
        holders={holders}
        isWelcome={bundle.is_welcome ?? false}
        welcomeName={welcome?.name}
        includedIn={bundle.included_in ?? []}
        onRetire={() => setDeleting(true)}
      />

      {publishing && draft.data && (
        <PublishVersionDialog
          bundleId={bundleId}
          name={name}
          draft={draft.data}
          // Every bundle whose working copy includes this one must be answered.
          includers={includedBy.filter((i) => i.included_version > 0 || (i.draft_version ?? 0) > 0)}
          onClose={() => setPublishing(false)}
        />
      )}
      {adding && (
        <AddRolesToBundle bundleId={bundleId} name={name} holders={holders} onClose={() => setAdding(false)} />
      )}
      {renaming && (
        <RenameBundleDialog
          bundleId={bundleId}
          name={name}
          description={bundle.description ?? ""}
          onClose={() => setRenaming(false)}
        />
      )}
      {deleting && (
        <DeleteBundleDialog
          bundleId={bundleId}
          name={name}
          holders={holders}
          isWelcome={bundle.is_welcome ?? false}
          roleCount={roleCount}
          onCancel={() => setDeleting(false)}
          onDeleted={() => router.push("/bundles")}
        />
      )}
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
 * The working copy's changes, in one violet panel above the tabs: each role
 * added or dropped, what dropping it would cost, and the two ways on.
 */
function UnpublishedPanel({
  bundleId,
  name,
  holders,
  draft,
  onPublish,
}: {
  bundleId: string;
  name: string;
  holders: number;
  draft: NonNullable<ReturnType<typeof useBundleDraft>["data"]>;
  onPublish: () => void;
}) {
  const discard = useDiscardBundleDraft(bundleId);
  const [failure, setFailure] = useState<Outcome | null>(null);
  const n = draftChangeCount(draft);
  return (
    <section className="flex flex-col gap-3 rounded-panel border border-accent-line bg-accent-soft px-5 py-4">
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="type-card-title min-w-0 flex-1">
          {n} {n === 1 ? "change" : "changes"}, not published yet
        </h2>
        <Button
          disabled={discard.isPending}
          isPending={discard.isPending}
          onClick={async () => {
            setFailure(null);
            try {
              await discard.mutateAsync();
            } catch (error) {
              setFailure(outcomeFromError(error));
            }
          }}
        >
          Discard changes
        </Button>
        <Button variant="accent" onClick={onPublish}>
          Publish v{draft.next_version}
        </Button>
      </div>
      <ul className="flex flex-col gap-1.5 text-[14px]">
        {(draft.include_changes ?? []).map((change) => (
          <li key={`i${change.child_bundle_id}`} className="flex items-baseline gap-2">
            <IncludeChangeLine change={change} />
          </li>
        ))}
        {draft.added.map((role) => (
          <li key={`+${idOf(role)}`} className="flex items-baseline gap-2">
            <span aria-label="Adds" className="w-3 font-semibold text-accent-text">
              +
            </span>
            <RoleRef projectId={role.zitadel_project_id} roleKey={role.zitadel_role_key} />
          </li>
        ))}
        {draft.removed.map((role) => (
          <li key={`-${idOf(role)}`} className="flex items-baseline gap-2">
            <span aria-label="Drops" className="w-3 font-semibold text-warn-text">
              &minus;
            </span>
            <RoleRef projectId={role.zitadel_project_id} roleKey={role.zitadel_role_key} />
          </li>
        ))}
      </ul>
      <p className="text-[13px] text-muted">
        {holders === 0
          ? `Nobody's access has changed. Nobody has ${name} yet, so publishing moves nobody.`
          : `Nobody's access has changed. Publishing asks whether the ${holders} ${
              holders === 1 ? "person" : "people"
            } with ${name} move to v${draft.next_version}.`}
      </p>
      {failure && <ActionOutcome outcome={failure} />}
    </section>
  );
}

/** What it gives: a card per project, each role with its verb. */
function WhatItGives({
  bundleId,
  name,
  roles,
  draft,
  childVersions,
  onAdd,
}: {
  bundleId: string;
  name: string;
  roles: ReturnType<typeof useBundleRoles>;
  draft: ReturnType<typeof useBundleDraft>["data"];
  childVersions: Record<string, BundleVersion[] | undefined>;
  onAdd: () => void;
}) {
  const catalog = useGlobalRoleCatalog();
  const [removing, setRemoving] = useState<BundleRoleRow | null>(null);
  const keep = useAddBundleRole(bundleId);
  const byId = useMemo(() => {
    const m = new Map<string, CatalogRole>();
    for (const r of catalog.data ?? []) m.set(`${r.project_id}:${r.role_key}`, r);
    return m;
  }, [catalog.data]);

  const added = new Set((draft?.added ?? []).map(idOf));
  const leaving = useMemo(() => draft?.removed ?? [], [draft?.removed]);
  const rows = useMemo(() => [...(roles.data ?? []), ...leaving], [roles.data, leaving]);
  const groups = useMemo(() => {
    const out = new Map<string, BundleRoleRow[]>();
    for (const r of rows) {
      const project = byId.get(idOf(r))?.project_name || r.zitadel_project_id;
      out.set(project, [...(out.get(project) ?? []), r]);
    }
    return Array.from(out.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [rows, byId]);
  const next = draft?.next_version;
  const leavingIds = new Set(leaving.map(idOf));
  const includes = draft?.includes ?? [];

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <p className="min-w-0 flex-1 text-[13.5px] text-muted">
          {next
            ? `Changes here go into v${next}. Nobody's access changes until you publish.`
            : "Changes here go into the next version. Nobody's access changes until you publish."}
        </p>
        <Button size="sm" onClick={onAdd}>
          Add roles
        </Button>
      </div>

      <ListStates
        isLoading={roles.isLoading}
        error={roles.error}
        isEmpty={rows.length === 0 && includes.length === 0}
        onRetry={() => roles.refetch()}
        errorTitle={`Couldn't load what ${name} gives.`}
        skeleton={
          <Card>
            <RowSkeleton rows={3} avatar={false} label="Loading roles" />
          </Card>
        }
        empty={
          <Card>
            <EmptyState
              title={`${name} gives no roles yet.`}
              guidance="Until it does, giving it to someone gives them nothing."
              action={{ label: "Add roles", onClick: onAdd }}
            />
          </Card>
        }
      >
        {includes.map((include) => (
          <IncludedBundleCard
            key={include.child_bundle_id}
            parentId={bundleId}
            parentName={name}
            include={include}
            versions={childVersions[include.child_bundle_id]}
            catalog={byId}
          />
        ))}
        {groups.map(([project, list]) => (
          <Card key={project}>
            <CardHeader title={project} note={`${list.length} ${list.length === 1 ? "role" : "roles"}`} />
            {list.map((role) => {
              const id = idOf(role);
              const entry = byId.get(id);
              const isLeaving = leavingIds.has(id);
              const isNew = added.has(id);
              const open = removing !== null && idOf(removing) === id;
              return (
                <div key={id} className={`row-divider motion-tint ${isLeaving ? "bg-warn-soft" : ""}`}>
                  <div className="flex flex-wrap items-center gap-x-[18px] gap-y-2 px-5 py-3.5">
                    <div className="min-w-[min(240px,100%)] flex-1">
                      <span className="text-[15px] font-semibold">
                        {entry?.display_name || humanizeKey(role.zitadel_role_key)}
                      </span>{" "}
                      <Mono className="font-normal text-faint">{role.zitadel_role_key}</Mono>
                      {entry?.waits_for_send && <WaitsMark className="ml-2" />}
                      {entry?.description && (
                        <p className="mt-0.5 max-w-[80ch] text-[13.5px] text-muted [text-wrap:pretty]">
                          {entry.description}
                        </p>
                      )}
                    </div>
                    {(isNew || isLeaving) && next && (
                      <span
                        className={`text-[13px] font-semibold ${isLeaving ? "text-warn-text" : "text-accent-text"}`}
                      >
                        {isLeaving ? `Leaves in v${next}` : `New in v${next}`}
                      </span>
                    )}
                    <span className="flex justify-end whitespace-nowrap">
                      {isLeaving ? (
                        <Button
                          size="sm"
                          isPending={keep.isPending}
                          onClick={() =>
                            keep.mutate({ project_id: role.zitadel_project_id, role_key: role.zitadel_role_key })
                          }
                        >
                          Keep
                        </Button>
                      ) : (
                        <Button size="sm" onClick={() => setRemoving(open ? null : role)}>
                          {open ? "Keep it" : "Remove"}
                        </Button>
                      )}
                    </span>
                  </div>
                  <Unfold open={open}>
                    <div className="px-5 pb-4">
                      {open && removing && (
                        <RemovalImpact
                          bundleId={bundleId}
                          bundleName={name}
                          role={removing}
                          onCancel={() => setRemoving(null)}
                        />
                      )}
                    </div>
                  </Unfold>
                </div>
              );
            })}
          </Card>
        ))}
      </ListStates>
    </div>
  );
}

/** The default for new members, and retiring: last, under everything it does. */
function EndCards({
  bundleId,
  name,
  holders,
  isWelcome,
  welcomeName,
  includedIn,
  onRetire,
}: {
  bundleId: string;
  name: string;
  holders: number;
  isWelcome: boolean;
  welcomeName?: string;
  /** Bundles that include it: retiring is refused while there are any (B8). */
  includedIn: string[];
  onRetire: () => void;
}) {
  const refused = includedIn.length > 0 ? `Refused while ${formatList(includedIn)} include it.` : undefined;
  const setWelcome = useSetWelcomeBundle();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  return (
    <Card>
      <div className="row-divider flex flex-wrap items-center gap-3 px-5 py-4">
        <div className="min-w-[240px] flex-1">
          <div className="text-[14.5px] font-semibold">Given to everyone who joins</div>
          <p className="mt-0.5 text-[13px] text-muted">
            {isWelcome
              ? `${name} is given to new members now. One bundle at a time.`
              : welcomeName
                ? `${welcomeName} is given to new members now. One bundle at a time.`
                : "No bundle is given to new members now. One bundle at a time."}
          </p>
        </div>
        {!isWelcome && (
          <Button
            isPending={setWelcome.isPending}
            onClick={async () => {
              try {
                await setWelcome.mutateAsync(bundleId);
                setOutcome({
                  kind: "applied",
                  message: `${name} is now given to everyone who joins`,
                  detail: "Members who joined before this keep what they already hold.",
                });
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {welcomeName ? `Give ${name} instead` : `Give ${name} to everyone who joins`}
          </Button>
        )}
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-5 my-3" />}
      <div className="flex flex-wrap items-center gap-3 px-5 py-4">
        <div className="min-w-[240px] flex-1">
          <div className="text-[14.5px] font-semibold">Retire {name}</div>
          <p className="mt-0.5 text-[13px] text-muted">
            {refused ??
              (holders === 0
                ? "Nobody has it, so retiring it takes nothing from anyone."
                : `The ${holders} ${holders === 1 ? "person" : "people"} with it lose whatever only ${name} gave them. You see who and what before anything is sent.`)}
          </p>
        </div>
        {/* The line beside it is the reason; Button's own would say it twice. */}
        <Button variant="danger" disabled={Boolean(refused)} onClick={onRetire}>
          Retire bundle
        </Button>
      </div>
    </Card>
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
        {/* Closing on success used to fire in the same breath as `setOutcome`,
            which unmounts this dialog before the operator can read what it
            just set — "Now called X" was computed and never shown. Held open
            until Close is pressed instead, same as the delete dialog. */}
        {!succeeded(outcome) && (
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
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            Save
          </Button>
        )}
        <Button disabled={update.isPending} onClick={onClose}>
          {succeeded(outcome) ? "Close" : "Cancel"}
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
  // Retiring revokes what the bundle carried, which can cancel a change still
  // waiting for you to send: the sheet turns to the step first (A-04 §14).
  const step = useCancelStep({ host: `Retire ${name}`, goAhead: "retire it", tone: "danger" });
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
      <PushedStep step={step}>
      <ModalHeader
        title={`Retire ${name}?`}
        titleId="delete-bundle"
        lede={
          holders === 0
            ? "Nobody holds it. Retiring it takes nothing away from anybody."
            : `${holders} ${holders === 1 ? "person holds" : "people hold"} it right now.`
        }
      />

      <div className="px-6">
        <div className="danger-note px-4 py-3.5 text-[14px] leading-[1.55]">
          <div className="type-label mb-1 text-danger-text">What happens</div>
          <ul className="flex flex-col gap-1 text-muted">
            {holders > 0 && (
              <li>
                Everyone holding it loses whichever of its {roleCount}{" "}
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
      {gone && step.after}

      <ModalFooter
        // Static across `gone`, it used to recommend an alternative to a
        // click that had already happened — "empty it instead" above a
        // bundle already deleted, with nothing left to empty.
        note={step.note ?? (gone ? undefined : "Emptying the bundle instead leaves it assignable and grants nothing.")}
      >
        {/* `Keep it` precedes the destructive button in DOM order so the
            focus trap's initial focus (first focusable element) never lands
            on `Retire and revoke`; `order-1`/`order-2` keep it visually
            second, matching every other confirm dialog.
            `onDeleted` clears the parent's selection, and the parent's own
            comment says why it must: the deleted bundle stays selected
            otherwise, and the list falls back to the first bundle under the
            old id's heading. Called from HERE rather than from the mutation,
            because clearing the selection unmounts this dialog — and the
            outcome the operator has not read yet goes with it. */}
        <Button
          className={gone ? undefined : "order-2"}
          disabled={remove.isPending}
          onClick={gone ? onDeleted : onCancel}
        >
          {gone ? "Close" : "Keep it"}
        </Button>
        {!gone && (
        <Button
          variant="dangerConfirm"
          className="order-1"
          isPending={remove.isPending}
          onClick={async () => {
            try {
              const result = await step.run(() => remove.mutateAsync(bundleId));
              if (result === KEPT) return;
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
                      message: `${name} retired`,
                      detail: `Nobody's access changed — it carried nothing anybody held.${orphaned}`,
                    }
                  : {
                      kind: auto ? "applied" : "queued",
                      message: `${name} retired — ${n} ${n === 1 ? "change" : "changes"} ${
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
          Retire and revoke
        </Button>
        )}
      </ModalFooter>
      </PushedStep>
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
/**
 * Whether the action ran. A FAILED attempt keeps its button — retiring the
 * control on any outcome at all would leave a refusal on screen with no way to
 * try again.
 */
function succeeded(outcome: Outcome | null): boolean {
  return outcome !== null && !statesNothingChanged(outcome.kind);
}

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
      <p className="mb-2.5 max-w-[60ch] text-[13.5px] leading-[1.55] text-ink/80">
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
              They would also lose <RoleRef projectId={rule.target_project} roleKey={rule.target_role} /> by <Term name="cascade">cascade</Term> — this role is that rule&rsquo;s input.
            </span>
          </li>
        ))}
      </ul>

      {outcome && <ActionOutcome outcome={outcome} className="mb-4" />}

      <div className="flex flex-wrap items-center gap-2.5">
        {/* This panel has no `open`/`onClose` of its own — it stays mounted in
            the workspace column until `onCancel` clears `pendingRemoval`, and
            nothing did that on success. Without this wrapper "Drop it" stayed
            live after it had already dropped, and the next click dropped a
            role already gone from the draft. Not `dangerConfirm`. That
            treatment is for a click that takes access away, and this one
            edits a draft — dressing it as the destructive act is the same
            misreading the copy above used to invite. The destructive confirm
            lives on Publish, where the revocation actually happens. */}
        {!succeeded(outcome) && (
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
        )}
        <Button variant={succeeded(outcome) ? "outline" : "ghost"} onClick={onCancel}>
          {succeeded(outcome) ? "Close" : "Cancel"}
        </Button>
      </div>
    </div>
  );
}

/** The roles an include's pinned version gives, once its versions are read. */
function pinnedRoles(include: BundleInclude, versions: BundleVersion[] | undefined): BundleRoleRow[] {
  return versions?.find((v) => v.version === include.child_version)?.roles ?? [];
}

/**
 * An included bundle on its parent's What it gives tab (B8, 2e): pinned to a
 * version, each role saying where it comes from, changed on its own page. A
 * newer version is offered here and reaches nobody until the parent publishes.
 */
function IncludedBundleCard({
  parentId,
  parentName,
  include,
  versions,
  catalog,
}: {
  parentId: string;
  parentName: string;
  include: BundleInclude;
  versions: BundleVersion[] | undefined;
  catalog: Map<string, CatalogRole>;
}) {
  const remove = useRemoveBundleInclude(parentId);
  const update = useSetBundleInclude(parentId);
  const [failure, setFailure] = useState<Outcome | null>(null);
  const child = include.child_name;
  const childHref = `/bundles/${encodeURIComponent(include.child_bundle_id)}`;
  const roles = pinnedRoles(include, versions);
  const newer = include.child_latest_version > include.child_version;
  const have = new Set(roles.map(idOf));
  const adds = newer
    ? (versions?.find((v) => v.version === include.child_latest_version)?.roles ?? []).filter(
        (r) => !have.has(idOf(r)),
      )
    : [];
  const label = (r: BundleRoleRow) => {
    const entry = catalog.get(idOf(r));
    return roleLabel(entry?.project_name || r.zitadel_project_id, r.zitadel_role_key, entry?.display_name);
  };
  async function run(act: () => Promise<unknown>) {
    setFailure(null);
    try {
      await act();
    } catch (error) {
      setFailure(outcomeFromError(error));
    }
  }

  return (
    <Card>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-5 py-4">
        <Link
          href={childHref}
          className="type-card-title text-ink motion-tint hover:text-accent-text hover:underline hover:underline-offset-[3px]"
        >
          {child}
        </Link>
        <Chip>Included bundle</Chip>
        <span className="text-[13.5px] text-faint">
          v{include.child_version} · {roles.length} {roles.length === 1 ? "role" : "roles"}
        </span>
        <span className="flex-1" />
        <Button
          size="sm"
          isPending={remove.isPending}
          onClick={() => run(() => remove.mutateAsync(include.child_bundle_id))}
        >
          Remove from {parentName}
        </Button>
      </div>
      {newer && (
        // A next step, not a fault: the lead and its caption sit on the row with
        // the action, with no box around them.
        <div className="row-divider flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3">
          <span className="min-w-[min(300px,100%)] flex-1 text-[14px] leading-[1.5] text-muted [text-wrap:pretty]">
            <strong className="font-semibold text-ink">
              {child} v{include.child_latest_version} is out.
            </strong>{" "}
            {adds.length > 0 && `It adds ${formatList(adds.map(label))}. `}
            Nothing changes for {parentName} until you add it and publish.
          </span>
          <Button
            size="sm"
            isPending={update.isPending}
            onClick={() =>
              run(() => update.mutateAsync({ child: include.child_bundle_id, version: include.child_latest_version }))
            }
          >
            Add to next version
          </Button>
        </div>
      )}
      {failure && <ActionOutcome outcome={failure} className="mx-5 my-3" />}
      {roles.map((role) => {
        const entry = catalog.get(idOf(role));
        return (
          <div key={idOf(role)} className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-5 py-3">
            <div className="min-w-[min(260px,100%)] flex-1">
              <span className="text-[15px] font-semibold">
                {entry?.display_name || humanizeKey(role.zitadel_role_key)}
              </span>
              {entry?.waits_for_send && <WaitsMark className="ml-2" />}{" "}
              <span className="text-[13.5px] text-muted">
                ·{" "}
                <Link
                  href={`/projects/${encodeURIComponent(role.zitadel_project_id)}`}
                  className="motion-tint hover:text-accent-text hover:underline hover:underline-offset-[3px]"
                >
                  {entry?.project_name || role.zitadel_project_id}
                </Link>
              </span>
              {entry?.description && (
                <p className="mt-0.5 max-w-[80ch] text-[13.5px] text-muted [text-wrap:pretty]">{entry.description}</p>
              )}
            </div>
            <span className="text-[13px] text-faint">Comes with {child}</span>
          </div>
        );
      })}
      <div className="row-divider px-5 py-3">
        <ArrowLink href={childHref}>Change what&rsquo;s in {child}</ArrowLink>
      </div>
    </Card>
  );
}

/**
 * An included bundle's three counts (Bundles 6): who has it directly, who has
 * it through the bundles that include it, and who is behind the latest either
 * way. Each jumps to its group on the People tab.
 */
function IncludedStats({
  latest,
  holders,
  includers,
}: {
  latest: number;
  holders: BundleHolder[];
  includers: BundleIncluder[];
}) {
  const byVersion = new Map<number, number>();
  for (const h of holders) byVersion.set(h.version, (byVersion.get(h.version) ?? 0) + 1);
  const versions = Array.from(byVersion.entries()).sort((a, b) => b[0] - a[0]);
  const through = includers.reduce((n, i) => n + i.people, 0);
  const directBehind = holders.filter((h) => h.version < latest).length;
  const lagging = includers.filter((i) => i.included_version > 0 && i.included_version < latest);
  const throughBehind = lagging.reduce((n, i) => n + i.people, 0);
  const behind = directBehind + throughBehind;
  const oldest = versions.find(([v]) => v < latest)?.[0];

  return (
    <div className="grid gap-4 [grid-template-columns:repeat(auto-fit,minmax(220px,1fr))]">
      <StatCard
        label="Directly"
        value={String(holders.length)}
        detail={versions.length ? versions.map(([v, n]) => `${n} on v${v}`).join(" · ") : "Nobody has it directly"}
        href={`#on-v${versions[0]?.[0] ?? latest}`}
      />
      <StatCard
        label="Through bundles"
        value={String(through)}
        detail={`In ${formatList(includers.map((i) => i.name))}`}
        href="#through-bundles"
      />
      <StatCard
        label={`Behind v${latest}`}
        value={String(behind)}
        tone={behind > 0 ? "warn" : "neutral"}
        detail={
          behind === 0
            ? `Everyone is on v${latest}`
            : [
                directBehind ? `${directBehind} directly` : null,
                throughBehind ? `${throughBehind} through ${formatList(lagging.map((i) => i.name))}` : null,
              ]
                .filter(Boolean)
                .join(" · ")
        }
        href={oldest !== undefined ? `#on-v${oldest}` : "#through-bundles"}
      />
    </div>
  );
}
