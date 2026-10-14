import { Card, CardHeader, RowSkeleton } from "ui";

/** People lists: avatar circle + text bars, fading as the list runs past the fold. */
export const WithAvatars = () => (
  <Card>
    <CardHeader title="Role members" />
    <RowSkeleton rows={5} label="Loading role members" />
  </Card>
);

/** Projects and roles have no avatar column. */
export const TextOnly = () => (
  <Card>
    <CardHeader title="Projects" />
    <RowSkeleton rows={4} avatar={false} label="Loading projects" />
  </Card>
);
