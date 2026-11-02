import { Card, PaginationRow } from "ui";

const noop = () => {};

/** With a total: the whole sentence. */
export const FirstPage = () => (
  <Card>
    <PaginationRow shown={10} total={61} onMore={noop} onAll={noop} onBack={noop} />
  </Card>
);

/** After Show more: Back to 10 joins the line. */
export const PartWay = () => (
  <Card>
    <PaginationRow shown={35} total={61} onMore={noop} onAll={noop} onBack={noop} />
  </Card>
);

/** Without a total, at the end: it says the list has ended. */
export const ServerPagedEnd = () => (
  <Card>
    <PaginationRow shown={42} hasMore={false} since="1 Aug" onMore={noop} onBack={noop} />
  </Card>
);

/** Without a total, mid-way, fetching more. */
export const ServerPagedLoading = () => (
  <Card>
    <PaginationRow shown={25} hasMore pending onMore={noop} onBack={noop} />
  </Card>
);
