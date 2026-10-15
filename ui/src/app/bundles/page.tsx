"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { RolePicker, splitRoleId } from "@/components/bundles/RolePicker";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { Segmented, Select } from "@/components/ui/Select";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader, mountedWhileOpen } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { outcomeFromError, statesNothingChanged, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useBundleRoles, useBundles, useCreateBundle, type BundleRow } from "@/lib/queries/useBundles";
import { useCatalogUsers } from "@/lib/queries/useCatalogUsers";
import { useUserAccess } from "@/lib/queries/useUsers";

/**
 * S1 · Bundles (1c): a list, like Projects, and a page for each bundle.
 * Advanced, because this is the machine that acts on everyone: publishing a
 * version can change access for every holder at once.
 *
 * Name, what it's for, then Roles and People in the number seats. What needs
 * attention is written under the name in words. The list is short, so it has
 * no search.
 */
export default function BundlesPage() {
  const bundles = useBundles();
  const [creating, setCreating] = useState(false);
  const rows = [...(bundles.data ?? [])].sort((a, b) => a.name.localeCompare(b.name));

  return (
    <div className="flex flex-col gap-7">
      <PageHeader
        title="Bundles"
        lede="A bundle is a set of roles given together. Give someone a bundle and they get every role in it. Publish a new version and you choose whether they move to it."
        actions={
          <Button variant="accent" onClick={() => setCreating(true)}>
            New bundle
          </Button>
        }
      />

      <Card>
        <CardColumns>
          <span className="w-[min(260px,40%)]">Bundle ↑</span>
          <span className="hidden flex-1 tablet:block">What it&rsquo;s for</span>
          <span className="w-[110px] text-right">Roles</span>
          <span className="w-[110px] text-right">People</span>
        </CardColumns>
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
            <BundleListRow key={bundle.id} bundle={bundle} />
          ))}
        </ListStates>
      </Card>

      <CreateBundleDialog open={creating} onClose={() => setCreating(false)} />
    </div>
  );
}

/** One bundle: its name links to its page, and what needs attention says so in words. */
function BundleListRow({ bundle }: { bundle: BundleRow }) {
  const notes: { text: string; tone: string }[] = [];
  if ((bundle.unpublished_changes ?? 0) > 0) notes.push({ text: "Unpublished changes", tone: "text-accent-text" });
  if ((bundle.stale_holders ?? 0) > 0) {
    const n = bundle.stale_holders ?? 0;
    notes.push({ text: `${n} ${n === 1 ? "person" : "people"} on an older version`, tone: "text-warn-text" });
  }
  if (bundle.is_welcome) notes.push({ text: "Given to everyone who joins", tone: "text-faint" });
  return (
    <div className="row-divider flex flex-wrap items-start gap-x-[18px] gap-y-1 px-5 py-3.5">
      <div className="w-[min(260px,40%)] min-w-0">
        <Link
          href={`/bundles/${encodeURIComponent(bundle.id)}`}
          className="text-[15px] font-semibold text-ink motion-tint hover:text-accent-text hover:underline hover:underline-offset-[3px]"
        >
          {bundle.name}
        </Link>
        {notes.map((n) => (
          <span key={n.text} className={`block text-[12.5px] font-semibold ${n.tone}`}>
            {n.text}
          </span>
        ))}
      </div>
      <span className="hidden min-w-0 flex-1 text-[14px] text-muted [text-wrap:pretty] tablet:block">
        {bundle.description}
      </span>
      <span className="w-[110px] text-right text-[15px] font-semibold tabular-nums">{bundle.roles?.length ?? "—"}</span>
      <span className="w-[110px] text-right text-[15px] font-semibold tabular-nums">{bundle.holder_count ?? 0}</span>
    </div>
  );
}

/**
 * Creating a bundle, with the roles it is for.
 *
 * The roles are asked for here rather than added afterwards, and that is the
 * correction. A bundle is a set of roles handed out as one unit; creating one
 * empty published an empty v1, so assigning it granted nothing, and the roles
 * the operator then added were reported back to them as "2 unpublished
 * changes" against a version that had never described anything. There was no
 * point in that sequence where the screen was telling the truth about a bundle
 * the operator considered finished.
 *
 * So a bundle is born as the thing it was described as: name, purpose, and the
 * roles that go in v1.
 */
function CreateBundleDialogOpen({ onClose }: { open: boolean; onClose: () => void }) {
  const create = useCreateBundle();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());
  const copyInto = useCallback((ids: string[]) => setSelected(new Set(ids)), []);


  const trimmed = name.trim();
  const chosen = Array.from(selected);
  const blocked = !trimmed
    ? "A bundle needs a name."
    : chosen.length === 0
      ? "Pick at least one role. A bundle that grants nothing has nothing to hand out, and its first version would describe nothing."
      : undefined;

  return (
    <Modal open onClose={onClose} busy={create.isPending} size="lg" labelledBy="new-bundle-title">
      <ModalHeader
        title="New bundle"
        titleId="new-bundle-title"
        lede="Name it after who it's for. The roles you choose become version 1, so it can be given straight away."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div className="grid gap-3.5 tablet:grid-cols-2">
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
            <FieldLabel htmlFor="bundle-description">What it&rsquo;s for</FieldLabel>
            <Input
              id="bundle-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="Student staff who run the fabrication lab"
            />
          </div>
        </div>
        <StartFrom
          newName={trimmed || "this bundle"}
          disabled={create.isPending}
          onCopy={copyInto}
        />
        <div>
          <FieldLabel id="new-bundle-roles">What it gives</FieldLabel>
          <RolePicker
            labelledBy="new-bundle-roles"
            selected={selected}
            disabled={create.isPending}
            onToggle={(id) =>
              setSelected((prev) => {
                const next = new Set(prev);
                if (!next.delete(id)) next.add(id);
                return next;
              })
            }
          />
        </div>
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        {/* Closing on success used to fire alongside `setOutcome`, which
            unmounts this dialog before "X created" is ever painted — same
            fix as the rename dialog: hold it open, let Close close it. */}
        {!succeeded(outcome) && (
          <Button
            variant="accent"
            disabled={Boolean(blocked)}
            reason={blocked}
            isPending={create.isPending}
            onClick={async () => {
              try {
                await create.mutateAsync({
                  name: trimmed,
                  description,
                  roles: chosen.map((id) => {
                    const [project_id, role_key] = splitRoleId(id);
                    return { project_id, role_key };
                  }),
                });
                setOutcome({
                  kind: "applied",
                  message: `${trimmed} created`,
                  detail: `Its first version carries ${chosen.length} ${
                    chosen.length === 1 ? "role" : "roles"
                  }, so it can be assigned now. Nobody holds it yet.`,
                });
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {chosen.length === 0
              ? "Create bundle"
              : `Create with ${chosen.length} ${chosen.length === 1 ? "role" : "roles"}`}
          </Button>
        )}
        <Button
          onClick={() => {
            onClose();
            if (succeeded(outcome)) {
              setName("");
              setDescription("");
              setSelected(new Set());
              setOutcome(null);
            }
          }}
        >
          {succeeded(outcome) ? "Close" : "Cancel"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

const CreateBundleDialog = mountedWhileOpen(CreateBundleDialogOpen);

/** Whether the action ran; a failed attempt keeps its button so it can be tried again. */
function succeeded(outcome: Outcome | null): boolean {
  return outcome !== null && !statesNothingChanged(outcome.kind);
}

type Source = "none" | "person" | "bundle";

/**
 * Start from (B5, 1a): a copy, once. Choosing a bundle or a person ticks
 * what it gives, so the operator only unticks what doesn't belong; nothing
 * links back, so later changes there don't reach this bundle.
 */
function StartFrom({
  newName,
  disabled,
  onCopy,
}: {
  newName: string;
  disabled: boolean;
  onCopy: (ids: string[]) => void;
}) {
  const [source, setSource] = useState<Source>("none");
  const [bundleId, setBundleId] = useState("");
  const [personId, setPersonId] = useState("");
  const [search, setSearch] = useState("");
  const bundles = useBundles();
  const people = useCatalogUsers();
  const bundleRoles = useBundleRoles(source === "bundle" ? bundleId || null : null, { published: true });
  const access = useUserAccess(source === "person" ? personId : "");
  const [copied, setCopied] = useState<string | null>(null);

  const from = (bundles.data ?? []).find((b) => b.id === bundleId);
  const person = people.data.find((p) => p.id === personId);

  // The copy lands once, when what it copies has been read.
  const bundleIds = bundleRoles.data?.map((r) => `${r.zitadel_project_id}:${r.zitadel_role_key}`);
  const personIds = access.data?.projects.flatMap((p) => p.effective_role_keys.map((k) => `${p.project_id}:${k}`));
  const ready = source === "bundle" ? bundleIds : source === "person" ? personIds : undefined;
  const key = source === "bundle" ? `b:${bundleId}` : source === "person" ? `p:${personId}` : null;
  const readyKey = ready ? ready.join(",") : null;
  useEffect(() => {
    if (!key || readyKey === null || copied === key) return;
    setCopied(key);
    onCopy(readyKey ? readyKey.split(",") : []);
  }, [key, readyKey, copied, onCopy]);

  const matches = search.trim()
    ? people.data
        .filter((p) => `${p.name} ${p.email}`.toLowerCase().includes(search.trim().toLowerCase()))
        .slice(0, 5)
    : [];
  const n = ready?.length ?? 0;

  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className="text-[13.5px] font-semibold">Start from</span>
        <Segmented<Source>
          size="sm"
          label="Start from"
          value={source}
          onChange={(next) => {
            setSource(next);
            if (next === "none") {
              setCopied(null);
              setBundleId("");
              setPersonId("");
              onCopy([]);
            }
          }}
          options={[
            { value: "none", label: "Nothing" },
            { value: "person", label: "A person" },
            { value: "bundle", label: "Another bundle" },
          ]}
        />
        {source === "none" && (
          <span className="text-[13px] text-faint">Ticks what they have, so you only untick what doesn&rsquo;t belong.</span>
        )}
      </div>

      {source === "bundle" && (
        <Select
          aria-label="Copy from which bundle"
          value={bundleId}
          disabled={disabled}
          onChange={(e) => setBundleId(e.target.value)}
        >
          <option value="">Choose a bundle</option>
          {(bundles.data ?? []).map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </Select>
      )}

      {source === "person" && !personId && (
        <div className="flex flex-col gap-1.5">
          <Input
            aria-label="Choose a person"
            placeholder="Choose a person"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          {matches.map((p) => (
            <ChoiceRow
              key={p.id}
              single
              name="start-from-person"
              selected={false}
              onToggle={() => setPersonId(p.id)}
              title={p.name}
              description={[p.title, p.email].filter(Boolean).join(" · ")}
            />
          ))}
        </div>
      )}

      {ready && copied === key && (
        <p className="tint-in flex flex-wrap items-baseline gap-x-2 rounded-inner border border-accent-line bg-accent-soft px-3.5 py-2.5 text-[13.5px]">
          <span>
            <strong className="font-semibold text-accent-text">
              {source === "bundle"
                ? `Copied ${from?.name ?? "the bundle"}${from?.latest_version ? ` v${from.latest_version}` : ""}`
                : `Copied ${person?.name ?? "their"}${person ? "'s" : ""} access`}
              :
            </strong>{" "}
            {source === "bundle" ? "its" : "their"} {n} {n === 1 ? "role is" : "roles are"} ticked below. This is a copy.
            Later changes to {source === "bundle" ? from?.name ?? "it" : "their access"} don&rsquo;t reach {newName}.
          </span>
          <button
            type="button"
            className="font-semibold text-accent-text hover:brightness-110"
            onClick={() => {
              setSource("none");
              setCopied(null);
              setBundleId("");
              setPersonId("");
              onCopy([]);
            }}
          >
            Start empty
          </button>
        </p>
      )}
    </div>
  );
}
