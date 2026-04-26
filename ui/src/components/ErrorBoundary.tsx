"use client";

import React from "react";

interface ErrorBoundaryState {
  error: Error | null;
}

/**
 * Page-scoped error boundary. Catches render errors that escape try/catch
 * (e.g., async component throws, malformed API response). Renders a small
 * recovery surface with a Retry button rather than letting the whole tree
 * crash. Mounted around each major view.
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
          className="rounded-xl border border-dashed border-red-500/40 bg-red-500/5 p-8 text-center"
        >
          <p className="text-sm font-semibold text-foreground">Something went wrong</p>
          <p className="mx-auto mt-2 max-w-md text-sm text-muted">
            The page hit an unexpected error while rendering. Try again, or refresh if it
            keeps happening.
          </p>
          <button
            onClick={this.reset}
            type="button"
            className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-white"
          >
            Try again
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
