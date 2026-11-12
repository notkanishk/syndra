import { mappingFields } from "@/lib/fields";
import { targetLabel } from "@/lib/nav";
import { TRUENAS_SCHEMA } from "./truenas-schema";

/**
 * The field hooks answering with the TrueNAS add-on's schema, for a test that
 * renders a component without the network. Used as
 * `vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock)`.
 * A test of the "not read yet" states mocks its own.
 */
const READ = { loading: false, error: null, refetch: () => {} };

/**
 * The roster's `name`, as the backend shapes it: these fixtures' operator
 * named the TrueNAS registration "TrueNAS"; any other system has no name and
 * no manifest, so it is its id title-cased.
 */
export const TRUENAS_NAME = "TrueNAS";
export const truenasName = (system: string) => (system === "truenas" ? TRUENAS_NAME : targetLabel(system));

export const fieldSchemaMock = {
  useFieldSchema: () => TRUENAS_SCHEMA,
  useFieldSchemaState: () => ({ schema: TRUENAS_SCHEMA, ...READ }),
  useMappingFields: () => mappingFields(TRUENAS_SCHEMA),
  useSchemaSource: () => Object.assign(() => TRUENAS_SCHEMA, READ, { nameOf: truenasName }),
  useSystemNames: () => truenasName,
  useSystemNamesIfRead: () => truenasName,
};

/**
 * The real field hooks, with the roster's names: for a test about something
 * other than the schema whose roster mock carries no `name`. Used as
 * `vi.mock("@/lib/queries/useFieldSchema", async (orig) => (await import("@/lib/__fixtures__/field-schema-mock")).withRosterNames(orig))`.
 */
export async function withRosterNames(importOriginal: () => Promise<unknown>) {
  return { ...(await importOriginal() as object), useSystemNames: () => truenasName, useSystemNamesIfRead: () => truenasName };
}
