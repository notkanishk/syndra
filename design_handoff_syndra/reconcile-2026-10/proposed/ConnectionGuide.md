# ConnectionGuide (proposed)

Drawn on route-storage. Option 4b was picked on 9 Oct: a dialog opened from "First time? How to connect". The DS has no numbered-steps or key-cap component. This guide is built from Segmented, Select, CopyableValue and FieldLabel, plus the two pieces below.

## Purpose
Step-by-step instructions to reach a share from a member's own device, for someone who has never connected to network storage. The guide is hidden until it is asked for.

## Props
- `host: string`. From the add-on registration (`view.connection.host`).
- `shares: string[]`. Only the shares the member's access actually reaches.
- `username: string`
- `defaultDevice?: "mac" | "windows" | "linux" | "ios" | "android"`. Detected from the user agent; Mac is the fallback. Labels are device names, which members know before OS names.

## Parts
- **Device switch.** A Segmented control: Mac · Windows · Linux · iPhone/iPad · Android. Below tablet width it becomes a Select with the same five options, because five pills do not fit at 390px.
- **Share.** Select, shown only when there are two or more shares.
- **Steps.** Exactly four, numbered in a 26px outlined circle:
  - open the place
  - paste the address (a CopyableValue built from the protocol for that device)
  - sign in (a name/password panel: the account name, then "Your storage password, not your Syndra or Google one")
  - find it again
- **Kbd** (proposed sub-part). An inline key cap: mono 12px, 22px tall, a 1px `--line-strong` border with a 2px bottom border, on `--surface-1`.
- **If it doesn't connect.** Three plain lines: check the status beside the system, check it is the storage password, and ask staff, quoting your account name.

## States
- **Closed.** Only the entry point shows: "First time? How to connect", a ghost sm button at the right of "Your shares".
- **Open.** A Modal (size lg); a full-height sheet below tablet width. "Done" (outline) closes it.
- **Hidden.** No entry point while the system is offline (the addresses won't answer), or when no shares resolve.

## Copy rule
Each step uses the device's own words for menus and buttons (Connect to Server, Registered User, Map network drive). Nothing says SMB to the member except the address itself.

## Android
Android has no built-in SMB client, and the guide says so. Step 1 is installing a file manager that mentions SMB or LAN. Step 2 copies the server name, because most apps ask for the server and the share separately. Syndra names no app.
