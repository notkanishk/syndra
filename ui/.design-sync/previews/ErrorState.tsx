import { Card, CardHeader, ErrorState } from "ui";

/** Standalone: its own danger-lined surface, a retry, and "Nothing was changed." */
export const WithRetry = () => (
  <ErrorState
    title="Could not load people"
    error={new Error("Syndra's server did not answer in time.")}
    onRetry={() => {}}
  />
);

/** Inside a card it drops its own box (`bare`). */
export const BareInCard = () => (
  <Card>
    <CardHeader title="Expiring soon" />
    <ErrorState
      bare
      title="Could not load expiring access"
      error={new Error("Zitadel did not answer the lookup.")}
      onRetry={() => {}}
    />
  </Card>
);

/** An error with no message falls back to the shared sentence; no retry when there is nothing to retry. */
export const UnknownNoRetry = () => <ErrorState title="Could not load the change history" error={null} />;
