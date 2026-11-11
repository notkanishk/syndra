"use client";

import type { EntitlementField, MappingFields } from "@/lib/fields";
import { mappingFields } from "@/lib/fields";
import { targetLabel } from "@/lib/nav";
import { useMyStorage } from "./useMyStorage";
import { useTargets } from "./useTargets";

/**
 * A system's fields come from the response the page loaded: the roster for an
 * operator (GET /targets), the member's own list for a member (/me/targets).
 * Both carry the ACCEPTED manifest's schema and nothing before one is
 * accepted, and then the answer here is `undefined`: "not read yet". There is
 * no schema of Syndra's own to fall back on, because a guess about what a
 * system's fields mean is how a hold on an account gets worded as the word
 * `true`.
 */
export type SchemaLookup = (system: string | undefined) => EntitlementField[] | undefined;

/**
 * The state of the read the schema comes from, one shape for every caller:
 * `loading` (not answered yet), `error` (the read failed) and, with neither,
 * the answer, where an absent schema means the read succeeded and no manifest
 * is accepted: "not read yet". Only that last one may be worded as such.
 */
export interface SchemaRead {
  loading: boolean;
  error: unknown;
  refetch: () => void;
}

/**
 * A lookup that also says how the read it came from went, and what each
 * system is called: the same rows carry the backend's `name` (one source:
 * the operator's name, else the manifest's product, else the id), and before
 * they arrive the id title-cased (`targetLabel`).
 */
export type SchemaSource = SchemaLookup & SchemaRead & { nameOf: (system: string) => string };

type Row = { target: string; name?: string; entitlement_schema?: EntitlementField[] };

/** A schema with entries, or undefined: an empty one says nothing was read. */
const read = (rows: Row[] | undefined): SchemaLookup => (system) => {
  const schema = rows?.find((t) => t.target === system)?.entitlement_schema;
  return schema && schema.length > 0 ? schema : undefined;
};

type Query = { data: Row[] | undefined; isLoading: boolean; error: unknown; refetch: () => unknown };

/**
 * A failed background refresh keeps the rows it had: it is "failed" only when
 * there are none. The roster refetches every 30s, and treating one failed
 * refresh as a failed read blanked whatever a page had drawn from the rows
 * (a form being filled in, with its reason) until the next one landed.
 */
const sourceOf = (q: Query): SchemaSource =>
  Object.assign(read(q.data), {
    loading: q.isLoading,
    error: q.data ? null : q.error,
    refetch: () => void q.refetch(),
    nameOf: (system: string) => q.data?.find((t) => t.target === system)?.name ?? targetLabel(system),
  });

/** The operator's: from the roster the page loads, for one system. */
export function useFieldSchemaState(system: string | undefined): { schema: EntitlementField[] | undefined } & SchemaRead {
  const source = sourceOf(useTargets());
  return { schema: source(system), loading: source.loading, error: source.error, refetch: source.refetch };
}

/**
 * What an operator's page calls each system: the roster's name for it.
 * `enabled` false asks nothing (a member's shell, whose proxy refuses it).
 */
export function useSystemNames(enabled = true): (system: string) => string {
  return sourceOf(useTargets(enabled)).nameOf;
}

export function useFieldSchema(system: string | undefined): EntitlementField[] | undefined {
  return useFieldSchemaState(system).schema;
}

export function useMappingFields(system: string | undefined): MappingFields {
  return mappingFields(useFieldSchema(system));
}

/**
 * Many systems at once, for a list that names holds on several: the roster for
 * an operator, `/me/targets` for anyone else (their proxy refuses the roster,
 * so it is not asked). Only the one that is needed is fetched.
 */
export function useSchemaSource(isOperator: boolean, enabled = true): SchemaSource {
  // `enabled` false asks nothing: a page with no hold to word does not wait on
  // a read that calls each add-on (a member's /me/targets).
  const roster = useTargets(isOperator && enabled);
  const mine = useMyStorage(!isOperator && enabled);
  return sourceOf(isOperator ? roster : mine);
}
