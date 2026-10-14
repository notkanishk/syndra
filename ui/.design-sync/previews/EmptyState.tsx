import { Card, EmptyState } from "ui";

export const Resolved = () => (
  <Card>
    <EmptyState
      resolved
      title="Nothing here needs you"
      guidance="No open requests, and no access expiring in the next 14 days."
    />
  </Card>
);

export const Absent = () => (
  <Card>
    <EmptyState
      title="No people yet"
      guidance="People appear here after their first sign-in with a university Google account."
      action={{ label: "Invite someone", href: "/users/invite" }}
    />
  </Card>
);

export const NoFilterMatches = () => (
  <Card>
    <EmptyState
      title="No one matches these filters"
      guidance="Try removing the role filter, or search by email instead of name."
      action={{ label: "Clear filters", onClick: () => {} }}
    />
  </Card>
);
