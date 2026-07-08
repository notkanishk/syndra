"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { request } from "@/lib/api-client";
import type { RotationStatus } from "@/components/zitadel/types";

// --- Section: Actions v2 signing-key rotation status ---
//
// Read-only by design. We deliberately do NOT render a "Rotate now" button:
// rotation is a cryptographic mutation whose failure modes (Zitadel accepts
// the new key but the backend is still serving the old one) are easier to
// reason about when the operator runs the command themselves with full
// terminal context. The panel exists for observability — age, threshold,
// and a copyable snippet — not as a click-to-rotate trigger.
export default function Rotation() {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<RotationStatus | null>(null);
  const [err, setErr] = useState<string>("");
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setErr("");
    try {
      setResult(
        await request<RotationStatus>("zitadel/action-rotation-status", { cache: "no-store" }),
      );
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setResult(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const badge = useMemo(() => {
    if (!result) return null;
    switch (result.status) {
      case "ok":
        return <Badge>ok</Badge>;
      case "warn":
        return <Badge variant="secondary">warn</Badge>;
      case "stale":
        return <Badge variant="destructive">stale</Badge>;
      case "disabled":
        // disabled is strictly worse than unknown: verification is actively
        // off. Use the destructive styling so the operator sees it as a
        // production problem, not a missing-config nit.
        return <Badge variant="destructive">disabled</Badge>;
      default:
        return <Badge variant="secondary">unknown</Badge>;
    }
  }, [result]);

  const subtext = useMemo(() => {
    if (!result) return null;
    if (result.status === "disabled") {
      return "ZITADEL_ACTION_SIGNING_KEY is unset on the backend — signature verification is passing every Action request through unchecked. Set the env var to the value from zitadel/actions/.action-signing-key and restart the backend before trusting the rotation age.";
    }
    if (result.status === "unknown") {
      return result.key_installed
        ? "ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT is unset, malformed, or in the future — run rotate.sh once or set the env var manually."
        : "Rotation timestamp could not be read.";
    }
    const age = result.age_days ?? 0;
    const threshold = result.threshold_days;
    if (result.status === "stale") {
      return `Key is ${age}d old — past 2× the ${threshold}d threshold. Rotate now.`;
    }
    if (result.status === "warn") {
      return `Key is ${age}d old, above the ${threshold}d threshold. Schedule a rotation.`;
    }
    return `Key is ${age}d old, within the ${threshold}d threshold.`;
  }, [result]);

  const onCopy = useCallback(async () => {
    if (!result) return;
    try {
      await navigator.clipboard.writeText(result.rotate_command);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access may be denied (e.g. non-HTTPS dev origin); fall back
      // to letting the user select the <code> text themselves.
    }
  }, [result]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Rotation Status (Actions v2 signing key)</CardTitle>
      </CardHeader>
      <div className="flex items-center gap-3 flex-wrap">
        <button
          onClick={load}
          disabled={loading}
          className="rounded-lg bg-surface-container-high px-3 py-1.5 text-xs font-medium text-on-surface disabled:opacity-50"
        >
          {loading ? "Checking..." : "Refresh"}
        </button>
        {badge}
        {result?.last_rotated_at && (
          <span className="text-xs text-on-surface-variant">
            last rotated: {new Date(result.last_rotated_at).toISOString().replace("T", " ").slice(0, 19)} UTC
          </span>
        )}
        {result?.age_days !== undefined && (
          <span className="text-xs text-on-surface-variant">· age {result.age_days}d</span>
        )}
        {result && <span className="text-xs text-on-surface-variant">· threshold {result.threshold_days}d</span>}
      </div>
      {subtext && <p className="mt-3 text-sm text-on-surface-variant max-w-3xl">{subtext}</p>}
      {err && <p className="mt-3 text-sm text-error">{err}</p>}
      {result && (
        <div className="mt-4 flex items-center gap-2 flex-wrap">
          <code className="rounded-lg bg-surface-container-high px-3 py-2 text-xs text-on-surface">
            {result.rotate_command}
          </code>
          <button
            onClick={onCopy}
            className="rounded-lg bg-surface-container-high px-3 py-2 text-xs font-medium text-on-surface"
            aria-label="Copy rotate command"
          >
            {copied ? "Copied" : "Copy"}
          </button>
          <span className="text-xs text-on-surface-variant">
            Paste into your terminal — this panel intentionally does not trigger rotation.
          </span>
        </div>
      )}
    </Card>
  );
}
