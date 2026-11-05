# TechnicalDetail (proposed, A-20, 5 Nov 2026)

The one fold for the machine's words: an id, a payload, a command, an expected-and-found pair. The copy guide §8 allows these "under a Technical detail disclosure for the person who runs the server"; this is that disclosure, built once. Drawn inline on `a20-system-in-words` §3.

## What it is
A row-shaped fold: a 28px round `FoldChevron`, the words "Technical detail" (14, 600, ink), and a faint 12.5 suffix "for whoever runs the Syndra server". Closed by default. Opened by `Unfold`, it holds one block: a `TokenBlock` for JSON, a `CommandBlock` for commands, or a `CopyableValue` list for ids. Mono 12.5 on `--surface-2` with a `--line` hairline and `--radius-inner`. The block wraps (`white-space: pre-wrap`) rather than scrolling sideways; a `CopyableValue` keeps its S18 copy.

## Where it appears
- Incoming events: under an opened row's facts, holding the raw event (`app/operations/page.tsx:434-438` today).
- A system's page: the Capabilities card, holding the operation ids (`TargetOverview.tsx:918-923`); the Technical detail that already exists for log hashes (`:1000-1005`) becomes this part.
- Mappings: under a row, holding the field and value (`MappingManagement.tsx:290-305`).
- Zitadel: under the key's row, holding the rotation command and the setting's name (`app/zitadel/page.tsx:200-228`); under Claims delivery's checks, holding expected and found (`ClaimsDelivery.tsx:480`).
- `ErrorState`, when a failure carries a body worth keeping (a status, a path), under the reference line.

## Copy
The title is always "Technical detail". The suffix is always "for whoever runs the Syndra server". Nothing inside it is rewritten: it is the machine's words, kept exact. Everything outside it is a sentence (G10).

## Motion
Opens by Unfold (settle, contents tint in 120ms behind); closes fade first. Reduced motion: opens and closes at once. Nothing inside it moves; a `CopyableValue` keeps its own crossfade and bloom.

## Phone
The same row. The block wraps at the card's width; a long id breaks anywhere (`overflow-wrap: anywhere`). The chevron's seat is 44px.

## Not for
- Explaining a state to an operator: that is the sentence above it, or a Notice.
- A reference on an error: the reference stays on the Notice's own line, with "Quote this if you ask for help."; Technical detail may sit under it holding more.
- The claims preview's token: `TokenBlock` on the claims page is the content, not a detail.
- Anything a member reads.

## Props
```ts
type TechnicalDetailProps = {
  /** Default false. Back keeps it (usePlace) when a page keys it. */
  open?: boolean;
  onToggle?: (open: boolean) => void;
  /** One block: TokenBlock, CommandBlock, or a list of CopyableValue. */
  children: React.ReactNode;
  /** Names the fold in the place store when the page has more than one. */
  placeKey?: string;
};
```
