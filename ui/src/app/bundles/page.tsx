"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import { ProjectName, UserName } from "@/components/names";
import {
  useAddBundleRole,
  useBundleImpact,
  useBundleRoles,
  useBundles,
  useCreateBundle,
  useRemoveBundleRole,
  useSetWelcomeBundle,
  type BundleRoleRow,
} from "@/lib/queries/useBundles";
import { useMappingRules } from "@/lib/queries/useMappingRules";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";

/**
 * S1 · Bundles. Advanced, because this is the machine that acts on everyone:
 * a bundle edit cascades to every holder at once.
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
                <span className="text-[13.5px] text-faint">{bundle.holder_count ?? 0}</span>
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

  const roleRows = roles.data ?? [];

  return (
    <div className="flex min-w-[420px] flex-1 flex-wrap items-start gap-5">
      <Card className="min-w-[340px] flex-1">
        <CardHeader
          title={name}
          note={`${roleRows.length} ${roleRows.length === 1 ? "role" : "roles"} · ${holders} ${
            holders === 1 ? "holder" : "holders"
          }`}
        />

        <div className="row-divider px-5 py-3 text-[13.5px] leading-[1.55] text-muted">
          Adding or removing a role here changes access for all {holders} of them at the next
          cascade. It is not a per-person action.
        </div>

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
              guidance="Add one below. Until then, assigning it grants nothing."
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
                  selectedForRemoval ? "bg-danger-soft" : ""
                }`}
              >
                <span className="min-w-0 flex-1 truncate">
                  <ProjectName id={role.zitadel_project_id} /> /{" "}
                  <Mono>{role.zitadel_role_key}</Mono>
                </span>
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => setPendingRemoval(selectedForRemoval ? null : role)}
                >
                  Remove
                </Button>
              </div>
            );
          })}
        </ListStates>

        <AddRoleRow bundleId={bundleId} name={name} holders={holders} />

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
          <HoldersPanel
            name={name}
            holders={impact.data?.users ?? []}
            isLoading={impact.isLoading}
          />
        )}
      </div>
    </div>
  );
}

/**
 * The impact panel replaces the space a confirmation dialog would have taken.
 * It names, before the click: who loses the role outright, who keeps it through
 * another source, and what cascades away with it.
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
    <div className="rounded-card border border-danger-line bg-danger-soft p-5">
      <div className="type-label mb-2.5 text-danger-text">
        Removing <ProjectName id={role.zitadel_project_id} /> / {role.zitadel_role_key} affects{" "}
        {holders.length} {holders.length === 1 ? "person" : "people"} now
      </div>

      <ul className="mb-4 flex flex-col gap-2 text-[14px] leading-[1.5]">
        <li className="flex items-start gap-2.5">
          <span aria-hidden className="mt-[7px] h-2 w-2 flex-none rounded-pill bg-danger" />
          <span>
            <strong className="font-semibold">
              {holders.length} {holders.length === 1 ? "person loses" : "people lose"} it through
              this bundle
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
              Anybody holding <ProjectName id={coveringRule.source_project} /> /{" "}
              <Mono>{coveringRule.source_role}</Mono> keeps it — an automatic rule produces it
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
              They also lose <ProjectName id={rule.target_project} /> /{" "}
              <Mono>{rule.target_role}</Mono> by cascade — this role is that rule&rsquo;s input.
            </span>
          </li>
        ))}
      </ul>

      <div className="flex flex-wrap items-center gap-2.5">
        <Button
          variant="dangerConfirm"
          isPending={remove.isPending}
          onClick={async () => {
            try {
              await remove.mutateAsync({
                projectId: role.zitadel_project_id,
                roleKey: role.zitadel_role_key,
              });
              toast.success(`Removed from ${bundleName}.`);
              onCancel();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "The removal didn't go through.");
            }
          }}
        >
          Remove role from bundle
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

function AddRoleRow({
  bundleId,
  name,
  holders,
}: {
  bundleId: string;
  name: string;
  holders: number;
}) {
  const addRole = useAddBundleRole(bundleId);
  const projects = useProjects();
  const catalog = useGlobalRoleCatalog();
  const [projectId, setProjectId] = useState("");
  const [roleKey, setRoleKey] = useState("");

  const projectRoles = (catalog.data ?? []).filter((role) => role.project_id === projectId);

  return (
    <div className="row-divider flex flex-wrap items-end gap-2.5 px-5 py-4">
      <div className="min-w-[160px] flex-1">
        <FieldLabel htmlFor="bundle-project">Add a role — project</FieldLabel>
        <Select
          id="bundle-project"
          value={projectId}
          onChange={(event) => {
            setProjectId(event.target.value);
            setRoleKey("");
          }}
        >
          <option value="">Choose…</option>
          {(projects.data ?? []).map((entry) => (
            <option key={entry.project.id} value={entry.project.id}>
              {entry.project.name}
            </option>
          ))}
        </Select>
      </div>
      <div className="min-w-[160px] flex-1">
        <FieldLabel htmlFor="bundle-role">Role</FieldLabel>
        <Select
          id="bundle-role"
          value={roleKey}
          disabled={!projectId}
          onChange={(event) => setRoleKey(event.target.value)}
        >
          <option value="">{projectId ? "Choose…" : "Pick a project"}</option>
          {projectRoles.map((role) => (
            <option key={role.role_key} value={role.role_key}>
              {role.display_name || humanizeKey(role.role_key)}
            </option>
          ))}
        </Select>
      </div>
      <Button
        variant="accent"
        disabled={!projectId || !roleKey}
        isPending={addRole.isPending}
        onClick={async () => {
          try {
            await addRole.mutateAsync({ project_id: projectId, role_key: roleKey });
            toast.success(`Added to ${name}. ${holders} holders will get it.`);
            setRoleKey("");
          } catch (error) {
            toast.error(error instanceof Error ? error.message : "That didn't save.");
          }
        }}
      >
        Add role
      </Button>
    </div>
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
