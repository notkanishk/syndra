"use client";

import { QueryClientContext } from "@tanstack/react-query";
import { useContext, useSyncExternalStore } from "react";

import { STORAGE_SCHEMA, mappingFields, type EntitlementField, type MappingFields } from "@/lib/fields";
import type { TargetSummary } from "./useTargets";

type Row = Pick<TargetSummary, "target" | "entitlement_schema" | "kind">;

/**
 * A system's field schema, from the roster the pages already read (read from
 * the cache, never fetched here: see `useCachedRoster`). A row that carries
 * none (a backend that does not send it yet) gets the storage kind's, the one
 * kind that exists today; see `STORAGE_SCHEMA`.
 */
export function schemaOf(rows: Row[] | undefined, system: string | undefined): EntitlementField[] {
  return rows?.find((t) => t.target === system)?.entitlement_schema ?? STORAGE_SCHEMA;
}

export function useFieldSchema(system: string | undefined): EntitlementField[] {
  return schemaOf(useCachedRoster(), system);
}

export function useMappingFields(system: string | undefined): MappingFields {
  return mappingFields(useFieldSchema(system));
}

/**
 * The roster's rows as the cache holds them: never fetched here (a member's
 * proxy refuses it, and these rows show for members too) and safe with no
 * query client at all, where it answers with nothing and the storage kind's
 * schema applies.
 */
export function useCachedRoster(): Row[] | undefined {
  const client = useContext(QueryClientContext);
  const data = useSyncExternalStore(
    (notify) => (client ? client.getQueryCache().subscribe(notify) : () => {}),
    () => client?.getQueryData<{ targets?: Row[] }>(["targets"]),
    () => undefined,
  );
  return data?.targets;
}

/** One field of one system, by name; for a row that holds a field name and a system. */
export function useFieldLookup(): (system: string | undefined, name: string) => EntitlementField | undefined {
  const rows = useCachedRoster();
  return (system, name) => schemaOf(rows, system).find((f) => f.name === name);
}
