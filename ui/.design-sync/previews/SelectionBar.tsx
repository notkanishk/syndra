import { SelectionBar, SelectionAction } from "ui";

const noop = () => {};
const Verbs = () => (
  <>
    <SelectionAction onClick={noop}>Give a role</SelectionAction>
    <SelectionAction onClick={noop}>Add to a bundle</SelectionAction>
    <SelectionAction tone="danger" onClick={noop}>Revoke a role</SelectionAction>
  </>
);

export const Picked = () => (
  <SelectionBar count={9} noun={["person", "people"]} scope="in Laser lab" composition="3 supervisors · 6 members" onClear={noop}>
    <Verbs />
  </SelectionBar>
);

/** Select-all over a filter offers the escape hatch back to what is on screen. */
export const WholeScope = () => (
  <SelectionBar
    count={140}
    noun={["person", "people"]}
    scope="matching 'wood shop'"
    wholeScope
    visibleCount={50}
    onSelectVisibleOnly={noop}
    onClear={noop}
  >
    <Verbs />
  </SelectionBar>
);

/** Over the bulk limit the verbs give way to the one move that gets under it. */
export const OverCeiling = () => (
  <SelectionBar count={612} noun={["person", "people"]} wholeScope ceiling={500} onTakeCeiling={noop} onClear={noop}>
    <Verbs />
  </SelectionBar>
);

/** Grants selected, people capped: the bar says why a number is refused. */
export const GrantsCoverPeople = () => (
  <SelectionBar
    count={601}
    noun={["grant", "grants"]}
    ceiling={500}
    ceilingCount={520}
    ceilingNoun={["person", "people"]}
    onTakeCeiling={noop}
    onClear={noop}
  >
    <SelectionAction onClick={noop}>Extend by 90 days</SelectionAction>
  </SelectionBar>
);
