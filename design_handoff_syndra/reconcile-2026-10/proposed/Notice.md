# PROPOSED — Notice

Status: proposed 4 Oct 2026. Replaces ErrorState's two looks (bare and bordered) and gives DegradedBanner and OfflineBanner one shared anatomy. Behaviour of all three is unchanged.

## Purpose
The one way Syndra says something is wrong with what you are looking at: data is stale, a read failed, or the network is gone.

## Anatomy
Tinted box (`--radius-panel`, 1px tone line, tone soft fill, 16px 20px) · 8px dot · title in `--font-display` 19px/600 in the tone's text colour · one sentence in `--ink` 14.5px, max 76ch · optional action row: `Button variant="outline" size="sm"` and a reference.

## Props
```ts
interface NoticeProps {
  tone: "warn" | "danger" | "neutral";
  title: string;            // what happened, as a sentence
  children: ReactNode;      // one sentence: what it means; danger always ends "Nothing was changed."
  live?: boolean;           // dot breathes while the condition is still going on
  action?: { label: string; onClick: () => void }; // outline, never danger
  reference?: string;       // request id, shown as CopyableValue inline
}
```

## Tones
- **warn** — stale or paused (Zitadel down, last known data). live.
- **danger** — a read failed. Not live. Action "Try again", reference when there is one.
- **neutral** — no network. Hollow dot, `--surface-2` fill, `--line-strong` border.

## Placement
Content column, above the page header for app-wide conditions; in the place of the content it failed to load for a failed read (decided 4 Oct: 6a — one place for every notice, never a band under the top bar).

## Replaces
- `ErrorState` → `<Notice tone="danger" title={title} action={{label:"Try again", onClick:onRetry}} reference={requestId}>{describeFailure(error)} Nothing was changed.</Notice>`. ListStates keeps its API.
- `DegradedBanner` / `OfflineBanner` render a Notice; their copy and conditions stay as they are (see DegradedBanner.md for last known data).

## Motion
Arrives by Motion 6a; leaves the same way reversed.
