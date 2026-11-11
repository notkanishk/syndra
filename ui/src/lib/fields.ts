/**
 * The UI reads a target's fields by ROLE, never by name (addon-contract §3).
 *
 * A schema entry says what a field is (`account`, `channel`, `membership`,
 * `flag`, `limit`) and how to say it (`label`, `unit`). A screen asks this file
 * for "the account field" or "the limits", and says what comes back in the
 * add-on's words. The five TrueNAS field names and `smb` are allowed in this
 * file and in `components/kinds/storage/` only; `eslint.config.mjs` enforces it.
 */

export type FieldRole = "account" | "channel" | "membership" | "flag" | "limit";

export type LimitUnit = "count" | "items" | "bytes" | "seconds" | "grams";

/** One entry of a manifest's `entitlement_schema`, as the backend carries it. */
export interface EntitlementField {
  name: string;
  role: FieldRole;
  label?: string;
  unit?: LimitUnit | string;
  combine?: string;
  unlimited?: string;
  per_person?: boolean;
}

export type FieldsByRole = Record<FieldRole, EntitlementField[]>;

/** Every role present, so a caller never checks for an absent list. */
export function fieldsByRole(schema: EntitlementField[] | undefined): FieldsByRole {
  const out: FieldsByRole = { account: [], channel: [], membership: [], flag: [], limit: [] };
  for (const f of schema ?? []) out[f.role]?.push(f);
  return out;
}

/** THE on/off of the account: exactly one per schema. */
export function accountField(schema: EntitlementField[] | undefined): EntitlementField | undefined {
  return fieldsByRole(schema).account[0];
}

export function limitFields(schema: EntitlementField[] | undefined): EntitlementField[] {
  return fieldsByRole(schema).limit;
}

/** The role a field name has on this schema; undefined for a name it does not declare. */
export function roleOf(schema: EntitlementField[] | undefined, name: string): FieldRole | undefined {
  return (schema ?? []).find((f) => f.name === name)?.role;
}

/** A field is the account itself, or one of its channels: the two the resolver derives, never mapped. */
export function isLifecycleField(schema: EntitlementField[] | undefined, name: string): boolean {
  const role = roleOf(schema, name);
  return role === "account" || role === "channel";
}

const GB = 1024 ** 3;
const MB = 1024 ** 2;
const KB = 1024;

function plain(n: number): string {
  return Number.isInteger(n) ? String(n) : n.toFixed(1);
}

/**
 * A limit's number, in its unit's own words: "20 GB", "90 minutes", "3 items".
 * `unlimited` is the field's, so the caller says "no limit" for that value
 * before it gets here; a value that is not a number is returned as it came.
 */
export function formatLimit(value: string | number, unit: string | undefined): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return String(value);
  switch (unit) {
    case "bytes":
      if (n >= GB) return `${plain(n / GB)} GB`;
      if (n >= MB) return `${plain(n / MB)} MB`;
      if (n >= KB) return `${plain(n / KB)} KB`;
      return `${n} bytes`;
    case "seconds":
      if (n >= 3600 && n % 3600 === 0) return `${n / 3600} ${n === 3600 ? "hour" : "hours"}`;
      if (n >= 60 && n % 60 === 0) return `${n / 60} ${n === 60 ? "minute" : "minutes"}`;
      return `${n} ${n === 1 ? "second" : "seconds"}`;
    case "grams":
      return n >= 1000 ? `${plain(n / 1000)} kg` : `${n} g`;
    case "items":
      return `${n} ${n === 1 ? "item" : "items"}`;
    default:
      return String(n);
  }
}

/** "no limit" when the value is the field's `unlimited`, else the number in its unit. */
export function limitWordsFor(field: EntitlementField, value: string | number | undefined | null): string {
  if (value === undefined || value === null || value === "" || (field.unlimited !== undefined && String(value) === field.unlimited)) {
    return "no limit";
  }
  return formatLimit(value, field.unit);
}

/**
 * A field named for a person, lowercase, to sit in a sentence ("its group").
 * A flag's label reads "Gives a home folder"; the noun is what is left.
 */
export function fieldNoun(field: EntitlementField | undefined): string {
  if (!field?.label) return "setting";
  const label = field.role === "flag" ? field.label.replace(/^Gives (an? )?/i, "") : field.label;
  return label.toLowerCase();
}

/** The names a role mapping works with: its membership, its flag, its limit. */
export interface MappingFields {
  membership?: string;
  flag?: string;
  limit?: string;
}

export function mappingFields(schema: EntitlementField[] | undefined): MappingFields {
  const by = fieldsByRole(schema);
  return { membership: by.membership[0]?.name, flag: by.flag[0]?.name, limit: by.limit[0]?.name };
}

/**
 * What the storage kind declares today, held here until the backend carries
 * the schema on the targets endpoint (it does not yet: see tasks 3.1). Used
 * only when a target arrives with no schema, so a deployment that has not
 * shipped it keeps working; a target that sends one is never second-guessed.
 */
export const STORAGE_SCHEMA: EntitlementField[] = [
  { name: "group", role: "membership", label: "Group" },
  { name: "home", role: "flag", label: "Gives a home folder" },
  { name: "home_quota", role: "limit", label: "Home folder limit", unit: "bytes", combine: "max", unlimited: "0", per_person: true },
  { name: "enabled", role: "account" },
  { name: "smb_enabled", role: "channel", label: "SMB" },
];
