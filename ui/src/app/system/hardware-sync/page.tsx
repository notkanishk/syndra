"use client";

import { Badge, Mono } from "@/components/ui/Badge";
import { ButtonLink } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { CommandBlock } from "@/components/ui/CommandBlock";
import { PageHeader } from "@/components/ui/PageHeader";
import { UserName } from "@/components/names";
import { useIntents } from "@/lib/queries/useIntents";
import { Relative } from "@/components/ui/Time";

/**
 * S10 · System › Hardware sync — parked.
 *
 * The whole panel carries a dashed border and says "not connected yet" out
 * loud, because the alternatives all lie: a spinner implies it is coming, an
 * empty table implies it works and is idle, and "0 intents" implies a queue
 * that is being drained. None of that is true — the LLDAP integration is not
 * built, and the honest state is unbuilt rather than empty.
 *
 * The intent ledger is only rendered when it actually holds something. Real
 * rows are evidence worth showing; zero rows are exactly the "0 intents" the
 * design forbids.
 */
export default function HardwareSyncPage() {
  const intents = useIntents();
  const rows = intents.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Hardware sync"
        meta="For the door controllers and shop machines that can't speak OIDC."
      />

      <div className="rounded-card border border-dashed border-line-strong px-[30px] py-12">
        <span
          aria-hidden
          className="mb-5 flex h-11 w-11 items-center justify-center rounded-pill border border-dashed border-line-strong"
        />
        <h2 className="font-display text-[26px] font-semibold">Not connected yet.</h2>

        <p className="mt-3 max-w-[68ch] text-[15px] leading-[1.6] text-muted">
          This is where provisioning intents for the sync service and per-user shadow credentials
          will live — the path for door controllers and machine interlocks that can&rsquo;t speak
          OIDC.
        </p>
        <p className="mt-3 max-w-[68ch] text-[15px] leading-[1.6] text-muted">
          The LLDAP integration isn&rsquo;t built. Nothing is queued, nothing is pending, and no
          hardware is currently reading from MkAuth. When it lands, this page gains an intents
          queue and a per-person credential panel on the person detail.
        </p>

        <ButtonLink
          href="https://github.com/lldap/lldap"
          target="_blank"
          rel="noreferrer"
          className="mt-6"
        >
          Read the integration plan
        </ButtonLink>
      </div>

      {/*
        The sync container ships in docker-compose and refuses to start without
        LLDAP credentials, so a deployment with no directory server has one
        service restarting every sixty seconds forever. That looks like a fault
        and is in fact the configured behaviour — worth saying out loud here,
        because this is the only page in the product that has anything to do
        with it.
      */}
      <Card>
        <CardHeader
          title="If the sync container is restarting"
          note="Expected on any deployment without an LLDAP server. It is not a fault, and nothing else is affected."
        />
        <div className="flex flex-col gap-4 px-5 py-4">
          <p className="max-w-[84ch] text-[14.5px] leading-[1.6]">
            <Mono>mkauth_sync</Mono> exits at startup unless <Mono>LLDAP_BIND_DN</Mono> and{" "}
            <Mono>LLDAP_BIND_PASSWORD</Mono> are both set, and Compose restarts it on a loop.
            Nothing queues up and nothing is lost while it is down — the intents ledger is written
            by the backend, and the sync service only reads from it.
          </p>

          <CommandBlock
            command="docker compose logs --tail 20 sync"
            caption="Confirm that is what you're looking at — the last line names the missing variable."
          />

          <CommandBlock
            command="docker compose stop sync"
            caption="If you have no LLDAP server, stop it. Leaving it looping buries real failures in restart noise."
            steps={[
              <>
                When you do connect one, set <Mono>LLDAP_URL</Mono>, <Mono>LLDAP_BIND_DN</Mono>,{" "}
                <Mono>LLDAP_BIND_PASSWORD</Mono> and <Mono>LLDAP_BASE_DN</Mono> in{" "}
                <Mono>.env</Mono>.
              </>,
              <>
                Bring it back with <Mono>docker compose up -d sync</Mono>. Any intents recorded in
                the meantime are picked up on its first poll.
              </>,
            ]}
          />
        </div>
      </Card>

      {/*
        Only rendered when the ledger genuinely holds rows. An empty table here
        would say the feature works and simply has nothing to do.
      */}
      {rows.length > 0 && (
        <Card>
          <CardHeader
            title="Intents already recorded"
            count={rows.length}
            note="Written by MkAuth, consumed by nothing yet."
          />
          {rows.map((intent) => (
            <div key={intent.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3">
              <Badge tone={intent.action === "remove" ? "danger" : "accent"}>{intent.action}</Badge>
              <span className="min-w-[180px] flex-1 truncate text-[14.5px] font-semibold">
                <UserName id={intent.target_uid} />
              </span>
              <Mono className="w-[200px] shrink-0 truncate text-muted">{intent.lldap_group}</Mono>
              <Badge tone={intent.status === "failed" ? "danger" : "neutral"}>
                {intent.status}
              </Badge>
              <div className="w-[110px] shrink-0 text-right text-[13px] text-faint">
                <Relative iso={intent.created_at} />
              </div>
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
