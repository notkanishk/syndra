import { SelectAllRow, Card } from "ui";

const noop = () => {};

/** Filtered list: the label says which "all" it means, and the wider total sits under it. */
export const Filtered = () => (
  <Card>
    <SelectAllRow inScope={12} total={340} noun={["person", "people"]} allSelected={false} checked={false} ref={noop} onChange={noop} />
  </Card>
);

export const Unfiltered = () => (
  <Card>
    <SelectAllRow inScope={38} noun={["grant", "grants"]} allSelected={false} checked={false} ref={noop} onChange={noop} />
  </Card>
);

export const AllSelected = () => (
  <Card>
    <SelectAllRow inScope={12} total={340} noun={["person", "people"]} allSelected checked ref={noop} onChange={noop} />
  </Card>
);
