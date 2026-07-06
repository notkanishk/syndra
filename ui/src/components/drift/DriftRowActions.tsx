"use client";

import { useState } from "react";

import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { useBundleRolesByBundle, useBundles } from "@/lib/queries/useBundles";
import { useAttributeDrift, useMarkExternalDrift, useRevokeDrift } from "@/lib/queries/useDrift";
import type { DriftItem } from "@/lib/queries/useGovernance";

type AttributeSource = "external_backfill" | "bundle" | "rule";

/**
 * Per-row triage actions for a single drift item: Attribute (inline form),
 * Revoke, Mark external. Split out of DriftTriageClient because the
 * attribute form (source select + bundle role validation) pushed the row
 * past a comfortable single-component size.
 */
export function DriftRowActions({ item }: { item: DriftItem }) {
  const [mode, setMode] = useState<"idle" | "attribute" | "mark-external">("idle");
  const [source, setSource] = useState<AttributeSource>("external_backfill");
  const [sourceRef, setSourceRef] = useState("");
  const [reason, setReason] = useState("");

  const attribute = useAttributeDrift();
  const revoke = useRevokeDrift();
  const markExternal = useMarkExternalDrift();

  const bundles = useBundles();
  const bundleIds = (bundles.data ?? []).map((b) => b.id);
  const { byId: rolesByBundle } = useBundleRolesByBundle(source === "bundle" ? bundleIds : []);

  function bundleHasDriftRole(bundleId: string) {
    const roles = rolesByBundle[bundleId] ?? [];
    return item.role_keys.every((rk) =>
      roles.some((r) => r.zitadel_project_id === item.project_id && r.zitadel_role_key === rk),
    );
  }

  function submitAttribute() {
    attribute.mutate(
      { id: item.id, body: { source, source_ref: sourceRef || undefined } },
      { onSuccess: () => setMode("idle") },
    );
  }

  function submitMarkExternal() {
    markExternal.mutate(
      { id: item.id, body: { reason: reason || undefined } },
      { onSuccess: () => setMode("idle") },
    );
  }

  if (mode === "attribute") {
    return (
      <div className="mt-3 space-y-2 rounded-card border border-outline-variant bg-surface-container-low/60 p-3">
        <div className="flex flex-wrap items-center gap-2">
          <Select
            value={source}
            onChange={(e) => setSource(e.target.value as AttributeSource)}
            aria-label="Attribution source"
            className="w-auto"
          >
            <option value="external_backfill">External backfill</option>
            <option value="bundle">Bundle</option>
            <option value="rule">Rule</option>
          </Select>
          {source === "bundle" ? (
            <Select
              value={sourceRef}
              onChange={(e) => setSourceRef(e.target.value)}
              aria-label="Bundle"
              className="w-auto"
            >
              <option value="">Choose a bundle…</option>
              {(bundles.data ?? []).map((b) => (
                <option key={b.id} value={b.id} disabled={!bundleHasDriftRole(b.id)}>
                  {b.name}
                  {!bundleHasDriftRole(b.id) ? " (missing role)" : ""}
                </option>
              ))}
            </Select>
          ) : (
            <Input
              value={sourceRef}
              onChange={(e) => setSourceRef(e.target.value)}
              placeholder="source_ref (optional)"
              aria-label="Source reference"
              className="w-auto"
            />
          )}
        </div>
        {attribute.isError && (
          <p role="alert" className="text-xs text-error">
            {attribute.error instanceof Error ? attribute.error.message : "Attribution failed"}
          </p>
        )}
        <div className="flex gap-2">
          <Button
            size="sm"
            onClick={submitAttribute}
            isPending={attribute.isPending}
            disabled={source === "bundle" && !sourceRef}
          >
            Confirm attribute
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setMode("idle")}>
            Cancel
          </Button>
        </div>
      </div>
    );
  }

  if (mode === "mark-external") {
    return (
      <div className="mt-3 space-y-2 rounded-card border border-outline-variant bg-surface-container-low/60 p-3">
        <Input
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder="Reason (optional)"
          aria-label="Mark-external reason"
        />
        {markExternal.isError && (
          <p role="alert" className="text-xs text-error">
            {markExternal.error instanceof Error ? markExternal.error.message : "Mark external failed"}
          </p>
        )}
        <div className="flex gap-2">
          <Button size="sm" onClick={submitMarkExternal} isPending={markExternal.isPending}>
            Confirm mark external
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setMode("idle")}>
            Cancel
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="mt-2 flex flex-wrap gap-2">
      <Button size="sm" variant="outline" onClick={() => setMode("attribute")}>
        Attribute
      </Button>
      <Button
        size="sm"
        variant="destructive"
        onClick={() => revoke.mutate({ id: item.id })}
        isPending={revoke.isPending}
      >
        Revoke
      </Button>
      <Button size="sm" variant="ghost" onClick={() => setMode("mark-external")}>
        Mark external
      </Button>
      {revoke.isError && (
        <p role="alert" className="w-full text-xs text-error">
          {revoke.error instanceof Error ? revoke.error.message : "Revoke failed"}
        </p>
      )}
    </div>
  );
}
