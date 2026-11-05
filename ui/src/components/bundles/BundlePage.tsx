"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";

import { AddRolesToBundle } from "@/components/bundles/AddRolesToBundle";
import { BundlePeople, BundleVersions, ThroughOtherBundles } from "@/components/bundles/BundleVersions";
import { ChangeLines } from "@/components/bundles/IncludeChangeLine";
import { TabLabel } from "@/components/bundles/TabLabel";
import { RetiredBundlePage } from "@/components/bundles/RetiredBundle";
import { ProjectName, RoleRef } from "@/components/names";
import { projectLabel } from "@/components/names/Unresolved";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Chip, Mono, StatusDot } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { KEPT, PushedStep, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { Card, CardHeader } from "@/components/ui/Card";
import { FormPart, useAddedKeys } from "@/components/ui/FormPart";
import { FieldLabel, Input } from "@/components/ui/Input";
import { useLeavingRows } from "@/lib/motion";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import { Tabs } from "@/components/ui/Tabs";
import { Term } from "@/components/ui/Term";
import { Unfold } from "@/lib/motion";
import { retireOutcome } from "@/lib/retry";
import { glideTo } from "@/lib/motion";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { formatList, formatShortDate, roleLabel } from "@/lib/format";
import {
  outcomeFromError,
  succeeded,
  type ActionOutcome as Outcome,
} from "@/lib/outcome";
import { useCrumb } from "@/lib/page-crumb";
import { useSeededBundleName } from "@/lib/queries/seed";
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
import { useInsightsWhatIf, type RoleRef as RoleRefValue } from "@/lib/queries/useInsights";
import { useGlobalRoleCatalog, type CatalogRole } from "@/lib/queries/useRoles";
import dynamic from "next/dynamic";
import { compareNames, sortByName, sortNames } from "@/lib/sort";
import { roleName } from "@/lib/names";

// Opened on demand, so it stays out of the page's first load.
const PublishVersionDialog = dynamic(() => import("@/components/bundles/PublishVersionDialog").then((m) => m.PublishVersionDialog));

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
  // The name from the list the reader came from, before this page's own read lands (A-19 P1).
  const seeded = useSeededBundleName(bundleId);
  useCrumb(bundle?.name ?? seeded);

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
    requestAnimationFrame(() => {
      const target = document.getElementById(link.hash.slice(1));
      if (target) glideTo(target);
    });
  }

  if (!bundle) {
    // Not on the list: still loading it, or retired (the list leaves retired bundles out, but
    // its page still opens), or never there.
    if (bundles.isLoading || bundles.error) {
      return (
        <div className="flex flex-col gap-4">
          <PageHeader title={seeded ?? "Bundle"} lede="What it gives, who has it, and its history." />
          <Card>
            <ListStates
              isLoading={bundles.isLoading}
              error={bundles.error}
              isEmpty
              onRetry={() => bundles.refetch()}
              errorTitle="Couldn't load this bundle."
              skeleton={<RowSkeleton rows={3} avatar={false} label="Loading the bundle" />}
              empty={null}
            >
              {null}
            </ListStates>
          </Card>
        </div>
      );
    }
    return <RetiredBundlePage bundleId={bundleId} />;
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
  // Who has it is what Zitadel shows; the assignments explain it and are what
  // a publish or a retire moves (`holders`, below).
  const seen = bundle.observed_holder_count;
  const people =
    seen === undefined || seen === null
      ? `Assigned to ${holders}, not checked in Zitadel`
      : `${seen} ${seen === 1 ? "person has" : "people have"} it${seen === holders ? "" : `, assigned to ${holders}`}`;
  const welcome = (bundles.data ?? []).find((b) => b.is_welcome);

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={name}
        meta={
          <MetaRow>
            {[
              latest ? `v${latest.version}, published ${formatShortDate(latest.published_at)}` : null,
              `${roleCount} ${roleCount === 1 ? "role" : "roles"}`,
              people,
            ]}
          </MetaRow>
        }
        lede={bundle.description || "What it gives, who has it, and its history."}
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

      <Unfold open={pending > 0 && Boolean(draft.data)} gap={16}>
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

      <>
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
      </>

      <EndCards
        bundleId={bundleId}
        name={name}
        holders={holders}
        isWelcome={bundle.is_welcome ?? false}
        welcomeName={welcome?.name}
        includedIn={sortNames(bundle.included_in ?? [])}
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
  const discardDraft = async () => {
    setFailure(null);
    try {
      await discard.mutateAsync();
    } catch (error) {
      setFailure(outcomeFromError(error));
    }
  };
  return (
    <Card>
      <CardHeader
        title="Not published yet"
        count={n}
        action={
          <>
            <Button
              variant="ghost"
              size="sm"
              disabled={discard.isPending}
              isPending={discard.isPending}
              onClick={discardDraft}
            >
              Discard changes
            </Button>
            <Button variant="accent" size="sm" onClick={onPublish}>
              Publish v{draft.next_version}
            </Button>
          </>
        }
      />
      <ChangeLines draft={draft} className="px-5 pb-3" />
      <p className="px-5 pb-4 text-[13px] text-muted">
        {holders === 0
          ? `${name} is assigned to nobody yet, so publishing moves nobody.`
          : `Publishing asks whether the ${holders} ${
              holders === 1 ? "person" : "people"
            } ${name} is assigned to move to v${draft.next_version}.`}
      </p>
      {failure && <ActionOutcome outcome={failure} word="Not discarded" className="mx-5 mb-4" />}
    </Card>
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
  // Form parts: a role added while you are here opens to its height; one that goes fades where it
  // was, then its gap settles. What the editor opens with is simply there.
  const read = !roles.isLoading && !roles.error;
  const drawn = useLeavingRows(rows, idOf, read);
  const arrived = useAddedKeys(rows.map(idOf), read);
  const live = new Set(rows.map(idOf));
  const groups = useMemo(() => {
    // By project id, so the heading can link to the project; the name is only
    // what it is called. A project nobody could name has no name here and is
    // headed by <ProjectName/>'s fallback, never by its id.
    const out = new Map<string, { name: string | undefined; list: BundleRoleRow[] }>();
    for (const r of drawn.rows) {
      const group = out.get(r.zitadel_project_id) ?? {
        name: byId.get(idOf(r))?.project_name,
        list: [],
      };
      group.list.push(r);
      out.set(r.zitadel_project_id, group);
    }
    // Projects A to Z, and the roles inside each A to Z by their shown name.
    const nameOfRole = (r: BundleRoleRow) => roleName(byId.get(idOf(r))?.display_name, r.zitadel_role_key);
    return Array.from(out.entries())
      .sort(([, a], [, b]) => compareNames(projectLabel(a.name, null), projectLabel(b.name, null)))
      .map(([id, g]) => [id, { ...g, list: sortByName(g.list, nameOfRole, idOf) }] as const);
  }, [drawn.rows, byId]);
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
        {sortByName(includes, (i) => i.child_name, (i) => i.child_bundle_id).map((include) => (
          <IncludedBundleCard
            key={include.child_bundle_id}
            parentId={bundleId}
            parentName={name}
            include={include}
            versions={childVersions[include.child_bundle_id]}
            catalog={byId}
          />
        ))}
        {groups.map(([projectId, { name: project, list }]) => (
          <Card key={projectId}>
            <CardHeader
              title={
                <ProjectName id={projectId} name={project} />
              }
              count={list.filter((role) => live.has(idOf(role))).length}
            />
            {list.map((role) => {
              const id = idOf(role);
              const entry = byId.get(id);
              const isLeaving = leavingIds.has(id);
              const isNew = added.has(id);
              const open = removing !== null && idOf(removing) === id;
              return (
                <FormPart key={id} appear={arrived.has(id)} {...drawn.props(id)}>
                <div className={`row-divider motion-tint ${isLeaving ? "bg-warn-soft" : ""}`}>
                  <div className="flex flex-wrap items-center gap-x-[18px] gap-y-2 px-5 py-3.5">
                    <div className="min-w-[min(240px,100%)] flex-1">
                      <span className="text-[15px] font-semibold">
                        {roleName(entry?.display_name, role.zitadel_role_key)}
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
                </FormPart>
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
              ? `${name} is given to new members now.`
              : welcomeName
                ? `${welcomeName} is given to new members now.`
                : "No bundle is given to new members now. One bundle at a time."}
          </p>
        </div>
        {!isWelcome && (
          <Button
            size="sm"
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
      {outcome && <ActionOutcome outcome={outcome} word="Default set" className="mx-5 my-3" />}
      <div className="flex flex-wrap items-center gap-3 px-5 py-4">
        <div className="min-w-[240px] flex-1">
          <div className="text-[14.5px] font-semibold">Retire {name}</div>
          <p className="mt-0.5 text-[13px] text-muted">
            {refused ??
              (holders === 0
                ? "It is assigned to nobody, so retiring it takes nothing from anyone."
                : `The ${holders} ${holders === 1 ? "person" : "people"} it is assigned to lose whatever only ${name} gave them. You see who and what before anything is sent.`)}
          </p>
        </div>
        {/* The line beside it is the reason; Button's own would say it twice. */}
        <Button variant="danger" size="sm" disabled={Boolean(refused)} onClick={onRetire}>
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
        lede="This is what the bundle is called, not what it grants."
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
      {outcome && (
        <ActionOutcome
          outcome={outcome}
          word={succeeded(outcome) ? (trimmed === name ? "Saved" : "Renamed") : "Not saved"}
          className="mx-6 mb-1"
        />
      )}

      <ModalFooter cancel={succeeded(outcome) ? undefined : { onClick: onClose, disabled: update.isPending }}>
        {/* Closing on success used to fire in the same breath as `setOutcome`,
            which unmounts this dialog before the operator can read what it
            just set — "Now called X" was computed and never shown. Held open
            until Close is pressed instead, same as the delete dialog. */}
        {!succeeded(outcome) ? (
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
        ) : (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        )}
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
 * Retire keeps its history (owner, 19 Oct 2026): the bundle stops being given, and its versions
 * stay readable on its page. The confirm says "and revoke" only when someone would lose something.
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
            ? "It is assigned to nobody. Retiring it takes nothing away from anybody."
            : `It is assigned to ${holders} ${holders === 1 ? "person" : "people"} right now.`
        }
      />

      <div className="px-6">
        <div className="danger-note px-4 py-3.5 text-[14px] leading-[1.55]">
          <div className="type-label mb-1 text-danger-text">What happens</div>
          <ul className="flex flex-col gap-1 text-muted">
            {holders > 0 && (
              <li>
                Everyone it is assigned to loses whichever of its {roleCount}{" "}
                {roleCount === 1 ? "role" : "roles"} nothing else gives them. A rule or direct
                access to the same role keeps it.
              </li>
            )}
            {isWelcome && (
              <li className="font-semibold text-danger-text">
                This is the default for new members. Onboarding will stop handing anything out
                until another bundle is set.
              </li>
            )}
            <li>It can&rsquo;t be given again. Its history stays readable.</li>
          </ul>
        </div>
      </div>

      {outcome && <ActionOutcome outcome={outcome} word={gone ? "Retired" : "Not retired"} className="mx-6 mb-1" />}
      {gone && step.after}

      <ModalFooter
        // Static across `gone`, it used to recommend an alternative to a
        // click that had already happened: "empty it instead" above a
        // bundle already deleted, with nothing left to empty.
        note={step.note ?? (gone ? undefined : "Emptying the bundle instead leaves it assignable and grants nothing.")}
        cancel={gone ? undefined : { onClick: onCancel, disabled: remove.isPending }}
      >
        {/* `onDeleted` clears the parent's selection, and the parent's own
            comment says why it must: the deleted bundle stays selected
            otherwise, and the list falls back to the first bundle under the
            old id's heading. Called from HERE rather than from the mutation,
            because clearing the selection unmounts this dialog, and the
            outcome the operator has not read yet goes with it. */}
        {gone && (
          <Button variant="outline" onClick={onDeleted}>
            Close
          </Button>
        )}
        {!gone && (
        <Button
          variant="dangerConfirm"
          // A dialog must not open with the cursor on the button that revokes access.
          data-skip-initial-focus
          isPending={remove.isPending}
          onClick={async () => {
            try {
              const result = await step.run(() => remove.mutateAsync(bundleId));
              if (result === KEPT) return;
              const n = result.cascade?.enqueued ?? 0;
              // What the server says happened: any wait reason means some
              // of it is not in Zitadel yet, and which one says who moves it.
              const reason = result.cascade?.wait_reason;
              // The welcome-bundle consequence is folded into the same report
              // rather than fired as a second notification. It is the same
              // event, and it is the half an operator most needs: from now on
              // a new member receives nothing.
              const orphaned = result.was_welcome
                ? " New members no longer receive a bundle. Set another as the default."
                : "";
              setOutcome(
                n === 0
                  ? {
                      kind: "applied",
                      message: `Nobody's access changed. It carried nothing anybody held.`,
                      detail: `Its history stays readable.${orphaned}`,
                    }
                  : retireOutcome(n, reason, result.cascade?.drain?.retry, orphaned),
              );
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          {holders === 0 ? `Retire ${name}` : "Retire and revoke"}
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
  const remove = useRemoveBundleRole(bundleId);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const holders = impact.data?.users ?? [];

  // Who would lose what is the backend's answer (the what-if), computed by the
  // same closure the publish runs: every rule, down the whole chain, and every
  // other source that keeps the role. This panel used to walk the rule list
  // itself, one hop, and named only the first rule that produced the role.
  const whatIf = useInsightsWhatIf({
    kind: "bundle_gives_not",
    bundle_id: bundleId,
    project_id: role.zitadel_project_id,
    role_key: role.zitadel_role_key,
  });
  const key = `${role.zitadel_project_id}:${role.zitadel_role_key}`;
  const losses = whatIf.data?.losses ?? [];
  const lose = losses.filter((p) => p.roles.some((r) => `${r.project_id}:${r.role_key}` === key)).length;
  const keep = Math.max(holders.length - lose, 0);
  // What goes with it: every role a rule gave only through this one.
  const alsoLost = useMemo(() => {
    const seen = new Map<string, RoleRefValue>();
    for (const person of whatIf.data?.losses ?? [])
      for (const r of person.roles) {
        const k = `${r.project_id}:${r.role_key}`;
        if (k !== key) seen.set(k, r);
      }
    return Array.from(seen.values());
  }, [whatIf.data, key]);

  return (
    <div className="border-t border-line pt-4">
      <div className="type-label mb-1">
        Removing <RoleRef projectId={role.zitadel_project_id} roleKey={role.zitadel_role_key} /> from the next
        version
      </div>
      <p className="mb-2.5 max-w-[60ch] text-[13.5px] leading-[1.55] text-muted">
        This is what would happen to the {holders.length} {holders.length === 1 ? "person" : "people"}{" "}
        {bundleName} is assigned to <em>if</em> you publish the next version and move them onto it.
      </p>

      <ul className="mb-4 flex flex-col gap-2 text-[14px] leading-[1.5]">
        <li className="flex items-start gap-2.5">
          <StatusDot tone="danger" className="mt-[7px] flex-none" />
          <span>
            {whatIf.isLoading ? (
              <span className="text-muted">Working out who would lose it.</span>
            ) : whatIf.error ? (
              <span className="text-muted">Couldn&rsquo;t work out who would lose it. Try again in a moment.</span>
            ) : (
              <strong className="font-semibold">
                {lose} {lose === 1 ? "person would lose" : "people would lose"} it
              </strong>
            )}
          </span>
        </li>

        {!whatIf.isLoading && !whatIf.error && keep > 0 && (
          <li className="flex items-start gap-2.5">
            <StatusDot tone="neutral" className="mt-[7px] flex-none" />
            <span className="text-muted">
              {keep} {keep === 1 ? "person keeps" : "people keep"} it, because something else gives it to them: an
              automatic rule, direct access or an included bundle.
            </span>
          </li>
        )}

        {alsoLost.map((r) => (
          <li key={`${r.project_id}:${r.role_key}`} className="flex items-start gap-2.5">
            <StatusDot tone="neutral" className="mt-[7px] flex-none" />
            <span className="text-muted">
              Some would also lose <RoleRef projectId={r.project_id} roleKey={r.role_key} /> by{" "}
              <Term name="cascade">cascade</Term>, because an automatic rule gave it to them from this role.
            </span>
          </li>
        ))}
      </ul>

      {outcome && (
        <ActionOutcome
          outcome={outcome}
          word={succeeded(outcome) ? "Removed" : "Not removed"}
          className="mb-4"
        />
      )}

      <div className="flex flex-wrap items-center gap-2.5">
        {/* This panel has no `open`/`onClose` of its own: it stays mounted in
            the workspace column until `onCancel` clears `pendingRemoval`, and
            nothing did that on success. Without this wrapper the button stayed
            live after it had already removed, and the next click removed a
            role already gone from the next version. Not `dangerConfirm`. That
            treatment is for a click that takes access away, and this one
            edits the next version. The destructive confirm lives on Publish,
            where the revocation actually happens. */}
        {!succeeded(outcome) && (
          <Button
            variant="accent"
            size="sm"
            isPending={remove.isPending}
            onClick={async () => {
              try {
                await remove.mutateAsync({
                  projectId: role.zitadel_project_id,
                  roleKey: role.zitadel_role_key,
                });
                setOutcome({
                  // Nobody's access moved: the next version did. Same distinction
                  // the add-roles panel makes, in the other direction.
                  kind: "no_change",
                  message: `Removed from ${bundleName}'s next version`,
                  detail: "Nobody loses it until you publish a version and move them onto it.",
                });
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            Remove from the next version
          </Button>
        )}
        <Button variant={succeeded(outcome) ? "outline" : "ghost"} size="sm" onClick={onCancel}>
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
    return roleLabel(projectLabel(entry?.project_name, r.zitadel_project_id), r.zitadel_role_key, entry?.display_name);
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
          className="type-card-title text-ink link-hover"
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
      {failure && <ActionOutcome outcome={failure} word="Not changed" className="mx-5 my-3" />}
      {sortByName(
        roles,
        (r) => roleName(catalog.get(idOf(r))?.display_name, r.zitadel_role_key),
        idOf,
      ).map((role) => {
        const entry = catalog.get(idOf(role));
        return (
          <div key={idOf(role)} className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-5 py-3">
            <div className="min-w-[min(260px,100%)] flex-1">
              <span className="text-[15px] font-semibold">
                {roleName(entry?.display_name, role.zitadel_role_key)}
              </span>
              {entry?.waits_for_send && <WaitsMark className="ml-2" />}{" "}
              <span className="text-[13.5px] text-muted">
                ·{" "}
                <ProjectName id={role.zitadel_project_id} name={entry?.project_name} />
              </span>
              {entry?.description && (
                <p className="mt-0.5 max-w-[80ch] text-[13.5px] text-muted [text-wrap:pretty]">{entry.description}</p>
              )}
            </div>
            <span className="text-[13px] text-faint">Comes with {child}</span>
          </div>
        );
      })}
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
        detail={`In ${formatList(sortNames(includers.map((i) => i.name)))}`}
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
                throughBehind ? `${throughBehind} through ${formatList(sortNames(lagging.map((i) => i.name)))}` : null,
              ]
                .filter(Boolean)
                .join(" · ")
        }
        href={oldest !== undefined ? `#on-v${oldest}` : "#through-bundles"}
      />
    </div>
  );
}
