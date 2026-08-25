import type { MetadataRoute } from "next";

/**
 * Installable, because members are students and a URL is a thing they lose.
 *
 * Members are the largest population this serves and their whole surface is
 * four screens — their access, a request, storage, sign-in — every one of
 * which they use standing in the space rather than at a desk. An icon on a
 * home screen is the difference between "ask the steward" and "look it up".
 *
 * `display: standalone` rather than fullscreen: fullscreen takes the status
 * bar, and the status bar is where the clock and the battery are for somebody
 * deciding whether they have time to start a machine.
 *
 * The two icons are the same mark and different drawings, which is the point
 * of `maskable`. Android crops a maskable icon to whatever shape the launcher
 * uses — circle, squircle, rounded square — and anything inside the outer 20%
 * can be cut. `icon.svg` fills its box, so it is declared `any` only; the
 * maskable variant carries the same mark inset into the safe zone with the
 * ground painted behind it, because a transparent maskable icon is composited
 * onto whatever colour the launcher chooses and the arc's fade needs a known
 * ground to fade into.
 */
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Makerspace Syndra",
    short_name: "Syndra",
    description: "Access management for the makerspace",
    start_url: "/",
    display: "standalone",
    // The dark ground: this product is dark by default, and a white splash
    // flashing before a near-black app is the stutter D3 exists to avoid.
    background_color: "#080906",
    theme_color: "#080906",
    orientation: "any",
    icons: [
      {
        src: "/icon.svg",
        sizes: "any",
        type: "image/svg+xml",
        purpose: "any",
      },
      {
        src: "/icon-maskable.svg",
        sizes: "any",
        type: "image/svg+xml",
        purpose: "maskable",
      },
    ],
  };
}
