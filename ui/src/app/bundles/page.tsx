"use client";

import { useMemo, useState } from "react";

import AddRolesToBundlePicker from "@/components/bundles/AddRolesToBundlePicker";
import BundleImpactAccordion from "@/components/bundles/BundleImpactAccordion";
import CreateBundleModal from "@/components/bundles/CreateBundleModal";
import CreateRoleModal from "@/components/roles/CreateRoleModal";
import { ProjectName, RoleName } from "@/components/names";
import { ConfirmationModeControls } from "@/components/policies/ConfirmationModeControls";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { useBundleRoles, useBundles, useSetWelcomeBundle, type BundleRow } from "@/lib/queries/useBundles";

export default function BundlesView() {
  const bundlesQuery = useBundles();
  const bundles = useMemo(() => bundlesQuery.data ?? [], [bundlesQuery.data]);
  const loading = bundlesQuery.isLoading;

  const [createBundleOpen, setCreateBundleOpen] = useState(false);
  const [createRoleOpen, setCreateRoleOpen] = useState(false);
  const [expandedBundleId, setExpandedBundleId] = useState<string | null>(null);
  // Picker target — when set, AddRolesToBundlePicker is open for this bundle.
  const [pickerBundle, setPickerBundle] = useState<BundleRow | null>(null);
  const [bulkEdit, setBulkEdit] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  function toggleSelected(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function exitBulkEdit() {
    setBulkEdit(false);
    setSelectedIds(new Set());
  }

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow>Bundles</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Reusable role bundles
        </h1>
        <p className="text-on-surface-variant mt-2">
          Group role sets into reusable assignments and inspect the user pool
          currently influenced by each bundle.
        </p>
      </header>

      <Card variant="glass">
        <div className="flex items-center justify-between mb-6 gap-3 flex-wrap">
          <CardTitle>Bundle library</CardTitle>
          <div className="flex items-center gap-2">
            <Button
              variant={bulkEdit ? "secondary" : "outline"}
              size="sm"
              onClick={() => (bulkEdit ? exitBulkEdit() : setBulkEdit(true))}
            >
              {bulkEdit ? "Done" : "Bulk edit"}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setCreateRoleOpen(true)}>
              + Create role
            </Button>
            <Button variant="primary" size="sm" onClick={() => setCreateBundleOpen(true)}>
              + Create bundle
            </Button>
          </div>
        </div>

        {bulkEdit && (
          <div className="mb-4">
            <ConfirmationModeControls kind="bundle" selectedIds={selectedIds} onDone={exitBulkEdit} />
          </div>
        )}

        {loading ? (
          <SkeletonCardList count={3} />
        ) : bundles.length === 0 ? (
          <EmptyState
            eyebrow="Empty library"
            title="No bundles yet"
            description="Group related roles into a bundle so you can assign access in one click instead of granting individual roles."
            action={{ label: "Create bundle", onClick: () => setCreateBundleOpen(true) }}
          />
        ) : (
          <div className="space-y-3">
            {bundles.map((bundle) => (
              <BundleRowCard
                key={bundle.id}
                bundle={bundle}
                expanded={expandedBundleId === bundle.id}
                onToggle={() => setExpandedBundleId((prev) => (prev === bundle.id ? null : bundle.id))}
                onOpenPicker={() => setPickerBundle(bundle)}
                bulkEdit={bulkEdit}
                selected={selectedIds.has(bundle.id)}
                onToggleSelected={() => toggleSelected(bundle.id)}
              />
            ))}
          </div>
        )}
      </Card>

      <CreateBundleModal
        open={createBundleOpen}
        onClose={() => setCreateBundleOpen(false)}
      />
      <CreateRoleModal
        open={createRoleOpen}
        onClose={() => setCreateRoleOpen(false)}
      />
      {pickerBundle && (
        <PickerHost bundle={pickerBundle} onClose={() => setPickerBundle(null)} />
      )}
    </div>
  );
}

interface BundleRowCardProps {
  bundle: BundleRow;
  expanded: boolean;
  onToggle: () => void;
  onOpenPicker: () => void;
  bulkEdit: boolean;
  selected: boolean;
  onToggleSelected: () => void;
}

function BundleRowCard({
  bundle,
  expanded,
  onToggle,
  onOpenPicker,
  bulkEdit,
  selected,
  onToggleSelected,
}: BundleRowCardProps) {
  // Roles are fetched eagerly on expand so the role chips render immediately;
  // impact is deferred behind its own accordion so we never trigger the
  // user-scan unless the operator explicitly asks for it.
  const rolesQuery = useBundleRoles(expanded ? bundle.id : null);
  const roles = rolesQuery.data ?? [];
  const distinctProjects = Array.from(new Set(roles.map((r) => r.zitadel_project_id)));

  return (
    <div
      className={`rounded-card border bg-surface-container-low transition-all ${
        expanded ? "border-primary-container/60" : "border-outline-variant hover:border-primary-container/40"
      }`}
    >
      <div className="flex items-center gap-3 px-4 py-3">
        {bulkEdit && (
          <input
            type="checkbox"
            aria-label={`Select bundle ${bundle.name}`}
            checked={selected}
            onChange={onToggleSelected}
            className="h-4 w-4 shrink-0"
          />
        )}
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={expanded}
          className="flex flex-1 items-center justify-between gap-3 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container rounded-card"
        >
          <div>
            <h3 className="font-semibold text-on-surface">{bundle.name}</h3>
            <p className="mt-0.5 text-xs text-on-surface-variant">
              {bundle.description || "No description provided."}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {bundle.is_welcome && (
              <Badge variant="secondary" className="bg-tertiary-container text-on-tertiary-container">
                Welcome
              </Badge>
            )}
            <Badge variant="outline" className="text-[10px] capitalize">
              {bundle.confirmation_mode ?? "auto"}
            </Badge>
            {bundle.created_at ? (
              <Badge variant="outline">
                {new Date(bundle.created_at).toLocaleDateString()}
              </Badge>
            ) : null}
            <span aria-hidden="true" className="text-on-surface-variant">
              {expanded ? "▾" : "▸"}
            </span>
          </div>
        </button>
      </div>

      {expanded && (
        <div className="px-4 pb-4 pt-2 border-t border-outline-variant bg-surface-container/40 animate-fade-in-up space-y-4">
          {distinctProjects.length > 0 && (
            <div>
              <Eyebrow>Affected projects ({distinctProjects.length})</Eyebrow>
              <div className="mt-2 flex flex-wrap gap-2">
                {distinctProjects.map((projectId) => (
                  <Badge key={projectId} variant="secondary" title={projectId}>
                    <ProjectName id={projectId} />
                  </Badge>
                ))}
              </div>
            </div>
          )}

          <div className="flex items-center justify-between gap-3 flex-wrap">
            <Eyebrow>Contained roles</Eyebrow>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={(event) => {
                event.stopPropagation();
                onOpenPicker();
              }}
            >
              Manage roles
            </Button>
          </div>
          <div className="-mt-2 flex flex-wrap gap-2">
            {roles.length === 0 && rolesQuery.isLoading ? (
              <p className="text-xs text-on-surface-variant italic">Loading roles…</p>
            ) : roles.length === 0 ? (
              <p className="text-xs text-on-surface-variant italic">No roles in this bundle yet.</p>
            ) : (
              roles.map((role) => (
                <Badge
                  key={`${role.zitadel_project_id}-${role.zitadel_role_key}`}
                  variant="outline"
                  className="border-primary-container/40 bg-primary-container/10 text-primary-container"
                  title={`${role.zitadel_project_id}:${role.zitadel_role_key}`}
                >
                  <ProjectName id={role.zitadel_project_id} /> ·{" "}
                  <RoleName
                    projectId={role.zitadel_project_id}
                    roleKey={role.zitadel_role_key}
                  />
                </Badge>
              ))
            )}
          </div>

          <div className="flex items-center justify-between gap-3 flex-wrap pt-3 border-t border-outline-variant">
            <div>
              <Eyebrow>Welcome bundle</Eyebrow>
              <p className="mt-1 text-sm text-on-surface-variant">
                {bundle.is_welcome
                  ? "Assigned to every newly-created Zitadel user via the onboarding trigger."
                  : "Mark this bundle as the welcome bundle to assign it on user creation."}
              </p>
            </div>
            <SetWelcomeButton bundle={bundle} />
          </div>

          <BundleImpactAccordion bundleId={bundle.id} />
        </div>
      )}
    </div>
  );
}

function SetWelcomeButton({ bundle }: { bundle: BundleRow }) {
  const setWelcome = useSetWelcomeBundle();
  if (bundle.is_welcome) {
    return (
      <Button variant="secondary" size="sm" disabled aria-label="Already welcome bundle">
        Welcome bundle
      </Button>
    );
  }
  return (
    <Button
      variant="primary"
      size="sm"
      onClick={() => setWelcome.mutate(bundle.id)}
      disabled={setWelcome.isPending}
      aria-label={`Set ${bundle.name} as welcome bundle`}
    >
      {setWelcome.isPending ? "Setting…" : "Set as welcome bundle"}
    </Button>
  );
}

/**
 * Bridges the Add-roles modal to the currently-expanded bundle's role list.
 * Hosting the role-fetch in this scoped component keeps the picker mount
 * lifetime tight to the open/close cycle and avoids leaking that fetch into
 * the page-level shell.
 */
function PickerHost({ bundle, onClose }: { bundle: BundleRow; onClose: () => void }) {
  const rolesQuery = useBundleRoles(bundle.id);
  const existingRoles = rolesQuery.data ?? [];
  return (
    <AddRolesToBundlePicker
      open
      onClose={onClose}
      bundleId={bundle.id}
      bundleName={bundle.name}
      existingRoles={existingRoles}
    />
  );
}
