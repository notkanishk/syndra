import { ArrowLink, Card, CardHeader } from "ui";

/** The one link that goes to another page: the words only, the arrow is drawn. */
export const OnItsOwn = () => (
  <div style={{ display: "grid", gap: 8, justifyItems: "start" }}>
    <ArrowLink href="/governance/pending">Pending changes</ArrowLink>
    <ArrowLink href="/users">See everyone with access</ArrowLink>
  </div>
);

/** Closing a card, where it says where the rest is. */
export const UnderACard = () => (
  <Card>
    <CardHeader title="Expiring this week" count={3} />
    <div style={{ padding: "12px 20px" }}>
      <ArrowLink href="/users?filter=expiring">All expiring access</ArrowLink>
    </div>
  </Card>
);
