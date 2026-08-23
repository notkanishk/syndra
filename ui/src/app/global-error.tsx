"use client";

import { useEffect } from "react";

/**
 * When the root layout itself fails.
 *
 * `error.tsx` catches everything thrown inside the layout's children, which is
 * almost everything — but not the layout, not the providers it mounts, and not
 * the shell around them. A throw there unmounts the layout, so the boundary
 * that lives inside it never renders and Next falls back to its own default
 * page: unstyled, unthemed, no identifier, no way back. On a phone that is the
 * blank screen this branch set out to remove, arriving by a different door.
 *
 * Two rules follow from what has just failed, and both cost something:
 *
 *  - **No imports from the app.** Not the copy row, not a button, not a token.
 *    Whatever threw may be a provider every one of those sits inside, and a
 *    boundary that re-enters the broken tree is not a boundary. The colours are
 *    written out literally here — the one place in this product where that is
 *    correct rather than a violation — because `globals.css` is loaded by the
 *    document this component is replacing.
 *  - **No `reset()`.** Same reasoning as `error.tsx`: a render that threw on
 *    the state it was given throws again on the same state, and a button that
 *    repeats a failure while looking like a remedy is worse than no button.
 *
 * Both themes are authored, because the shell that would have chosen one is
 * the thing that is gone.
 */
export default function GlobalError({ error }: { error: Error & { digest?: string } }) {
  useEffect(() => {
    console.error("Syndra failed before the shell existed:", error);
  }, [error]);

  return (
    <html lang="en">
      <body
        style={{
          margin: 0,
          minHeight: "100dvh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          padding: "20px",
          background: "#080906",
          color: "#f3f5ef",
          fontFamily: "system-ui, -apple-system, sans-serif",
        }}
      >
        {/* A style element rather than inline props for the theme, because an
            inline style cannot hold a media query and a dark card on a light
            phone reads as a rendering fault — which is the wrong thing to be
            saying on the screen that reports one. */}
        <style>{`
          @media (prefers-color-scheme: light) {
            body { background: #f4f1fb !important; color: #1d1830 !important; }
            .syndra-global-error { background: #ffffff !important; border-color: rgba(213,56,42,.35) !important; }
            .syndra-global-error p { color: rgba(29,24,48,.6) !important; }
            .syndra-global-error a { background: #4c30c4 !important; color: #ffffff !important; }
          }
        `}</style>

        <div
          className="syndra-global-error"
          style={{
            width: "100%",
            maxWidth: "52ch",
            borderRadius: "18px",
            border: "1px solid rgba(255,92,77,.4)",
            background: "#1b1e19",
            padding: "28px 24px",
          }}
        >
          <h1 style={{ margin: 0, fontSize: "20px", lineHeight: 1.2, fontWeight: 600 }}>
            Syndra stopped before it could draw anything.
          </h1>
          <p style={{ margin: "8px 0 0", fontSize: "14.5px", lineHeight: 1.55, color: "rgba(243,245,239,.6)" }}>
            This failed while starting the application, not while writing anything. Nothing was
            changed.
          </p>

          {error.digest ? (
            <p
              style={{
                margin: "16px 0 0",
                fontSize: "13px",
                fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
                // Selectable rather than copyable: the copy row is a component
                // in the tree this boundary is not allowed to touch.
                userSelect: "all",
              }}
            >
              Error id {error.digest}
            </p>
          ) : null}

          {/* A plain anchor, for the reason `error.tsx` gives and one more: a
              soft navigation re-renders the same client tree, and here the
              tree is the thing that threw. `next/link` would also be an import
              from the app, which this file may not have. */}
          {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
          <a
            href="/"
            style={{
              display: "inline-flex",
              alignItems: "center",
              minHeight: "44px",
              marginTop: "20px",
              padding: "0 16px",
              borderRadius: "999px",
              background: "#cebcff",
              color: "#141414",
              fontSize: "13.5px",
              fontWeight: 600,
              textDecoration: "none",
            }}
          >
            Reload from the start
          </a>
        </div>
      </body>
    </html>
  );
}
