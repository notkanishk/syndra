import { CommandBlock, Mono } from "ui";

export const WithSteps = () => (
  <CommandBlock
    command="make zitadel-actions-rotate-key"
    caption="Run this in a terminal on the server Syndra runs on. This panel does not replace the key itself — you must restart afterwards."
    steps={[
      <>Put the new value in <Mono>ZITADEL_ACTION_SIGNING_KEY</Mono> in <Mono>.env</Mono>.</>,
      <>Restart the backend: <Mono>docker compose restart backend</Mono>.</>,
      <>Confirm it took: <Mono>make zitadel-actions-verify</Mono>.</>,
    ]}
  />
);

/** warn: a command that changes production state. */
export const Warn = () => (
  <CommandBlock
    tone="warn"
    command="make zitadel-actions-register"
    caption="No key is set up, so Syndra cannot tell genuine Zitadel requests from forged ones. Run this to register with Zitadel; it creates a key."
    steps={[<>Copy the printed key into <Mono>ZITADEL_ACTION_SIGNING_KEY</Mono>.</>]}
  />
);

/** onWarn: the same block sitting on an amber banner. */
export const OnWarnBanner = () => (
  <div style={{ padding: 16, background: "var(--warn)", color: "var(--warn-ink)" }}>
    <CommandBlock
      tone="onWarn"
      command="docker compose restart backend"
      caption="The backend stopped answering. Restarting it is safe; queued changes are kept."
    />
  </div>
);

export const CommandOnly = () => <CommandBlock command="docker compose logs -f --tail=200 sync" />;
