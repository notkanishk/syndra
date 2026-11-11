import { mappingFields } from "@/lib/fields";
import { TRUENAS_SCHEMA } from "./truenas-schema";

/**
 * The field hooks answering with the TrueNAS add-on's schema, for a test that
 * renders a component without the network. Used as
 * `vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock)`.
 * A test of the "not read yet" states mocks its own.
 */
const READ = { loading: false, error: null, refetch: () => {} };

export const fieldSchemaMock = {
  useFieldSchema: () => TRUENAS_SCHEMA,
  useFieldSchemaState: () => ({ schema: TRUENAS_SCHEMA, ...READ }),
  useMappingFields: () => mappingFields(TRUENAS_SCHEMA),
  useSchemaSource: () => Object.assign(() => TRUENAS_SCHEMA, READ),
};
