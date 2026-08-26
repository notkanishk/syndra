# The add-on screens against the board

`design_handoff_syndra/design/Syndra IA.dc.html` §19–§31 is the add-on
platform's design. This is what shipped against it, read section by section on
2026-08-26.

## Faithful

**§19 · the nav delta.** Target rows are built from `GET /api/v1/targets` and sit
between Identity provider and Event activity, exactly as drawn. Neither carries
a badge — the board's reason (a count on those rows would be data driving
structure) is the comment in `lib/nav.ts` that keeps it that way. Hardware sync
is gone, and there is a test that fails if a row or route for it returns.

**§20 · the member's three states.** All three render as three, including the
middle one the board exists to argue for — "recorded, not created yet" with no
credential form. The scope sentence under the password field is present as
written.

**§21 · Health.** All five readings, each naming the machine to look at, and
draining takes the accent rather than a warning tone for the reason the board
gives. It has since grown three readings the board does not have — an unreadable
transport secret, an edited change record, and two of Syndra's own records
disagreeing about who owns an account — each above reachability because each
*explains* a target that will not answer.

**§21 · the unmanaged inventory.** Listed, never rendered as drift; adoption
blocked while the read is stale, with the reason as text rather than a tooltip;
Syndra's own account named as not adoptable rather than silently missing.

**§30 · the last two feet.** Rendered only once an account exists, host from the
add-on's registration rather than a template.

**§31 · the three shared answers.** `ReadFreshness` is one component used by
every surface that reads from a target, and the freshness/consequence split the
board insists on — adoption blocked at eleven minutes, a fourteen-minute-old
plan still appliable — is honoured and commented in both places.

## Where it diverged, and why

**A row the board never drew: Connected systems.** §19 assumes two registered
add-ons and draws their rows. It has nothing to say about a deployment that has
registered none, and until 2026-08-26 such a deployment showed nothing about
add-ons anywhere — which reads as the platform not having shipped. The index row
is static structure, so it holds the board's own rule (the rail does not move in
response to data) more strictly than a derived row does. Deliberate addition.

**The page is much longer than §21's four questions.** Roles reaching the target,
published versions, the managed half of the roster, dormant accounts, merge
findings and the reconciliation control all sit on it now. §21 drew health,
unmanaged inventory, capabilities and maintenance. Everything added has its own
board section (§24, §29) or postdates the board entirely
(`reconciliation-as-merge`). Not drift, but §21 is no longer a description of
this page and should not be read as one.

**Maintenance buttons are states, not verbs.** The board labels them
`Resume | Drain | Read-only`; the page says `Active | Draining | Read-only` so
the buttons and the definition list above them agree. A button labelled with a
state is weaker — "Drain" is unambiguously an action and "Draining" is not —
and this is worth revisiting. Left as built.

**Capability names differ from the board's.** `account.provision` /
`account.converge` / `account.release` / `credential.set` against the board's
`account.create` / `account.adopt` / `account.set_credential`. The list is
rendered from the manifest, so these are the add-on's names and not a UI choice.

## What was actually wrong

**`confirm` reached no pixel.** §21 draws *confirmation required* beside
`account.adopt` and `account.purge`. Every operation in the manifest carries
`confirm`, the type declares it, and the capability list never rendered it — so
the one place an operator can learn which operations stop and ask said nothing.
The section's own argument for showing an unavailable operation rather than
omitting it applies unchanged: what is missing from a list is read as not
existing. Fixed, with a test, mutation-checked.
