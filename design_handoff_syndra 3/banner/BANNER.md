# Syndra — repo images

Two PNGs, exported at 2× from `Syndra Banner.dc.html`.

| File | Size | Where it goes |
| --- | --- | --- |
| `syndra-banner-1280x400.png` | 2560 × 800 (2×) | Top of `README.md` |
| `syndra-social-1280x640.png` | 2560 × 1280 (2×) | Settings → Social preview |

## README markup

```markdown
<p align="center">
  <img src="banner/syndra-banner-1280x400.png" alt="Syndra — Syn keeps the door. Syndra keeps the list." width="100%">
</p>
```

GitHub serves the 2× file scaled down, so it stays sharp on retina displays.

## If you re-export

Open `Syndra Banner.dc.html` and capture `[data-banner-wide]` and
`[data-banner-social]` at 2×.

Two things to preserve, both learned the hard way:

1. **The arch and the orb ring are filled SVG bands, not strokes.** Gradient
   *strokes* do not survive rasterisation — they flatten to the first stop's
   colour and the fade disappears. Gradient *fills* rasterise correctly, so each
   ring is drawn as a closed 1.5px band (outer outline plus reversed inner
   outline).
2. **The light inside the arch is an unclipped CSS radial gradient**, not a
   filled path. A gradient clipped to the arch path has a hard edge at the path
   boundary, which redraws the arch silhouette exactly where the stroke is
   supposed to have dissolved.

CSS `mask-image` does not rasterise at all — that is why nothing here uses it,
even though the live login screen does.
