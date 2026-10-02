import { SelectionBar, SelectionAction } from "ui";

const noop = () => {};

export const Tones = () => (
  <SelectionBar count={4} noun={["person", "people"]} scope="in Laser lab" onClear={noop}>
    <SelectionAction onClick={noop}>Give a role</SelectionAction>
    <SelectionAction onClick={noop}>Extend what expires</SelectionAction>
    <SelectionAction tone="danger" onClick={noop}>Revoke a role</SelectionAction>
  </SelectionBar>
);

export const Disabled = () => (
  <SelectionBar count={1} noun={["person", "people"]} onClear={noop}>
    <SelectionAction onClick={noop}>Add to a bundle</SelectionAction>
    <SelectionAction tone="danger" disabled onClick={noop}>Remove from a bundle</SelectionAction>
  </SelectionBar>
);
