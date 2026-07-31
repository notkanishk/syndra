"use client";

import { useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import {
  useAddBundleRole,
  useBundleImpact,
  useBundleRoles,
  useBundles,
  useCreateBundle,
  useSetWelcomeBundle,
} from "@/lib/queries/useBundles";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";

/**
 * S1 · Bundles. Advanced, because this is the machine that acts on everyone:
 * a bundle edit cascades to every holder.
 *
 * Assigning a bundle to one person is Basic and lives on their page. The rule
 * generalises: acting on one person is Basic; changing the thing that acts on
 * everyone is Advanced.
 *
 * The impact preview is the safety rail, so it sits beside the roles rather
 * than behind a disclosure.
 */
export default function BundlesPage() {
  const bundles = useBundles();
  const [selected, setSelected] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const rows = bundles.data ?? [];
  const active = selected ?? rows[0]?.id ?? null;

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
        <Card className="min-w-[380px] flex-1">
          <CardHeader title="Every bundle" count={rows.length} />
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
                guidance="A bundle is how a set of roles gets handed out as one unit — a Lab Tech, a Shop Steward."
                action={{ label: "Create the first one", onClick: () => setCreating(true) }}
              />
            }
          >
            {rows.map((bundle) => (
              <button
                key={bundle.id}
                type="button"
                onClick={() => setSelected(bundle.id)}
                aria-current={active === bundle.id ? "true" : undefined}
                className={`row-divider flex w-full items-center gap-3 px-5 py-3.5 text-left transition-colors ${
                  active === bundle.id ? "bg-accent-soft/60" : "hover:bg-[var(--hover)]"
                }`}
              >
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[15px] font-semibold">{bundle.name}</div>
                  <div className="truncate text-[13.5px] text-faint">
                    {bundle.description || "No description"}
                  </div>
                </div>
                {bundle.is_welcome && <Badge>Default for new members</Badge>}
              </button>
            ))}
          </ListStates>
        </Card>

        <div className="min-w-[420px] flex-1">
          {active ? (
            <BundleDetail
              bundleId={active}
              name={rows.find((bundle) => bundle.id === active)?.name ?? ""}
              isWelcome={rows.find((bundle) => bundle.id === active)?.is_welcome ?? false}
            />
          ) : null}
        </div>
      </div>

      <CreateBundleDialog open={creating} onClose={() => setCreating(false)} />
    </div>
  );
}

function BundleDetail({
  bundleId,
  name,
  isWelcome,
}: {
  bundleId: string;
  name: string;
  isWelcome: boolean;
}) {
  const roles = useBundleRoles(bundleId);
  const impact = useBundleImpact(bundleId);
  const setWelcome = useSetWelcomeBundle();
  const addRole = useAddBundleRole(bundleId);
  const projects = useProjects();
  const catalog = useGlobalRoleCatalog();

  const [projectId, setProjectId] = useState("");
  const [roleKey, setRoleKey] = useState("");

  const holders = impact.data?.users.length ?? 0;
  const projectRoles = (catalog.data ?? []).filter((role) => role.project_id === projectId);

  return (
    <Card>
      <CardHeader
        title={name}
        note={`${holders} ${holders === 1 ? "person holds" : "people hold"} this`}
      />

      {/* The safety rail, stated before any edit control. */}
      <div className="row-divider px-5 py-3.5 text-[13.5px] leading-[1.55] text-muted">
        Adding or removing a role here changes access for all {holders} of them at the next
        cascade. It is not a per-person action.
      </div>

      <ListStates
        isLoading={roles.isLoading}
        error={roles.error}
        isEmpty={(roles.data ?? []).length === 0}
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
        {(roles.data ?? []).map((role) => (
          <div
            key={`${role.zitadel_project_id}:${role.zitadel_role_key}`}
            className="row-divider flex items-center gap-3 px-5 py-3 text-[14.5px]"
          >
            <span className="min-w-0 flex-1 truncate">
              {role.zitadel_project_id} / <Mono>{role.zitadel_role_key}</Mono>
            </span>
          </div>
        ))}
      </ListStates>

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

      <div className="row-divider flex flex-wrap items-center gap-3 px-5 py-4">
        <div className="min-w-[240px] flex-1 text-[13.5px] text-muted">
          The default bundle is assigned automatically when somebody is onboarded.
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
          {isWelcome ? "Already the default" : "Make it the default for new members"}
        </Button>
      </div>
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
        lede="Name it after the person it describes, not the roles inside it — Lab Tech, not laser+3d+door."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="bundle-name">Name</FieldLabel>
          <Input
            id="bundle-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="Lab Tech"
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
