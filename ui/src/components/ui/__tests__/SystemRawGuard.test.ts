import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * A-20 P11 guard: the System group's pages say words. Raw JSON, ids, role keys
 * and field values live inside TechnicalDetail or nowhere. A System file that
 * dumps JSON, or prints an id as a bare mono value, fails here.
 */
const root = resolve(__dirname, "../../..");
const SYSTEM = [
  "app/zitadel/page.tsx",
  "app/zitadel/grants/page.tsx",
  "app/zitadel/projects/page.tsx",
  "app/zitadel/users/page.tsx",
  "app/operations/page.tsx",
  "app/operations/cascades/page.tsx",
  "app/system/targets/page.tsx",
  "app/system/targets/[target]/page.tsx",
  "app/system/targets/[target]/mappings/page.tsx",
  "app/system/service-accounts/page.tsx",
  "app/system/service-accounts/[id]/page.tsx",
  "app/audit/page.tsx",
  "components/audit/EventLine.tsx",
  "components/system/ClaimsDelivery.tsx",
  "components/system/ServiceAccountPage.tsx",
  "components/targets/TargetOverview.tsx",
  "components/targets/MappingManagement.tsx",
  "components/targets/MergeFindings.tsx",
  "components/targets/UnconfirmedChanges.tsx",
  "components/targets/VersionBand.tsx",
];
const BARE = /(<Mono[^>]*>|type-mono[^>]*>)\s*\{[\w.?]*(_id|\.id|role_key|\.field|operation_id)\}/;
const LABEL_IS_ID = /fallback=\{[\w.]*(_id|\.id)\}/;
const read = (f: string) => readFileSync(resolve(root, f), "utf8");

describe("the System group renders no raw JSON or ids outside TechnicalDetail", () => {
  it.each(SYSTEM)("%s", (file) => {
    const src = read(file);
    const folded = src.includes("TechnicalDetail");
    expect(/JSON\.stringify\(/.test(src) && !folded, `${file} dumps JSON outside TechnicalDetail`).toBe(false);
    expect(/<pre[\s>]/.test(src) && !folded, `${file} renders a <pre> outside TechnicalDetail`).toBe(false);
    expect(BARE.test(src), `${file} prints an id, role key or field as a bare mono value`).toBe(false);
    expect(LABEL_IS_ID.test(src), `${file} hands an id to a name as its label`).toBe(false);
  });

  it("the guard itself catches a dump and a bare id", () => {
    expect(BARE.test("<Mono>{grant.id}</Mono>")).toBe(true);
    expect(BARE.test('<span className="type-mono">{mapping.role_key}</span>')).toBe(true);
    expect(LABEL_IS_ID.test("<UserName id={x} fallback={conflict.bound_subject_id} />")).toBe(true);
  });
});
