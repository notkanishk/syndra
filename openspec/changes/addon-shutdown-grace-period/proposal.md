# The add-on's shutdown drain is truncated on every stop

## Why

The add-on implements a graceful shutdown deliberately. On `SIGINT`/`SIGTERM` it
sets itself to `draining`, then gives `httpSrv.Shutdown` **twenty seconds**, with
the reason written beside it:

> Draining before the deadline, so an in-flight mutation settles rather than
> being abandoned half-applied with no record of how far it got.

Docker's default stop timeout is **ten seconds**, after which the container is
`SIGKILL`ed. `docker-compose.yml` sets `stop_grace_period` on no service.

So the drain has never had the budget it was written for. Every
`docker compose down`, `up -d --force-recreate`, restart, or host reboot kills
the add-on halfway through the settle it performs precisely so a mutation is not
abandoned half-applied — which is the exact state it was built to prevent, and
the one where TrueNAS holds a change Syndra has no terminal record of.

Nothing reports it. A truncated drain looks like a normal stop: the process is
gone either way, and the mutation that was settling leaves the same silence as
one that never started.

**This is the branch's recurring defect in the deployment manifest**, which is
where `addon-platform` §32 already found four of them: the add-on's shutdown
budget and the deployment's stop timeout are two definitions of one thing, each
internally consistent, with nothing making them agree.

## What Changes

- `stop_grace_period` set on the add-on service, greater than the add-on's own
  shutdown budget so the process always reaches its own deadline first. The
  container timeout is the outer bound and must never be the binding one.
- A guard tying the two together, because a comment did not hold last time: the
  add-on's timeout is read from its source and asserted to be less than the
  Compose value. Changing either alone fails a test rather than silently
  restoring the truncation.

## Impact

- **Affected specs:** `addon-platform` (one requirement added)
- **Affected code:** `docker-compose.yml`, one guard test
- **Operator impact:** stops take longer when a mutation is genuinely in flight,
  which is the intended behaviour and the reason the drain exists.
- **Risk:** low. The change lengthens an upper bound; a stop with nothing in
  flight returns as fast as it does today, since `Shutdown` returns as soon as
  connections are idle.

## Why it is not part of `addon-transport-derived-keys`

It was found while specifying that change's rotation procedure, and it was
carried there for one draft. It does not belong there: it is a live defect on
the current branch, it is not caused by that change, and gating it behind a
proposal that may not land would leave a shipped truncation in place for the
length of that discussion. The transport change does not depend on it either —
its rotation quiesces through the runtime lifecycle operation before anything is
stopped.
