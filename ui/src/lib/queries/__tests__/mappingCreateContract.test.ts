import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * The create client, against the contract the backend actually has.
 *
 * Creating a mapping stopped being a bare write: it cites an approval whenever
 * the role has holders, and it answers with the row PLUS how many people it
 * queued. The typed client stayed on the old contract — it sent no `plan_id`
 * and declared the response a bare `RoleMapping`.
 *
 * Nothing called it, so nothing broke. But "the rehearsal is ready for when the
 * form lands" was not true while the client could not carry a citation: a form
 * built on it would have met a deterministic refusal on any held role, and read
 * the wrong shape back on a role nobody holds.
 */

const sent: Array<{ path: string; init?: { method?: string; body?: unknown } }> = [];

vi.mock("@/lib/api-client", () => ({
  request: vi.fn(async (path: string, init?: { method?: string; body?: unknown }) => {
    sent.push({ path, init });
    return { mapping: { id: "m1" }, queued_convergences: 3 };
  }),
  ApiError: class extends Error {},
}));

const { rehearseMappingCreate } = await import("@/lib/queries/useMappings");

beforeEach(() => {
  sent.length = 0;
});

const mapping = {
  target: "truenas",
  projectId: "pLab",
  roleKey: "maker",
  field: "group",
  value: "lab_makers",
};

describe("rehearsing a mapping that does not exist yet", () => {
  it("is keyed on what would be written, not on a row id", async () => {
    await rehearseMappingCreate(mapping, false);

    expect(sent[0].path).toBe("/targets/mappings/rehearse-create");
    expect(sent[0].init?.body).toMatchObject({
      target: "truenas",
      project_id: "pLab",
      role_key: "maker",
      field: "group",
      value: "lab_makers",
    });
  });

  // The blast radius is a backend refusal, not a checkbox drawn upfront: the
  // first ask is always unacknowledged and the ceremony appears only when the
  // backend says the change is bigger than the usual one.
  it("asks unacknowledged first, and carries the acknowledgement when it is given", async () => {
    await rehearseMappingCreate(mapping, false);
    expect(sent[0].init?.body).toMatchObject({ acknowledge_scope: false });

    await rehearseMappingCreate(mapping, true);
    expect(sent[1].init?.body).toMatchObject({ acknowledge_scope: true });
  });
});
