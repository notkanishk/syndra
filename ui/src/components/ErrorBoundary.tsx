"use client";

import React from "react";

interface ErrorBoundaryState {
  error: Error | null;
}

/**
 * Page-scoped error boundary. Catches render errors that escape try/catch and
 * renders a recovery surface rather than letting the tree crash and take the
 * shell with it.
 *
 * It says nothing was changed for the same reason the list-level error state
 * does: after a failure on a screen about who can operate a laser cutter, the
 * first question is whether the failure did something.
 */
export class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    if (typeof console !== "undefined") {
      console.error("[ErrorBoundary]", error, info);
    }
  }

  reset = () => this.setState({ error: null });

  render() {
    if (this.state.error) {
      return (
        <div
          role="alert"
          className="flex flex-col gap-2.5 rounded-card border border-danger-line bg-surface-1 px-6 py-7"
        >
          <div className="type-empty-title">This page couldn&rsquo;t render.</div>
          <p className="max-w-[60ch] text-[14px] text-muted">
            Something went wrong while drawing it. Nothing was changed — try again, or reload if it
            keeps happening.
          </p>
          <button
            onClick={this.reset}
            type="button"
            className="mt-1.5 min-h-[44px] self-start rounded-pill bg-tint-3 px-4 text-[13px] font-semibold text-ink motion-tint hover:bg-tint-2 desktop:min-h-0 desktop:py-1.5"
          >
            Try again
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
