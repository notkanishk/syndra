import { describe, expect, it } from "vitest";

import { mappingFields, type EntitlementField } from "@/lib/fields";
import { fieldWords, opsFor, rowsOf } from "@/lib/role-mapping-plan";
import type { RoleMapping } from "@/lib/queries/useMappings";

// A system whose fields share no name with TrueNAS's: the plan must follow roles.
const SCHEMA: EntitlementField[] = [
  { name: "policy", role: "membership", label: "Access policy" },
  { name: "materials", role: "flag", label: "Gives materials" },
  { name: "active", role: "account" },
];
const fields = mappingFields(SCHEMA);

const row = (field: string, value: string): RoleMapping =>
  ({ id: `${field}-1`, target: "doors", project_id: "p", role_key: "lead", field, value }) as RoleMapping;

describe("a role's mapping plan, by role", () => {
  it("reads a role's rows by the schema's roles", () => {
    const rows = rowsOf([row("policy", "workshop"), row("materials", "true")], "p", "lead", fields);
    expect(rows.group?.value).toBe("workshop");
    expect(rows.home?.value).toBe("true");
    expect(rows.quota).toBeUndefined();
  });

  it("writes the membership and the flag under their own names, and never a limit the kind lacks", () => {
    const ops = opsFor({}, { group: "workshop", home: true, quota: "5" }, fields);
    expect(ops.map((o) => `${o.kind} ${o.field}`)).toEqual(["create policy", "create materials"]);
  });

  it("says a field in the add-on's label", () => {
    expect(fieldWords("policy", SCHEMA)).toBe("access policy");
    expect(fieldWords("materials", SCHEMA)).toBe("materials");
    expect(fieldWords("unknown", SCHEMA)).toBe("setting");
  });
});
