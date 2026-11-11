"use client";

import type { EntitlementField, MappingFields } from "@/lib/fields";
import { mappingFields } from "@/lib/fields";
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

/** A lookup that also says how the read it came from went. */
export type SchemaSource = SchemaLookup & SchemaRead;

type Row = { target: string; entitlement_schema?: EntitlementField[] };

/** A schema with entries, or undefined: an empty one says nothing was read. */
const read = (rows: Row[] | undefined): SchemaLookup => (system) => {
  const schema = rows?.find((t) => t.target === system)?.entitlement_schema;
  return schema && schema.length > 0 ? schema : undefined;
};

type Query = { data: Row[] | undefined; isLoading: boolean; error: unknown; refetch: () => unknown };

const sourceOf = (q: Query): SchemaSource =>
  Object.assign(read(q.data), { loading: q.isLoading, error: q.error, refetch: () => void q.refetch() });

/** The operator's: from the roster the page loads, for one system. */
export function useFieldSchemaState(system: string | undefined): { schema: EntitlementField[] | undefined } & SchemaRead {
  const source = sourceOf(useTargets());
  return { schema: source(system), loading: source.loading, error: source.error, refetch: source.refetch };
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
export function useSchemaSource(isOperator: boolean): SchemaSource {
  const roster = useTargets(isOperator);
  const mine = useMyStorage(!isOperator);
  return sourceOf(isOperator ? roster : mine);
}
