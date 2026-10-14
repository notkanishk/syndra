# VersionBand (proposed for the DS)

Ships in the app as `components/targets/VersionBand.tsx`; not in the design system. Drawn on route-system-targets-id-mappings from Card, StatusDot and Button.

## Purpose
The fixed seat at the top of a mappings page. It says whether the live rows match the newest published version, and holds the one Publish control (#12).

## Props
- `reading`: `"matches" | "ahead" | "none"`
- `version?: number`, the newest published version
- `published?: { by: UserId; at: string; note: string }`
- `edits?: { kind: "added" | "changed" | "removed"; role: RoleRef; what: string; by: UserId; at: string; people: number }[]`
- `onPublish: () => void`. Opens the publish dialog, which requires a note.

## States
- **matches**: healthy dot, "Matches version N", who published it and their note. Publish is disabled, with "Nothing to publish" beneath it.
- **ahead**: warn dot, "Version N, plus K changes not yet published". It lists every change, including removals, which have no row anywhere else. Publish uses the accent colour and reads "Publish as version N+1…".
- **none**: neutral dot, "Nothing published yet". Publish is disabled.
- **loading / error**: not rendered. The page's skeleton or Notice takes its seat.

The band's shape never changes. Only its dot, its words and whether Publish is enabled change.
