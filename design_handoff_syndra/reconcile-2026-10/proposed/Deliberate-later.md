# To deliberate later

## Carried into later boards (settled 6 Oct 2026)

- Holding one share or group: option A of explore-hold-share.dc.html is chosen. A card per connected system on the person's page, one row per share and group, Put on hold per row, whole account from the card heading; the hold in force sits on its row and the band above the roles goes. Backend owed: an operator read of what a person's roles reach on a system (GET /users/{id}/targets), with the roles that reach each share and group.
- Connected systems index: a read-only column showing each system's bound Zitadel project, linking to the setter section on that system's page. It is never set from the index.
- System page: each connected system is bound to one Zitadel project, created by the owner. Draw the binding on the system's page (which project, how to bind or change it) when that board is built.

## Holding a role or a bundle (raised 6 Oct 2026)

Today a hold only blocks what a role reaches on a connected system: the whole account there, one share, or one group. The person keeps every role in Zitadel. There is no way to pause a direct role, a bundle or an automatic role. In Zitadel a role is either given or removed.

Questions to settle before anything is drawn:
- Is pausing a role in Zitadel wanted at all, or is Remove / Revoke plus giving it again enough?
- If wanted, is it a hold (Syndra removes the grant in Zitadel and restores it on lift) or a new Zitadel-side state?
- A bundle hold: does it pause every role the bundle gives, including roles the person also holds another way?
- An automatic rule would give the role straight back, so a held automatic role needs the rule to respect the hold.
- Needs backend work in every case. Holds due, the person's page and the member's own page would all need to show it.
